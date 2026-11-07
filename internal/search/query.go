package search

import (
	"slices"
	"strings"

	"github.com/blevesearch/bleve/v2/search/query"
)

var inQuotesReplacer = strings.NewReplacer(
	"\"", "\\\"",
	"\\", "\\\\",
)

// Quote returns val as a query-string phrase, escaping quotes and backslashes.
func Quote(val string) string {
	return `"` + inQuotesReplacer.Replace(val) + `"`
}

// Term builds a required exact-value term: +field:"value".
func Term(field string, val string) string {
	return "+" + strings.ToLower(field) + ":" + Quote(val)
}

// TermInt builds a required numeric term: +field:value.
func TermInt(field string, val string) string {
	return "+" + strings.ToLower(field) + ":" + inQuotesReplacer.Replace(val)
}

// KeywordField maps a field to its untokenized "_kw" copy when one exists, so
// a link to artist "bar" matches only "bar" and not "foo bar" or "bar baz".
func KeywordField(field string) string {
	if slices.Contains(KeywordFields, strings.ToLower(field)) {
		return field + "_kw"
	}
	return field
}

// ShouldToMust makes unprefixed query-string terms required, so "A B" means
// A AND B instead of Bleve's default (optional terms that only affect score).
// Explicit "+" and "-" terms keep their meaning.
func ShouldToMust(q query.Query) query.Query {
	bq, ok := q.(*query.BooleanQuery)
	if !ok || bq.Should == nil {
		return q
	}

	dq, ok := bq.Should.(*query.DisjunctionQuery)
	if !ok {
		return q
	}

	bq.AddMust(dq.Disjuncts...)
	bq.Should = nil
	return bq
}
