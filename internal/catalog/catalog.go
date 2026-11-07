// Package catalog ties the library (which archives exist), their metadata
// and the search index together: it loads items, applies changes to them
// under a per-item lock and keeps the index in sync, including full
// reindexes.
package catalog

import (
	"sync/atomic"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"
	"github.com/blamoo/sugoi/internal/search"
)

type Catalog struct {
	cfg          *config.Config
	lib          atomic.Pointer[library.Library]
	Index        *search.Index
	Autocomplete *search.Autocompleter
	Reindex      ReindexJob
}

func New(cfg *config.Config, lib *library.Library, index *search.Index) *Catalog {
	c := &Catalog{
		cfg:          cfg,
		Index:        index,
		Autocomplete: search.NewAutocompleter(index),
	}
	c.lib.Store(lib)
	c.Reindex.catalog = c
	return c
}

// Library returns the current file list.
func (c *Catalog) Library() *library.Library {
	return c.lib.Load()
}

// Reload re-reads files.txt and swaps the file list in. On error the current
// list is kept.
func (c *Catalog) Reload() (previous int, current int, err error) {
	lib, err := library.Load(c.cfg)
	if err != nil {
		return 0, 0, err
	}

	old := c.lib.Swap(lib)
	return len(old.List), len(lib.List), nil
}

// Get loads an item and its metadata. Missing or unreadable metadata files
// read as empty; placeholders (title, collection, page count) are filled in.
func (c *Catalog) Get(hash string) (*Thing, error) {
	file, err := c.Library().Find(hash)
	if err != nil {
		return nil, err
	}

	t := &Thing{File: file, catalog: c}

	static, err := metadata.ReadStatic(file.StaticMetaPath())
	if err == nil {
		t.Static = *static
	} else {
		logMetadataError(file.StaticMetaPath(), err)
	}
	c.fillPlaceholders(&t.Static, file)

	dynamic, err := metadata.ReadDynamic(file.DynamicMetaPath())
	if err == nil {
		t.Dynamic = *dynamic
	} else {
		logMetadataError(file.DynamicMetaPath(), err)
	}

	return t, nil
}

// fillPlaceholders fills an empty title/collection from the file name and
// the page count from the archive. Only for in-memory copies, never saved.
func (c *Catalog) fillPlaceholders(s *metadata.Static, file *library.File) {
	if len(s.Title) == 0 {
		s.Title = file.PlaceholderTitle()
	}

	if len(s.Collection) == 0 {
		s.Collection = file.PlaceholderCollection()
	}

	if s.Pages == 0 {
		pages, err := c.Library().Pages(file)
		if err == nil {
			s.Pages = len(pages)
		}
	}
}

// document builds the indexed document of a file from its metadata on disk.
func (c *Catalog) document(file *library.File) search.Document {
	static, err := metadata.ReadStatic(file.StaticMetaPath())
	if err != nil {
		logMetadataError(file.StaticMetaPath(), err)
		static = &metadata.Static{}
	}
	c.fillPlaceholders(static, file)

	dynamic, err := metadata.ReadDynamic(file.DynamicMetaPath())
	if err != nil {
		logMetadataError(file.DynamicMetaPath(), err)
		dynamic = &metadata.Dynamic{}
	}

	return search.NewDocument(*static, *dynamic)
}

// reindex updates one item in the search index.
func (c *Catalog) reindex(file *library.File) error {
	err := c.Index.Index(file.Hash, c.document(file))
	if err != nil {
		return err
	}

	c.Autocomplete.MarkDirty()
	return nil
}
