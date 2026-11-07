package search

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
	idx := newTestIndex(t)
	indexDocs(t, idx, map[string]Document{
		"1": meta("bar", 5, "Long Hair", "Glasses"),
		"2": meta("foo bar", 0, "Long Hair", "Long Coat"),
		"3": meta("bar baz", 0, "Short Hair", "Glasses"),
	})

	a := NewAutocompleter(idx)
	if err := a.Rebuild(); err != nil {
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
		got := autocompleteValues(a.Suggest(c.q, false))
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%q: got %v, want %v", c.q, got, c.want)
		}
	}

	// counts come from live documents only
	for _, e := range a.Suggest("glas", false) {
		if e.Value == "Glasses" && e.Count != 2 {
			t.Errorf("Glasses count = %d, want 2", e.Count)
		}
	}
	for _, e := range a.Suggest("rating:5", false) {
		if e.Count != 1 {
			t.Errorf("rating:5 count = %d, want 1", e.Count)
		}
	}

	// the inserted text is a valid exact search
	first := a.Suggest("bar", false)[0]
	if first.Insert != `artist_kw:"bar"` {
		t.Errorf("insert = %s", first.Insert)
	}
	if ids := searchIDs(t, idx, first.Insert); len(ids) != 1 || !ids["1"] {
		t.Errorf("%s matched %v", first.Insert, ids)
	}
}

func TestAutocompleteCountsIgnoreReplacedDocs(t *testing.T) {
	idx := newTestIndex(t)
	indexDocs(t, idx, map[string]Document{"1": meta("old name", 0)})
	indexDocs(t, idx, map[string]Document{"1": meta("new name", 0)})

	a := NewAutocompleter(idx)
	if err := a.Rebuild(); err != nil {
		t.Fatal(err)
	}

	if got := autocompleteValues(a.Suggest("old", false)); got != nil {
		t.Errorf("replaced value still suggested: %v", got)
	}
	if got := autocompleteValues(a.Suggest("new", false)); fmt.Sprint(got) != "[artist:new name]" {
		t.Errorf("got %v", got)
	}
}

func TestAutocompleteLimit(t *testing.T) {
	idx := newTestIndex(t)
	docs := map[string]Document{}
	for i := 0; i < 30; i++ {
		docs[fmt.Sprint(i)] = meta(fmt.Sprintf("artist %02d", i), 0)
	}
	indexDocs(t, idx, docs)

	a := NewAutocompleter(idx)
	if err := a.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if n := len(a.Suggest("artist", false)); n != autocompleteLimit {
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

func TestSuggestRebuildsWhenDirty(t *testing.T) {
	idx := newTestIndex(t)
	a := NewAutocompleter(idx)
	if err := a.Rebuild(); err != nil {
		t.Fatal(err)
	}

	indexDocs(t, idx, map[string]Document{"1": meta("late artist", 0)})
	a.MarkDirty()

	if got := autocompleteValues(a.Suggest("late", false)); got != nil {
		t.Errorf("rebuild=false must keep the old list, got %v", got)
	}
	if got := autocompleteValues(a.Suggest("late", true)); fmt.Sprint(got) != "[artist:late artist]" {
		t.Errorf("got %v", got)
	}
}
