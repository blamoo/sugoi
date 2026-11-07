package main

import (
	"github.com/blamoo/sugoi/internal/metadata"
)

// Temporary aliases while the code moves into internal packages.
type FileMetadataStatic = metadata.Static
type FileMetadataDynamic = metadata.Dynamic
type StringArray = metadata.StringArray
type IdArray = metadata.IdArray

var metadataKeywordsFields = []string{
	"tags",
	"artist",
	"parody",
	"magazine",
	"collection",
}

type KeywordsMetadata struct {
	CollectionKw string      `json:"collection_kw" schema:"collection_kw"`
	TagsKw       StringArray `json:"tags_kw" schema:"tags_kw"`
	ArtistKw     StringArray `json:"artist_kw" schema:"artist_kw"`
	ParodyKw     StringArray `json:"parody_kw" schema:"parody_kw"`
	MagazineKw   StringArray `json:"magazine_kw" schema:"magazine_kw"`
}

type FileMetadata struct {
	FileMetadataStatic
	FileMetadataDynamic
	KeywordsMetadata
	Random map[string]int `json:"random,omitempty"`
	Url    StringArray    `json:"url,omitempty"`
}

// fillStaticPlaceholders fills an empty title/collection from the file name and
// the page count from the archive. Applied to in-memory copies only.
func fillStaticPlaceholders(fms *FileMetadataStatic, file *FilePointer) {
	if file == nil {
		return
	}

	if len(fms.Title) == 0 {
		fms.Title = file.PlaceholderTitle()
	}

	if len(fms.Collection) == 0 {
		fms.Collection = file.PlaceholderCollection()
	}

	if fms.Pages == 0 {
		f, err := filePointers.Pages(file)
		if err == nil {
			fms.Pages = len(f)
		}
	}
}
