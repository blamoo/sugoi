package search

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
)

// KeywordFields get an untokenized "<field>_kw" copy for exact matches
// (artist_kw:"bar" doesn't match "foo bar") and facets.
var KeywordFields = []string{
	"tags",
	"artist",
	"parody",
	"magazine",
	"collection",
}

// BuildMapping is the index mapping. It only applies when an index is
// created, so changing it requires a full reindex.
func BuildMapping() *mapping.IndexMappingImpl {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddCustomAnalyzer("simpleUnicode", map[string]any{
		"type":      custom.Name,
		"tokenizer": unicode.Name,
		"token_filters": []string{
			lowercase.Name,
		},
	})
	indexMapping.DefaultAnalyzer = "simpleUnicode"

	thingMapping := bleve.NewDocumentMapping()
	indexMapping.DefaultMapping = thingMapping

	textFieldMapping := bleve.NewTextFieldMapping()
	numericFieldMapping := bleve.NewNumericFieldMapping()
	dateTimeFieldMapping := bleve.NewDateTimeFieldMapping()
	disabledMapping := bleve.NewDocumentDisabledMapping()
	keywordMapping := bleve.NewKeywordFieldMapping()

	thingMapping.AddFieldMappingsAt("title", textFieldMapping)
	thingMapping.AddFieldMappingsAt("tags", textFieldMapping)
	thingMapping.AddFieldMappingsAt("circle", textFieldMapping)
	thingMapping.AddFieldMappingsAt("artist", textFieldMapping)
	thingMapping.AddFieldMappingsAt("collection", textFieldMapping)
	thingMapping.AddFieldMappingsAt("cover", textFieldMapping)
	thingMapping.AddFieldMappingsAt("description", textFieldMapping)
	thingMapping.AddFieldMappingsAt("id", numericFieldMapping)
	thingMapping.AddFieldMappingsAt("id_source", textFieldMapping)
	thingMapping.AddFieldMappingsAt("language", textFieldMapping)
	thingMapping.AddFieldMappingsAt("magazine", textFieldMapping)
	thingMapping.AddFieldMappingsAt("parody", textFieldMapping)
	thingMapping.AddFieldMappingsAt("publisher", textFieldMapping)
	thingMapping.AddFieldMappingsAt("rating", numericFieldMapping)
	thingMapping.AddFieldMappingsAt("marks", numericFieldMapping)
	thingMapping.AddFieldMappingsAt("type", textFieldMapping)
	thingMapping.AddFieldMappingsAt("updated_at", dateTimeFieldMapping)
	thingMapping.AddFieldMappingsAt("series", keywordMapping)
	thingMapping.AddFieldMappingsAt("url", keywordMapping)

	thingMapping.AddSubDocumentMapping("files", disabledMapping)
	thingMapping.AddSubDocumentMapping("urls", disabledMapping)

	for _, name := range KeywordFields {
		thingMapping.AddFieldMappingsAt(name+"_kw", keywordMapping)
	}

	return indexMapping
}
