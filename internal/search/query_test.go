package search

import (
	"testing"

	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/metadata"
)

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	idx, err := OpenMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

// indexDocs stores documents built from the given metadata.
func indexDocs(t *testing.T, idx *Index, docs map[string]Document) {
	t.Helper()
	for id, doc := range docs {
		if err := idx.Index(id, doc); err != nil {
			t.Fatal(err)
		}
	}
}

func meta(artist string, rating int, tags ...string) Document {
	static := metadata.Static{Artist: metadata.StringArray{artist}, Tags: tags, Collection: "col"}
	return NewDocument(static, metadata.Dynamic{Rating: rating})
}

// searchIDs runs a query-string search the way the home page does.
func searchIDs(t *testing.T, idx *Index, q string) map[string]bool {
	t.Helper()
	parsed, err := bleve.NewQueryStringQuery(q).Parse()
	if err != nil {
		t.Fatal(err)
	}

	req := bleve.NewSearchRequest(ShouldToMust(parsed))
	req.Size = 1000
	res, err := idx.Search(req)
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for _, hit := range res.Hits {
		ids[hit.ID] = true
	}
	return ids
}

func TestSearchSemantics(t *testing.T) {
	idx := newTestIndex(t)
	indexDocs(t, idx, map[string]Document{
		"bar":    meta("bar", 5, "Long Hair", "Glasses"),
		"foobar": meta("foo bar", 3, "Long Hair", "Long Coat"),
		"barbaz": meta("bar baz", 5, "Short Hair", "Glasses"),
	})

	cases := []struct {
		q    string
		want []string
	}{
		// unprefixed terms are all required (shouldToMust)
		{`tags:glasses tags:long`, []string{"bar"}},
		{`+tags:glasses +tags:long`, []string{"bar"}},
		{`tags:glasses -artist_kw:"bar"`, []string{"barbaz"}},
		// a quick filter appended to a typed term no longer overrides it
		{`artist:baz +rating:5`, []string{"barbaz"}},
		// analyzed field matches words, _kw matches whole values
		{`artist:bar`, []string{"bar", "foobar", "barbaz"}},
		{`artist_kw:"bar"`, []string{"bar"}},
		{`artist_kw:bar*`, []string{"bar", "barbaz"}},
		{`artist_kw:*bar`, []string{"bar", "foobar"}},
		// phrases stay inside one tag
		{`tags:"long hair"`, []string{"bar", "foobar"}},
		{`tags_kw:"Long Hair"`, []string{"bar", "foobar"}},
	}

	for _, c := range cases {
		got := searchIDs(t, idx, c.q)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.q, got, c.want)
			continue
		}
		for _, id := range c.want {
			if !got[id] {
				t.Errorf("%s: got %v, want %v", c.q, got, c.want)
				break
			}
		}
	}
}

func TestTermBuilders(t *testing.T) {
	if got := Term("Artist", `a "b" \c`); got != `+artist:"a \"b\" \\c"` {
		t.Errorf("Term = %s", got)
	}
	if got := TermInt("rating", "5"); got != "+rating:5" {
		t.Errorf("TermInt = %s", got)
	}
	if KeywordField("artist") != "artist_kw" || KeywordField("circle") != "circle" || KeywordField("series") != "series" {
		t.Error("KeywordField maps only fields with a _kw copy")
	}
}

func TestRecreateEmptiesTheIndex(t *testing.T) {
	idx := newTestIndex(t)
	indexDocs(t, idx, map[string]Document{"1": meta("a", 1)})

	if err := idx.Recreate(); err != nil {
		t.Fatal(err)
	}
	if got := searchIDs(t, idx, "artist:a"); len(got) != 0 {
		t.Fatalf("index not empty after Recreate: %v", got)
	}

	// still usable
	indexDocs(t, idx, map[string]Document{"2": meta("b", 1)})
	if got := searchIDs(t, idx, "artist:b"); !got["2"] {
		t.Fatalf("got %v", got)
	}
}

func TestOpenCreatesAndReopens(t *testing.T) {
	dir := t.TempDir()
	idx, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	indexDocs(t, idx, map[string]Document{"1": meta("persisted", 0)})
	if err := idx.Close(); err != nil {
		t.Fatal(err)
	}

	idx, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	if got := searchIDs(t, idx, `artist_kw:"persisted"`); !got["1"] {
		t.Fatalf("document lost after reopen: %v", got)
	}
}
