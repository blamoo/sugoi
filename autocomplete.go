package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/blevesearch/bleve/v2"
)

// autocompleteFields maps the field name shown to the user (and used for the
// badge color) to the untokenized index field holding whole values.
var autocompleteFields = []struct {
	Name  string
	Index string
}{
	{"tags", "tags_kw"},
	{"artist", "artist_kw"},
	{"parody", "parody_kw"},
	{"collection", "collection_kw"},
	{"series", "series"}, // already mapped as keyword
}

// autocompleteMetatags are numeric filters offered as fixed suggestions.
var autocompleteMetatags = []struct {
	Field  string
	Values []int
}{
	{"rating", []int{5, 4, 3, 2, 1, 0}},
	{"marks", []int{0}},
}

const autocompleteLimit = 10

type AutocompleteEntry struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Count  uint64 `json:"count"`
	Insert string `json:"insert"`
	lower  string
}

var autocompleteMu sync.RWMutex
var autocompleteTerms []AutocompleteEntry
var autocompleteMeta []AutocompleteEntry

// autocompleteDirty is set whenever the index changes; the cache is rebuilt on
// the next autocomplete request instead of after every single write.
var autocompleteDirty atomic.Bool

func MarkAutocompleteDirty() {
	autocompleteDirty.Store(true)
}

// autocompleteFacetSize is large enough to return every distinct value.
const autocompleteFacetSize = 1 << 20

// RebuildAutocomplete collects every value of the autocomplete fields, with
// the number of documents holding it. It uses facets over a match-all search
// rather than the term dictionary (FieldDict), whose counts still include
// deleted/updated documents until segments are merged.
func RebuildAutocomplete() error {
	autocompleteDirty.Store(false)

	req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	req.Size = 0
	for _, f := range autocompleteFields {
		req.AddFacet(f.Name, bleve.NewFacetRequest(f.Index, autocompleteFacetSize))
	}

	res, err := bleveIndex.Search(req)
	if err != nil {
		autocompleteDirty.Store(true)
		return err
	}

	var terms []AutocompleteEntry
	for _, f := range autocompleteFields {
		facet, ok := res.Facets[f.Name]
		if !ok || facet.Terms == nil {
			continue
		}

		for _, t := range facet.Terms.Terms() {
			if t.Term == "" || t.Count == 0 {
				continue
			}

			terms = append(terms, AutocompleteEntry{
				Field:  f.Name,
				Value:  t.Term,
				Count:  uint64(t.Count),
				Insert: f.Index + `:"` + inQuotesReplacer.Replace(t.Term) + `"`,
				lower:  strings.ToLower(t.Term),
			})
		}
	}

	var meta []AutocompleteEntry
	for _, m := range autocompleteMetatags {
		for _, v := range m.Values {
			val := float64(v)
			inclusive := true
			q := bleve.NewNumericRangeInclusiveQuery(&val, &val, &inclusive, &inclusive)
			q.SetField(m.Field)
			req := bleve.NewSearchRequest(q)
			req.Size = 0

			res, err := bleveIndex.Search(req)
			if err != nil {
				autocompleteDirty.Store(true)
				return err
			}

			term := m.Field + ":" + strconv.Itoa(v)
			meta = append(meta, AutocompleteEntry{
				Field:  m.Field,
				Value:  term,
				Count:  res.Total,
				Insert: term,
				lower:  term,
			})
		}
	}

	autocompleteMu.Lock()
	autocompleteTerms = terms
	autocompleteMeta = meta
	autocompleteMu.Unlock()

	debugPrintf("Autocomplete: %d terms\n", len(terms))
	return nil
}

// wordPrefixMatch reports whether any word of s after the first starts with
// prefix ("hair" matches "long hair"). The first word is the plain prefix
// case, handled separately so it ranks higher.
func wordPrefixMatch(s, prefix string) bool {
	for i, r := range s {
		if i == 0 || !(unicode.IsSpace(r) || unicode.IsPunct(r)) {
			continue
		}
		rest := s[i+1:]
		if strings.HasPrefix(rest, prefix) {
			return true
		}
	}
	return false
}

// Autocomplete returns suggestions for one search term as typed by the user,
// without the leading +/- (e.g. "big bre", "artist:bif", "rating:").
func Autocomplete(term string) []AutocompleteEntry {
	term = strings.ToLower(strings.TrimSpace(term))
	field := ""
	value := term

	if before, after, ok := strings.Cut(term, ":"); ok {
		field = strings.TrimSuffix(before, "_kw")
		value = after
	}
	value = strings.Trim(value, `"`)

	autocompleteMu.RLock()
	defer autocompleteMu.RUnlock()

	var ret []AutocompleteEntry

	// Metatags: offered once the typed text is a prefix of "rating:N"/"marks:N".
	if len(term) >= 2 {
		for _, m := range autocompleteMeta {
			if strings.HasPrefix(m.lower, term) {
				ret = append(ret, m)
			}
		}
	}

	if field != "" {
		known := false
		for _, f := range autocompleteFields {
			if f.Name == field {
				known = true
				break
			}
		}
		if !known {
			// Not a field: the colon is part of a value, as in "Re:Start".
			field = ""
			value = strings.Trim(term, `"`)
		}
	}

	if field == "" && value == "" {
		return limitAutocomplete(ret)
	}

	var prefix, words []AutocompleteEntry
	for _, t := range autocompleteTerms {
		if field != "" && t.Field != field {
			continue
		}

		if strings.HasPrefix(t.lower, value) {
			prefix = append(prefix, t)
		} else if wordPrefixMatch(t.lower, value) {
			words = append(words, t)
		}
	}

	byCount := func(list []AutocompleteEntry) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].lower < list[j].lower
		})
	}
	byCount(prefix)
	byCount(words)

	ret = append(ret, prefix...)
	ret = append(ret, words...)
	return limitAutocomplete(ret)
}

func limitAutocomplete(list []AutocompleteEntry) []AutocompleteEntry {
	if list == nil {
		return []AutocompleteEntry{}
	}
	if len(list) > autocompleteLimit {
		return list[:autocompleteLimit]
	}
	return list
}

func RouteAutocomplete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if _, ret := CheckAnyAuth(w, r); ret {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(JsonResponse{"Unauthorized"})
		return
	}

	// While a full reindex runs the index is being replaced; keep serving the
	// previous cache.
	if autocompleteDirty.Load() && !reindexJob.Running {
		err := RebuildAutocomplete()
		if err != nil {
			log.Println("autocomplete rebuild:", err)
		}
	}

	json.NewEncoder(w).Encode(Autocomplete(r.FormValue("q")))
}
