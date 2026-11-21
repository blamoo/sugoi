package search

import (
	"reflect"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

func TestParseQueryString(t *testing.T) {
	tests := []struct {
		q    string
		want []Clause
	}{
		{"", nil},
		{"   ", nil},
		{"landscape", []Clause{{Raw: "landscape", Value: "landscape"}}},
		{"rating:5", []Clause{{Raw: "rating:5", Field: "rating", Value: "5"}}},
		{"+rating:5", []Clause{{Raw: "+rating:5", Prefix: "+", Field: "rating", Value: "5"}}},
		{"-marks:0", []Clause{{Raw: "-marks:0", Prefix: "-", Field: "marks", Value: "0"}}},
		{"rating:-1", []Clause{{Raw: "rating:-1", Field: "rating", Value: "-1"}}},
		{"rating:>=4", []Clause{{Raw: "rating:>=4", Field: "rating", Op: ">=", Value: "4"}}},
		{"marks:<2", []Clause{{Raw: "marks:<2", Field: "marks", Op: "<", Value: "2"}}},
		{"marks:>-2", []Clause{{Raw: "marks:>-2", Field: "marks", Op: ">", Value: "-2"}}},
		{`artist_kw:"alice b"`, []Clause{{Raw: `artist_kw:"alice b"`, Field: "artist_kw", Value: "alice b", Phrase: true}}},
		{`"red fox"`, []Clause{{Raw: `"red fox"`, Value: "red fox", Phrase: true}}},
		{`"my field":x`, []Clause{{Raw: `"my field":x`, Field: "my field", Value: "x"}}},
		{`date:>"2020-01-01"`, []Clause{{Raw: `date:>"2020-01-01"`, Field: "date", Op: ">", Value: "2020-01-01", Phrase: true}}},
		{"title:/lan.*pe/", []Clause{{Raw: "title:/lan.*pe/", Field: "title", Value: "/lan.*pe/"}}},
		{"tags:land*", []Clause{{Raw: "tags:land*", Field: "tags", Value: "land*"}}},
		{"landscape^2", []Clause{{Raw: "landscape^2", Value: "landscape", Suffix: "^2"}}},
		{"tags:landscape~1", []Clause{{Raw: "tags:landscape~1", Field: "tags", Value: "landscape", Suffix: "~1"}}},
		{"foo-bar", []Clause{{Raw: "foo-bar", Value: "foo-bar"}}},
		{"café", []Clause{{Raw: "café", Value: "café"}}},

		// escapes
		{`alice\ b`, []Clause{{Raw: `alice\ b`, Value: "alice b"}}},
		{`a\:b`, []Clause{{Raw: `a\:b`, Value: "a:b"}}},
		{`\-landscape`, []Clause{{Raw: `\-landscape`, Value: "-landscape"}}},
		{`a\xb`, []Clause{{Raw: `a\xb`, Value: `a\xb`}}}, // non-reserved: backslash kept
		{`"say \"hi\""`, []Clause{{Raw: `"say \"hi\""`, Value: `say "hi"`, Phrase: true}}},
		{`"back\\slash"`, []Clause{{Raw: `"back\\slash"`, Value: `back\slash`, Phrase: true}}},

		// several clauses and odd spacing
		{"\t alice   rating:5\u00a0 -marks:0 \n", []Clause{
			{Raw: "alice", Value: "alice"},
			{Raw: "rating:5\u00a0", Field: "rating", Value: "5\u00a0"}, // only ' ' ends a string
			{Raw: "-marks:0", Prefix: "-", Field: "marks", Value: "0"},
		}},
		{"a\u00a0 b", []Clause{{Raw: "a\u00a0", Value: "a\u00a0"}, {Raw: "b", Value: "b"}}},
		{"\u00a0b", []Clause{{Raw: "b", Value: "b"}}}, // but any space is skipped between clauses
		{`"a"b`, []Clause{
			{Raw: `"a"`, Value: "a", Phrase: true},
			{Raw: "b", Value: "b"},
		}},

		// malformed input never fails
		{`"unterminated phrase`, []Clause{{Raw: `"unterminated phrase`, Value: "unterminated phrase", Phrase: true}}},
		{`tags:"open`, []Clause{{Raw: `tags:"open`, Field: "tags", Value: "open", Phrase: true}}},
		{"rating:", []Clause{{Raw: "rating:", Field: "rating"}}},
		{"alice -", []Clause{{Raw: "alice", Value: "alice"}, {Raw: "-", Prefix: "-"}}},
		{"+ -", []Clause{{Raw: "+ -", Prefix: "+", Value: "-"}}},

		// whitespace only separates tokens, as in Bleve
		{"rating: 5", []Clause{{Raw: "rating: 5", Field: "rating", Value: "5"}}},
		{"rating : 5 x", []Clause{
			{Raw: "rating : 5", Field: "rating", Value: "5"},
			{Raw: "x", Value: "x"},
		}},
		{"+ rating:5", []Clause{{Raw: "+ rating:5", Prefix: "+", Field: "rating", Value: "5"}}},
		{"rating:> = 4", []Clause{{Raw: "rating:> = 4", Field: "rating", Op: ">=", Value: "4"}}},
		{"rating:- 1", []Clause{{Raw: "rating:- 1", Field: "rating", Value: "-1"}}},
		{"landscape ^2", []Clause{{Raw: "landscape ^2", Value: "landscape", Suffix: "^2"}}},
		{"foo - bar", []Clause{{Raw: "foo", Value: "foo"}, {Raw: "- bar", Prefix: "-", Value: "bar"}}},
		{`trailing\`, []Clause{{Raw: `trailing\`, Value: "trailing"}}}, // dropped, like Bleve
		{"^2", []Clause{{Raw: "^2", Suffix: "^2"}}},
	}

	for _, tt := range tests {
		got := ParseQueryString(tt.q)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseQueryString(%q)\n got %+v\nwant %+v", tt.q, got, tt.want)
		}
	}
}

func TestJoinClauses(t *testing.T) {
	q := `  alice  -marks:0   artist_kw:"bob c"  rating:>=4 `
	got := JoinClauses(ParseQueryString(q))
	want := `alice -marks:0 artist_kw:"bob c" rating:>=4`
	if got != want {
		t.Errorf("JoinClauses = %q, want %q", got, want)
	}

	// dropping a clause leaves the others untouched
	var kept []Clause
	for _, c := range ParseQueryString(q) {
		if c.Field != "marks" {
			kept = append(kept, c)
		}
	}
	if got, want := JoinClauses(kept), `alice artist_kw:"bob c" rating:>=4`; got != want {
		t.Errorf("JoinClauses without marks = %q, want %q", got, want)
	}

	if got := JoinClauses(nil); got != "" {
		t.Errorf("JoinClauses(nil) = %q", got)
	}
}

func TestClauseString(t *testing.T) {
	tests := []struct {
		c    Clause
		want string
	}{
		{Clause{Field: "rating", Value: "5"}, "rating:5"},
		{Clause{Prefix: "-", Field: "marks", Op: "<", Value: "2"}, "-marks:<2"},
		{Clause{Field: "rating", Value: "-1"}, "rating:-1"},
		{Clause{Field: "artist_kw", Value: `al "b"`, Phrase: true}, `artist_kw:"al \"b\""`},
		{Clause{Value: "a b:c"}, `a\ b\:c`},
		{Clause{Value: "-x"}, `\-x`},
		{Clause{Field: "tags", Value: ">x"}, `tags:\>x`},
		{Clause{Value: "landscape", Suffix: "^2"}, "landscape^2"},
		{Clause{Field: "2024", Value: "x"}, `"2024":x`},
		{Clause{Field: "tags", Value: "+x"}, `tags:\+x`},
		{Clause{Field: "tags", Value: "-x"}, `tags:\-x`},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.c, got, tt.want)
		}
	}
}

// canonical rebuilds q from the parsed parts of its clauses (not their Raw
// text), so comparing it with q under Bleve checks the parsed meaning.
func canonical(q string) string {
	clauses := ParseQueryString(q)
	parts := make([]string, len(clauses))
	for i, c := range clauses {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

func parseBleve(q string) (query.Query, error) {
	return bleve.NewQueryStringQuery(q).Parse()
}

// TestParseQueryStringMatchesBleve checks that the parsed clauses mean the
// same to Bleve as the original query.
func TestParseQueryStringMatchesBleve(t *testing.T) {
	queries := []string{
		"landscape",
		"alice rating:5 -marks:0",
		"+rating:5 marks:<2",
		"rating:>=4 rating:<=5 marks:>-1",
		`artist_kw:"alice b" -tags:"red fox"`,
		`date:>"2020-01-01"`,
		"title:/lan.*pe/ tags:land*",
		"landscape^2 tags:mountain~1",
		`alice\ b a\:b \-x`,
		`"say \"hi\"" "back\\slash"`,
		`"my field":x`,
		"rating:-1",
		"rating : 5 + marks:> = 1 foo - bar landscape ^2 rating:- 1",
	}
	for _, q := range queries {
		want, err := parseBleve(q)
		if err != nil {
			t.Fatalf("Bleve rejects test query %q: %v", q, err)
		}
		c := canonical(q)
		got, err := parseBleve(c)
		if err != nil {
			t.Errorf("Bleve rejects canonical form %q of %q: %v", c, q, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("canonical form %q of %q parses differently", c, q)
		}
	}
}

func FuzzParseQueryString(f *testing.F) {
	for _, s := range []string{
		"alice rating:5 -marks:0", `artist_kw:"alice b"`, "rating:>=4", `a\ b`,
		`"open`, "tags:land*^2", "x~1", `"a"b`, "+ -", `\`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, q string) {
		clauses := ParseQueryString(q)

		// Raw texts appear in order, separated only by whitespace.
		pos := 0
		for _, c := range clauses {
			pos += skipQuerySpace(q[pos:])
			if c.Raw == "" || !strings.HasPrefix(q[pos:], c.Raw) {
				t.Fatalf("clause %+v not found at %d in %q", c, pos, q)
			}
			pos += len(c.Raw)
		}
		if skipQuerySpace(q[pos:]) != len(q)-pos {
			t.Fatalf("unparsed tail %q in %q", q[pos:], q)
		}

		// Whatever Bleve accepts, the canonical form means the same, except
		// for what Clause can't represent: an escaped "field:\-1" (a string)
		// vs "field:-1" (a number), and an empty quoted field `"":x`.
		if strings.Contains(q, `\-`) || strings.Contains(q, `""`) {
			return
		}
		want, err := parseBleve(q)
		if err != nil {
			return
		}
		c := canonical(q)
		got, err := parseBleve(c)
		if err != nil {
			t.Fatalf("Bleve rejects canonical form %q of %q: %v", c, q, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("canonical form %q of %q parses differently", c, q)
		}
	})
}
