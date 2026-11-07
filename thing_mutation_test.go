package main

import (
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/blamoo/sugoi/internal/metadata"
)

const testKey = "{{LIB}}/collection/[Artist] Some Title.cbz"

func TestConcurrentMarksAreNotLost(t *testing.T) {
	setupTestEnv(t, testKey)

	const n = 40
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// each request loads its own copy, like the HTTP handlers
			errs <- mustThing(t, testKey).AddMark()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := mustThing(t, testKey).Marks; got != n {
		t.Fatalf("marks = %d, want %d", got, n)
	}
}

func TestRatingToggle(t *testing.T) {
	setupTestEnv(t, testKey)

	thing := mustThing(t, testKey)
	if err := thing.TrySaveRating(4, true); err != nil {
		t.Fatal(err)
	}
	if thing.Rating != 4 {
		t.Fatalf("rating = %d, want 4", thing.Rating)
	}

	// toggling the same rating clears it, based on what is on disk
	if err := mustThing(t, testKey).TrySaveRating(4, true); err != nil {
		t.Fatal(err)
	}
	if got := mustThing(t, testKey).Rating; got != 0 {
		t.Fatalf("rating = %d, want 0 after toggle", got)
	}

	// mutations update the search index
	if err := mustThing(t, testKey).TrySaveRating(5, false); err != nil {
		t.Fatal(err)
	}
	if ids := searchIDs(t, "rating:5"); len(ids) != 1 {
		t.Fatalf("rating:5 matched %v", ids)
	}
}

func TestWritesRefusedDuringReindex(t *testing.T) {
	setupTestEnv(t, testKey)
	thing := mustThing(t, testKey)

	reindexJob.Running = true
	defer func() { reindexJob.Running = false }()

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
	setupTestEnv(t, testKey)
	thing := mustThing(t, testKey)

	// as written by the importer: id_source is an array
	imported := `{"id":[1970],"id_source":["/item/1970/abc"],"circle":["Circle"],"title":"Old","pages":18}`
	if err := os.MkdirAll(thing.File.MetaPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thing.File.StaticMetaPath(), []byte(imported), 0644); err != nil {
		t.Fatal(err)
	}

	if err := mustThing(t, testKey).SaveStaticFromForm(url.Values{"title": {"New"}}); err != nil {
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
