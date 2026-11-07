package main

import (
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// setupTestEnv points the package globals (config, file list, search index)
// at a temporary database with an in-memory index using the real mapping, and
// restores them when the test ends. The given keys become the file list.
func setupTestEnv(t *testing.T, keys ...string) {
	t.Helper()

	oldConfig, oldPointers, oldIndex := config, filePointers, bleveIndex
	t.Cleanup(func() {
		if bleveIndex != nil && bleveIndex != oldIndex {
			bleveIndex.Close()
		}
		config, filePointers, bleveIndex = oldConfig, oldPointers, oldIndex
		reindexJob.Running = false
	})

	dir := t.TempDir()
	config = SugoiConfig{
		DatabaseDir: dir,
		CacheDir:    t.TempDir(),
		DirVars:     map[string]string{"LIB": dir + "/library"},
	}

	idx, err := bleve.NewMemOnly(BuildNewMapping())
	if err != nil {
		t.Fatal(err)
	}
	bleveIndex = idx

	filePointers = NewFilePointerList()
	for _, k := range keys {
		fp, err := NewFilePointer(k)
		if err != nil {
			t.Fatal(err)
		}
		filePointers.Push(fp)
	}
}

// mustThing returns the item for a key registered by setupTestEnv.
func mustThing(t *testing.T, key string) *Thing {
	t.Helper()
	fp, _ := NewFilePointer(key)
	thing, err := NewThingFromHash(fp.Hash)
	if err != nil {
		t.Fatal(err)
	}
	return thing
}

// searchIDs runs a query-string search the way RouteRoot does and returns
// the matching document ids.
func searchIDs(t *testing.T, q string) map[string]bool {
	t.Helper()
	parsed, err := bleve.NewQueryStringQuery(q).Parse()
	if err != nil {
		t.Fatal(err)
	}

	req := bleve.NewSearchRequest(shouldToMust(parsed))
	req.Size = 1000
	res, err := bleveIndex.Search(req)
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for _, hit := range res.Hits {
		ids[hit.ID] = true
	}
	return ids
}
