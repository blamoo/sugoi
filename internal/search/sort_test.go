package search

import (
	"fmt"
	"slices"
	"testing"

	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/metadata"
)

// sortedIDs returns every document ID in the order of sort.
func sortedIDs(t *testing.T, idx *Index, sort ...string) []string {
	t.Helper()
	req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	req.SortBy(sort)
	req.Size = 100
	res, err := idx.Search(req)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, h := range res.Hits {
		ids = append(ids, h.ID)
	}
	return ids
}

func TestSortByTitleAndCollection(t *testing.T) {
	idx := newTestIndex(t)
	docs := map[string]metadata.Static{
		"zulu":   {Title: "Zulu Alpha", Collection: "COL Zulu"},
		"banana": {Title: "banana", Collection: "col banana"},
		"apple":  {Title: "Apple", Collection: "Col Mango"},
		"pie":    {Title: "apple pie", Collection: "Col Kiwi"},
	}
	for id, static := range docs {
		if err := idx.Index(id, NewDocument(static, metadata.Dynamic{})); err != nil {
			t.Fatal(err)
		}
	}

	// whole value, ignoring case: "Zulu Alpha" isn't sorted by "alpha", and
	// "Apple" doesn't go before "banana" just for being uppercase
	if got, want := sortedIDs(t, idx, "title_sort", "_id"), []string{"apple", "pie", "banana", "zulu"}; !slices.Equal(got, want) {
		t.Errorf("by title = %v, want %v", got, want)
	}
	if got, want := sortedIDs(t, idx, "-title_sort", "_id"), []string{"zulu", "banana", "pie", "apple"}; !slices.Equal(got, want) {
		t.Errorf("by title desc = %v, want %v", got, want)
	}
	// every collection shares the token "col": the analyzed field can't sort these
	if got, want := sortedIDs(t, idx, "collection_sort", "_id"), []string{"banana", "pie", "apple", "zulu"}; !slices.Equal(got, want) {
		t.Errorf("by collection = %v, want %v", got, want)
	}

	// sort keys stay out of _all: a match on "zulu alpha" as one term finds nothing
	res, err := idx.Search(bleve.NewSearchRequest(bleve.NewTermQuery("zulu alpha")))
	if err != nil || res.Total != 0 {
		t.Errorf("sort key found through _all: %d hits, %v", res.Total, err)
	}
}

func TestSortTieBreakIsStable(t *testing.T) {
	// on disk like the real index: the in-memory one happens to keep ties
	// in ID order and doesn't show the problem
	idx, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })
	index := func(id string) {
		doc := NewDocument(metadata.Static{Title: "same"}, metadata.Dynamic{Rating: 3})
		if err := idx.Index(id, doc); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 5 {
		index(fmt.Sprintf("doc%d", i))
	}
	// Re-indexing an item (as any rating or mark change does) moves it to
	// the end of Bleve's natural order, which is what breaks ties without an
	// explicit tie-break: these two would jump to the last positions.
	index("doc1")
	index("doc3")

	want := []string{"doc0", "doc1", "doc2", "doc3", "doc4"}
	if got := sortedIDs(t, idx, "-rating", "_id"); !slices.Equal(got, want) {
		t.Errorf("equal ratings = %v, want by ID %v", got, want)
	}
}
