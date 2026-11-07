package catalog

import (
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"
	"github.com/blamoo/sugoi/internal/search"
)

const testKey = "{{LIB}}/collection/[Artist] Some Title.cbz"

// newTestCatalog builds a catalog over a temporary database and an in-memory
// index, with the given files.txt lines.
func newTestCatalog(t *testing.T, keys ...string) *Catalog {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{
		DatabaseDir: dir,
		CacheDir:    t.TempDir(),
		DirVars:     map[string]string{"LIB": dir + "/library"},
	}

	index, err := search.OpenMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { index.Close() })

	return New(cfg, library.New(cfg, keys), index)
}

func (c *Catalog) mustGet(t *testing.T, key string) *Thing {
	t.Helper()
	thing, err := c.Get(library.NewFile(c.cfg, key).Hash)
	if err != nil {
		t.Fatal(err)
	}
	return thing
}

func (c *Catalog) searchCount(t *testing.T, q string) uint64 {
	t.Helper()
	parsed, err := bleve.NewQueryStringQuery(q).Parse()
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Index.Search(bleve.NewSearchRequest(search.ShouldToMust(parsed)))
	if err != nil {
		t.Fatal(err)
	}
	return res.Total
}

func TestConcurrentMarksAreNotLost(t *testing.T) {
	c := newTestCatalog(t, testKey)

	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// each request loads its own copy, like the HTTP handlers
			errs <- c.mustGet(t, testKey).AddMark()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := c.mustGet(t, testKey).Marks; got != n {
		t.Fatalf("marks = %d, want %d", got, n)
	}
}

func TestRatingToggle(t *testing.T) {
	c := newTestCatalog(t, testKey)

	thing := c.mustGet(t, testKey)
	if err := thing.TrySaveRating(4, true); err != nil {
		t.Fatal(err)
	}
	if thing.Rating != 4 {
		t.Fatalf("rating = %d, want 4", thing.Rating)
	}

	// toggling the same rating clears it, based on what is on disk
	if err := c.mustGet(t, testKey).TrySaveRating(4, true); err != nil {
		t.Fatal(err)
	}
	if got := c.mustGet(t, testKey).Rating; got != 0 {
		t.Fatalf("rating = %d, want 0 after toggle", got)
	}

	// mutations update the search index
	if err := c.mustGet(t, testKey).TrySaveRating(5, false); err != nil {
		t.Fatal(err)
	}
	if n := c.searchCount(t, "rating:5"); n != 1 {
		t.Fatalf("rating:5 matched %d", n)
	}
}

func TestWritesRefusedDuringReindex(t *testing.T) {
	c := newTestCatalog(t, testKey)
	thing := c.mustGet(t, testKey)

	c.Reindex.update(func(s *ReindexStatus) { s.Running = true })
	defer c.Reindex.update(func(s *ReindexStatus) { s.Running = false })

	if err := thing.AddMark(); !errors.Is(err, ErrReindexRunning) {
		t.Errorf("AddMark: err = %v, want ErrReindexRunning", err)
	}
	if err := thing.SaveStaticFromForm(url.Values{"title": {"x"}}); !errors.Is(err, ErrReindexRunning) {
		t.Errorf("SaveStaticFromForm: err = %v, want ErrReindexRunning", err)
	}

	// nothing reached the disk
	for _, f := range []string{thing.File.DynamicMetaPath(), thing.File.StaticMetaPath()} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("%s was written: %v", f, err)
		}
	}
}

func TestSaveStaticFromFormKeepsImportedFields(t *testing.T) {
	c := newTestCatalog(t, testKey)
	thing := c.mustGet(t, testKey)

	// as written by the importer: id_source is an array
	imported := `{"id":[1970],"id_source":["/item/1970/abc"],"circle":["Circle"],"title":"Old","pages":18}`
	if err := os.MkdirAll(thing.File.MetaPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thing.File.StaticMetaPath(), []byte(imported), 0644); err != nil {
		t.Fatal(err)
	}

	if err := c.mustGet(t, testKey).SaveStaticFromForm(url.Values{"title": {"New"}}); err != nil {
		t.Fatal(err)
	}

	saved, err := metadata.ReadStatic(thing.File.StaticMetaPath())
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "New" || len(saved.Id) != 1 || len(saved.IdSource) != 1 ||
		saved.IdSource[0] != "/item/1970/abc" || len(saved.Circle) != 1 || saved.Pages != 18 {
		t.Fatalf("saved %+v", saved)
	}
}
