package main

import (
	"fmt"
	"html"
	"html/template"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type SearchTermType int

const TYPE_INT SearchTermType = 0
const TYPE_TEXT SearchTermType = 1

type SearchTerm struct {
	Key    string
	Label  string
	Type   SearchTermType
	Count  int
	Weight int
}

func (t SearchTerm) Url() string {
	u := new(url.URL)
	u.Path = "/"
	q := u.Query()

	switch t.Type {
	case TYPE_INT:
		q.Set("q", BuildBleveSearchTermInt(t.Key, t.Label))

	case TYPE_TEXT:
		q.Set("q", BuildBleveSearchTerm(searchField(t.Key), t.Label))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (t SearchTerm) Badge() template.HTML {
	if t.Count == 0 {
		return template.HTML(fmt.Sprintf(`<a class="badge bg-tag-%s text-decoration-none" href="%s">%s</a>`, t.Key, html.EscapeString(t.Url()), html.EscapeString(t.Label)))
	}
	return template.HTML(fmt.Sprintf(`<a class="badge bg-tag-%s text-decoration-none" href="%s">%s <small>(%d)</small></a>`, t.Key, html.EscapeString(t.Url()), html.EscapeString(t.Label), t.Count))
}

// searchField maps a field to its untokenized "_kw" copy when one exists, so a
// link to artist "bar" matches only "bar" and not "foo bar" or "bar baz".
func searchField(key string) string {
	if slices.Contains(metadataKeywordsFields, strings.ToLower(key)) {
		return key + "_kw"
	}
	return key
}

func NewSearchTerm(key string, val string) SearchTerm {
	ret := SearchTerm{}

	ret.Key = key
	ret.Label = val
	ret.Type = TYPE_TEXT

	return ret
}

func NewSearchTermInt(key string, val int) SearchTerm {
	ret := SearchTerm{}

	ret.Key = key
	ret.Label = strconv.Itoa(val)
	ret.Type = TYPE_INT

	return ret
}
