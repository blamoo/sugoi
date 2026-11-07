package main

import (
	"fmt"
	"testing"
)

func autocompleteValues(entries []AutocompleteEntry) []string {
	var ret []string
	for _, e := range entries {
		ret = append(ret, e.Field+":"+e.Value)
	}
	return ret
}

func TestAutocompleteFromIndex(t *testing.T) {
	setupTestEnv(t)
	indexDocs(t, map[string]FileMetadata{
		"1": meta("bar", 5, "Long Hair", "Glasses"),
		"2": meta("foo bar", 0, "Long Hair", "Long Coat"),
		"3": meta("bar baz", 0, "Short Hair", "Glasses"),
	})

	if err := RebuildAutocomplete(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		q    string
		want []string
	}{
		// prefix matches first, then word matches; by count, then name
		{"bar", []string{"artist:bar", "artist:bar baz", "artist:foo bar"}},
		{"ha", []string{"tags:Long Hair", "tags:Short Hair"}},
		{"LONG H", []string{"tags:Long Hair"}},
		{"artist:ba", []string{"artist:bar", "artist:bar baz", "artist:foo bar"}},
		{"tags:", []string{"tags:Glasses", "tags:Long Hair", "tags:Long Coat", "tags:Short Hair"}},
		{"rating:5", []string{"rating:rating:5"}},
		{"ma", []string{"marks:marks:0"}},
		{"zzz", nil},
		{"", nil},
		// unknown "field": the colon is part of the value
		{"nofield:x", nil},
	}

	for _, c := range cases {
		got := autocompleteValues(Autocomplete(c.q))
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%q: got %v, want %v", c.q, got, c.want)
		}
	}

	// counts come from live documents only
	for _, e := range Autocomplete("glas") {
		if e.Value == "Glasses" && e.Count != 2 {
			t.Errorf("Glasses count = %d, want 2", e.Count)
		}
	}
	for _, e := range Autocomplete("rating:5") {
		if e.Count != 1 {
			t.Errorf("rating:5 count = %d, want 1", e.Count)
		}
	}

	// the inserted text is a valid exact search
	first := Autocomplete("bar")[0]
	if first.Insert != `artist_kw:"bar"` {
		t.Errorf("insert = %s", first.Insert)
	}
	if ids := searchIDs(t, first.Insert); len(ids) != 1 || !ids["1"] {
		t.Errorf("%s matched %v", first.Insert, ids)
	}
}

func TestAutocompleteCountsIgnoreReplacedDocs(t *testing.T) {
	setupTestEnv(t)
	indexDocs(t, map[string]FileMetadata{"1": meta("old name", 0)})
	indexDocs(t, map[string]FileMetadata{"1": meta("new name", 0)})

	if err := RebuildAutocomplete(); err != nil {
		t.Fatal(err)
	}

	if got := autocompleteValues(Autocomplete("old")); got != nil {
		t.Errorf("replaced value still suggested: %v", got)
	}
	if got := autocompleteValues(Autocomplete("new")); fmt.Sprint(got) != "[artist:new name]" {
		t.Errorf("got %v", got)
	}
}

func TestAutocompleteLimit(t *testing.T) {
	setupTestEnv(t)
	docs := map[string]FileMetadata{}
	for i := 0; i < 30; i++ {
		docs[fmt.Sprint(i)] = meta(fmt.Sprintf("artist %02d", i), 0)
	}
	indexDocs(t, docs)

	if err := RebuildAutocomplete(); err != nil {
		t.Fatal(err)
	}
	if n := len(Autocomplete("artist")); n != autocompleteLimit {
		t.Errorf("got %d suggestions, want %d", n, autocompleteLimit)
	}
}

func TestWordPrefixMatch(t *testing.T) {
	cases := []struct {
		s, prefix string
		want      bool
	}{
		{"long hair", "hair", true},
		{"long hair", "long", false}, // first word is the plain prefix case
		{"re:start", "start", true},
		{"o.p", "p", true},
		{"glasses", "ass", false},
	}
	for _, c := range cases {
		if got := wordPrefixMatch(c.s, c.prefix); got != c.want {
			t.Errorf("wordPrefixMatch(%q, %q) = %v", c.s, c.prefix, got)
		}
	}
}
