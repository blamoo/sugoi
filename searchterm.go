package main

import (
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strconv"
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
		q.Set("q", BuildBleveSearchTerm(t.Key, t.Label))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (t SearchTerm) Badge() template.HTML {
	return template.HTML(fmt.Sprintf(`<a class="badge bg-tag-%s text-decoration-none" href="%s">%s <small>(%d)</small></a>`, t.Key, html.EscapeString(t.Url()), html.EscapeString(t.Label), t.Count))
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
