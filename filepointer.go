package main

import (
	"math/rand/v2"
	"strconv"

	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"
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

	MarkAutocompleteDirty()
	return nil
}

func buildReindexDoc(fp *FilePointer) FileMetadata {
	var err error
	var file FileMetadata
	static, err := metadata.ReadStatic(fp.StaticMetaPath())
	if err != nil {
		file.FileMetadataStatic = FileMetadataStatic{}
	} else {
		file.FileMetadataStatic = *static
	}
	fillStaticPlaceholders(&file.FileMetadataStatic, fp)

	dynamic, err := metadata.ReadDynamic(fp.DynamicMetaPath())
	if err != nil {
		file.FileMetadataDynamic = FileMetadataDynamic{}
	} else {
		file.FileMetadataDynamic = *dynamic
	}

	file.Random = make(map[string]int, RANDOM_POOL_SIZE)
	for i := range RANDOM_POOL_SIZE {
		k := strconv.Itoa(i)
		file.Random[k] = rand.Int()
	}

	for _, url := range file.Urls {
		file.Url = append(file.Url, url)
	}

	file.CollectionKw = file.Collection
	file.TagsKw = file.Tags
	file.ArtistKw = file.Artist
	file.ParodyKw = file.Parody
	file.MagazineKw = file.Magazine

	return file
}
