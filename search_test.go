package main

import (
	"net/url"
	"testing"
)

// indexDocs stores bare documents (no files on disk) under the given ids.
func indexDocs(t *testing.T, docs map[string]FileMetadata) {
	t.Helper()
	for id, doc := range docs {
		doc.TagsKw = doc.Tags
		doc.ArtistKw = doc.Artist
		doc.ParodyKw = doc.Parody
		doc.MagazineKw = doc.Magazine
		doc.CollectionKw = doc.Collection
		if err := bleveIndex.Index(id, doc); err != nil {
			t.Fatal(err)
		}
	}
}

func meta(artist string, rating int, tags ...string) FileMetadata {
	var m FileMetadata
	m.Artist = StringArray{artist}
	m.Tags = tags
	m.Rating = rating
	m.Collection = "col"
	return m
}

func TestSearchSemantics(t *testing.T) {
	setupTestEnv(t)
	indexDocs(t, map[string]FileMetadata{
		"bar":    meta("bar", 5, "Long Hair", "Glasses"),
		"foobar": meta("foo bar", 3, "Long Hair", "Long Coat"),
		"barbaz": meta("bar baz", 5, "Short Hair", "Glasses"),
	})

	cases := []struct {
		q    string
		want []string
	}{
		// unprefixed terms are all required (shouldToMust)
		{`tags:glasses tags:long`, []string{"bar"}},
		{`+tags:glasses +tags:long`, []string{"bar"}},
		{`tags:glasses -artist_kw:"bar"`, []string{"barbaz"}},
		// a quick filter appended to a typed term no longer overrides it
		{`artist:baz +rating:5`, []string{"barbaz"}},
		// analyzed field matches words, _kw matches whole values
		{`artist:bar`, []string{"bar", "foobar", "barbaz"}},
		{`artist_kw:"bar"`, []string{"bar"}},
		{`artist_kw:bar*`, []string{"bar", "barbaz"}},
		{`artist_kw:*bar`, []string{"bar", "foobar"}},
		// phrases stay inside one tag
		{`tags:"long hair"`, []string{"bar", "foobar"}},
		{`tags_kw:"Long Hair"`, []string{"bar", "foobar"}},
	}

	for _, c := range cases {
		got := searchIDs(t, c.q)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.q, got, c.want)
			continue
		}
		for _, id := range c.want {
			if !got[id] {
				t.Errorf("%s: got %v, want %v", c.q, got, c.want)
				break
			}
		}
	}
}

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
