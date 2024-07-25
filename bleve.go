package main

import (
	"os"
	"path"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/mapping"
)

var inQuotesReplacer *strings.Replacer

func init() {
	inQuotesReplacer = strings.NewReplacer(
		"\"", "\\\"",
		"\\", "\\\\",
	)
}

func InitializeBleve() error {
	path := path.Join(config.DatabaseDir, "sugoi.bleve")
	var err error

	stat, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if stat == nil {
		mapping := BuildNewMapping()
		bleveIndex, err = bleve.New(path, mapping)
	} else {
		bleveIndex, err = bleve.Open(path)
	}

	if err != nil {
		return err
	}

	return nil
}

func BuildNewMapping() *mapping.IndexMappingImpl {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.AddCustomAnalyzer("simpleUnicode", map[string]interface{}{
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

	thingMapping.AddSubDocumentMapping("files", disabledMapping)
	thingMapping.AddSubDocumentMapping("metadataSources", disabledMapping)

	return indexMapping
}

func BuildBleveSearchTerm(key string, val string) string {
	sb := strings.Builder{}

	sb.WriteString("+")
	sb.WriteString(strings.ToLower(key))
	sb.WriteString(`:"`)
	sb.WriteString(inQuotesReplacer.Replace(val))
	sb.WriteString(`"`)

	return sb.String()
}

func BuildBleveSearchTermMap(key string, val string) string {
	sb := strings.Builder{}

	spl := strings.SplitN(key, ".", 2)

	sb.WriteString("+")
	if len(spl) == 2 {
		sb.WriteString(strings.ToLower(spl[0]))
		sb.WriteString(".")
		sb.WriteString(spl[1])
	} else {
		sb.WriteString(strings.ToLower(key))
	}
	sb.WriteString(`:"`)
	sb.WriteString(inQuotesReplacer.Replace(val))
	sb.WriteString(`"`)

	return sb.String()
}

func BuildBleveSearchTermInt(key string, val string) string {
	sb := strings.Builder{}

	sb.WriteString("+")
	sb.WriteString(strings.ToLower(key))
	sb.WriteString(":")
	sb.WriteString(inQuotesReplacer.Replace(val))

	return sb.String()
}
