package search

import (
	"math/rand/v2"
	"strconv"

	"github.com/blamoo/sugoi/internal/metadata"
)

// RandomPoolSize is the number of random sort keys stored per document; a
// random search sorts by one of them ("random.<n>").
const RandomPoolSize = 10

// Keywords are the untokenized copies of KeywordFields.
type Keywords struct {
	CollectionKw string               `json:"collection_kw"`
	TagsKw       metadata.StringArray `json:"tags_kw"`
	ArtistKw     metadata.StringArray `json:"artist_kw"`
	ParodyKw     metadata.StringArray `json:"parody_kw"`
	MagazineKw   metadata.StringArray `json:"magazine_kw"`
}

// Document is what gets indexed for one item.
type Document struct {
	metadata.Static
	metadata.Dynamic
	Keywords
	Random map[string]int       `json:"random,omitempty"`
	Url    metadata.StringArray `json:"url,omitempty"`
}

// NewDocument builds the indexed document from an item's metadata (with
// placeholders already filled in).
func NewDocument(static metadata.Static, dynamic metadata.Dynamic) Document {
	doc := Document{Static: static, Dynamic: dynamic}

	doc.Random = make(map[string]int, RandomPoolSize)
	for i := range RandomPoolSize {
		doc.Random[strconv.Itoa(i)] = rand.Int()
	}

	doc.Url = append(doc.Url, doc.Urls...)

	doc.CollectionKw = doc.Collection
	doc.TagsKw = doc.Tags
	doc.ArtistKw = doc.Artist
	doc.ParodyKw = doc.Parody
	doc.MagazineKw = doc.Magazine

	return doc
}
