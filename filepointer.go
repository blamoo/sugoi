package main

import (
	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"

	"github.com/blamoo/sugoi/internal/search"
)

// Temporary alias while the code moves into internal packages.
type FilePointer = library.File

// InitializeFilePointers (re)loads files.txt. On error the current list is
// kept.
func InitializeFilePointers() error {
	lib, err := library.Load(&config)
	if err != nil {
		return err
	}

	filePointers = lib
	return nil
}

func reindexFileIntoBatch(fp *FilePointer, idx *bleve.Batch) error {
	doc := buildReindexDoc(fp)

	err := idx.Index(fp.Hash, doc)
	if err != nil {
		return err
	}

	return nil
}

func reindexFile(fp *FilePointer) error {
	doc := buildReindexDoc(fp)

	err := bleveIndex.Index(fp.Hash, doc)
	if err != nil {
		return err
	}

	autocompleter.MarkDirty()
	return nil
}

func buildReindexDoc(fp *FilePointer) search.Document {
	static, err := metadata.ReadStatic(fp.StaticMetaPath())
	if err != nil {
		static = &metadata.Static{}
	}
	fillStaticPlaceholders(static, fp)

	dynamic, err := metadata.ReadDynamic(fp.DynamicMetaPath())
	if err != nil {
		dynamic = &metadata.Dynamic{}
	}

	return search.NewDocument(*static, *dynamic)
}
