package main

import (
	"net/url"
	"strconv"
)

type SearchTermType int

const TYPE_INT SearchTermType = 0
const TYPE_TEXT SearchTermType = 1

// const TYPE_MAP SearchTermType = 2

type SearchTerm struct {
	Key   string
	Label string
	Type  SearchTermType
}

func (t *SearchTerm) Url() string {
	u := new(url.URL)
	u.Path = "/"
	q := u.Query()

	switch t.Type {
	case TYPE_INT:
		q.Set("q", BuildBleveSearchTermInt(t.Key, t.Label))

	// case TYPE_MAP:
	// 	q.Set("q", BuildBleveSearchTermMap(t.Key, t.Label))

	case TYPE_TEXT:
		q.Set("q", BuildBleveSearchTerm(t.Key, t.Label))
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func NewSearchTerm(key string, val string) SearchTerm {
	ret := SearchTerm{}

	ret.Key = key
	ret.Label = val
	ret.Type = TYPE_TEXT

	return ret
}

// func NewSearchTermMap(key string, mapKey string, val string) SearchTerm {
// 	ret := SearchTerm{}

// 	ret.Key = fmt.Sprintf("%s.%s", key, mapKey)
// 	ret.Label = val
// 	ret.Type = TYPE_MAP

// 	return ret
// }

func NewSearchTermInt(key string, val int) SearchTerm {
	ret := SearchTerm{}

	ret.Key = key
	ret.Label = strconv.Itoa(val)
	ret.Type = TYPE_INT

	return ret
}
