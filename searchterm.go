package main

import (
	"net/url"
)

type SearchTerm struct {
	Key   string
	Label string
}

func (t *SearchTerm) Url() string {
	u := new(url.URL)
	u.Path = "/"
	q := u.Query()
	q.Set("q", BuildBleveSearchTerm(t.Key, t.Label))
	u.RawQuery = q.Encode()
	return u.String()
}

func NewSearchTerm(key string, val string) SearchTerm {
	ret := SearchTerm{}

	ret.Key = key
	ret.Label = val

	return ret
}
