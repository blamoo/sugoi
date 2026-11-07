// Package search wraps the Bleve full-text index: the mapping, the indexed
// document, query-string handling and the autocomplete suggestions.
package search

import (
	"os"
	"path"
	"sync"

	"github.com/blevesearch/bleve/v2"
)

// Index is the search index. It can be dropped and recreated (full reindex)
// while requests use it: Recreate waits for in-flight calls and swaps the
// underlying index, instead of reassigning a variable other goroutines read.
type Index struct {
	mu  sync.RWMutex
	idx bleve.Index

	// dir is empty for in-memory indexes (tests).
	dir string
}

// Open opens <databaseDir>/sugoi.bleve, creating it with the current mapping
// when it doesn't exist.
func Open(databaseDir string) (*Index, error) {
	dir := path.Join(databaseDir, "sugoi.bleve")
	idx, err := openOrCreate(dir)
	if err != nil {
		return nil, err
	}
	return &Index{idx: idx, dir: dir}, nil
}

// OpenMem creates an empty in-memory index with the real mapping.
func OpenMem() (*Index, error) {
	idx, err := bleve.NewMemOnly(BuildMapping())
	if err != nil {
		return nil, err
	}
	return &Index{idx: idx}, nil
}

func openOrCreate(dir string) (bleve.Index, error) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return bleve.New(dir, BuildMapping())
	}
	if err != nil {
		return nil, err
	}
	return bleve.Open(dir)
}

// Recreate deletes every document by replacing the index with an empty one
// built from the current mapping.
func (i *Index) Recreate() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	err := i.idx.Close()
	if err != nil {
		return err
	}

	if i.dir == "" {
		i.idx, err = bleve.NewMemOnly(BuildMapping())
		return err
	}

	err = os.RemoveAll(i.dir)
	if err != nil {
		return err
	}
	i.idx, err = bleve.New(i.dir, BuildMapping())
	return err
}

func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.idx.Close()
}

func (i *Index) Search(req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.idx.Search(req)
}

// Index stores (or replaces) one document.
func (i *Index) Index(id string, doc Document) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.idx.Index(id, doc)
}

// NewBatch starts a batch for the current index; commit it with Batch.
func (i *Index) NewBatch() *bleve.Batch {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.idx.NewBatch()
}

func (i *Index) Batch(b *bleve.Batch) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.idx.Batch(b)
}

// Fields lists the indexed field names.
func (i *Index) Fields() ([]string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.idx.Fields()
}
