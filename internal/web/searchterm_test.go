package web

import (
	"net/url"
	"testing"
)

func TestSearchTermLinksUseKeywordFields(t *testing.T) {
	cases := map[string]string{
		"artist":     `+artist_kw:"Some \"Artist\""`,
		"tags":       `+tags_kw:"Some \"Artist\""`,
		"collection": `+collection_kw:"Some \"Artist\""`,
		"circle":     `+circle:"Some \"Artist\""`, // no _kw copy
		"series":     `+series:"Some \"Artist\""`, // keyword field itself
	}

	for key, want := range cases {
		u, err := url.Parse(NewSearchTerm(key, `Some "Artist"`).Url())
		if err != nil {
			t.Fatal(err)
		}
		if got := u.Query().Get("q"); got != want {
			t.Errorf("%s: q = %s, want %s", key, got, want)
		}
	}
}
