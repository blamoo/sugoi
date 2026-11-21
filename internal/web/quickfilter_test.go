package web

import (
	"net/url"
	"testing"

	"github.com/blevesearch/bleve/v2"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/search"
)

func TestQuickFilterApply(t *testing.T) {
	rules := newQuickFilterRules([]config.QuickFilter{
		{Add: "rating:5"},
		{Add: "-marks:0"},
		{Add: "marks:<2", Label: "few marks"},
		{Add: "landscape"},
		{Add: "tags:sea", Remove: `\bmountain\b`},
	})
	r5, notMarked, fewMarks, free, sea := rules[0], rules[1], rules[2], rules[3], rules[4]

	tests := []struct {
		name string
		rule quickFilterRule
		q    string
		want string
	}{
		{"empty search", r5, "", "rating:5"},
		{"appends", r5, "alice", "alice rating:5"},
		{"replaces same field", r5, "alice rating:0", "alice rating:5"},
		{"replaces any form", r5, `+rating:>=3 alice -rating:1 rating : 2 bob`, "alice bob rating:5"},
		{"keeps other fields", r5, `artist_kw:"alice b" marks:0`, `artist_kw:"alice b" marks:0 rating:5`},
		{"negated add", notMarked, "marks:<2 alice", "alice -marks:0"},
		{"operator add", fewMarks, "-marks:0", "marks:<2"},
		{"no field removes nothing", free, "landscape rating:5", "landscape rating:5 landscape"},
		{"extra Remove regexp", sea, "mountain tags:lake alice", "alice tags:sea"},
		{"unterminated phrase is closed", r5, `alice "open rating:1`, `alice "open rating:1" rating:5`},
		{"trailing backslash is dropped", r5, `alice\`, `alice rating:5`},
		{"both", r5, `alice "open\`, `alice "open" rating:5`},
		{"dangling prefix", r5, "alice -", "alice rating:5"},
		{"dangling field", r5, "alice tags:", "alice rating:5"},
		{"dangling field in the middle", r5, "tags: alice", "tags: alice rating:5"},
		{"stray colon", r5, "alice 0::0", "alice rating:5"},
	}
	for _, tt := range tests {
		if got := tt.rule.apply(tt.q); got != tt.want {
			t.Errorf("%s: apply(%q) = %q, want %q", tt.name, tt.q, got, tt.want)
		}
	}

	if r5.label != "rating:5" || fewMarks.label != "few marks" {
		t.Errorf("labels = %q, %q", r5.label, fewMarks.label)
	}
}

func TestQuickFiltersUrl(t *testing.T) {
	srv := &Server{quickFilterRules: newQuickFilterRules([]config.QuickFilter{{Add: "rating:5"}})}
	form := url.Values{
		"q":       {"alice rating:1"},
		"order":   {"title"},
		"page":    {"3"},
		"random":  {"1"},
		"refresh": {"1"},
	}

	got := srv.quickFilters(form)
	if len(got) != 1 {
		t.Fatalf("got %d filters", len(got))
	}
	u, err := url.Parse(got[0].Href)
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{"q": {"alice rating:5"}, "order": {"title"}}
	if u.Path != "/" || u.Query().Encode() != want.Encode() {
		t.Errorf("Href = %q, want /?%s", got[0].Href, want.Encode())
	}
	if form.Get("q") != "alice rating:1" {
		t.Error("form was modified")
	}
}

func TestQuickFilterToggle(t *testing.T) {
	rules := newQuickFilterRules([]config.QuickFilter{
		{Add: "rating:5"},
		{Add: "-marks:0"},
		{Add: "marks:<2"},
		{Add: `tags:"red fox" landscape`},
	})
	r5, notMarked, fewMarks, multi := rules[0], rules[1], rules[2], rules[3]

	tests := []struct {
		name   string
		rule   quickFilterRule
		q      string
		active bool
		off    string // unapply result, when active
	}{
		{"empty search", r5, "", false, ""},
		{"exact", r5, "alice rating:5", true, "alice"},
		{"plus prefix is the same", r5, "+rating:5 alice", true, "alice"},
		{"spaced", r5, "rating : 5", true, ""},
		{"other value", r5, "rating:4", false, ""},
		{"other operator", r5, "rating:>=5", false, ""},
		{"negated is different", r5, "-rating:5", false, ""},
		{"negated add", notMarked, "alice -marks:0", true, "alice"},
		{"negated add, positive search", notMarked, "marks:0", false, ""},
		{"operator", fewMarks, "marks:<2", true, ""},
		{"keeps other clauses on the field", r5, "rating:5 rating:>3", true, "rating:>3"},
		{"all clauses needed", multi, `tags:"red fox"`, false, ""},
		{"all clauses present", multi, `landscape alice tags:"red fox"`, true, "alice"},
		{"phrase is not a word", r5, `rating:"5"`, false, ""},
	}
	for _, tt := range tests {
		if got := tt.rule.isActive(tt.q); got != tt.active {
			t.Errorf("%s: isActive(%q) = %v, want %v", tt.name, tt.q, got, tt.active)
			continue
		}
		if tt.active {
			if got := tt.rule.unapply(tt.q); got != tt.off {
				t.Errorf("%s: unapply(%q) = %q, want %q", tt.name, tt.q, got, tt.off)
			}
		}
	}

	if (quickFilterRule{}).isActive("anything") {
		t.Error("a rule without clauses is active")
	}
}

func TestQuickFiltersActiveUrl(t *testing.T) {
	srv := &Server{quickFilterRules: newQuickFilterRules([]config.QuickFilter{{Add: "rating:5"}, {Add: "marks:0"}})}

	got := srv.quickFilters(url.Values{"q": {"rating:5"}, "order": {"title"}})
	if !got[0].Active || got[0].Href != "/?order=title" {
		t.Errorf("active filter = %+v, want a link without q", got[0])
	}
	if got[1].Active || got[1].Href != "/?order=title&q=rating%3A5+marks%3A0" {
		t.Errorf("inactive filter = %+v", got[1])
	}
}

func TestQuickFilterRedirect(t *testing.T) {
	srv := &Server{quickFilterRules: newQuickFilterRules([]config.QuickFilter{{Add: "rating:5"}, {Add: "marks:0"}})}
	form := func(qf, q string) url.Values {
		return url.Values{"qf": {qf}, "q": {q}, "order": {"title"}, "page": {"0"}, "format": {"table"}}
	}

	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{"no qf", url.Values{"q": {"alice"}}, ""},
		{"applies to the typed search", form("0", "alice landscape rating:1"), "/?format=table&order=title&q=alice+landscape+rating%3A5"},
		{"second filter", form("1", "alice"), "/?format=table&order=title&q=alice+marks%3A0"},
		{"toggles off", form("0", "alice rating:5"), "/?format=table&order=title&q=alice"},
		{"empty search", form("0", ""), "/?format=table&order=title&q=rating%3A5"},
		{"toggled off to empty", form("0", "rating:5"), "/?format=table&order=title"},
		{"bad index is dropped", form("9", "alice"), "/?format=table&order=title&q=alice"},
		{"negative index", form("-1", "alice"), "/?format=table&order=title&q=alice"},
		{"not a number", form("x", "alice"), "/?format=table&order=title&q=alice"},
	}
	for _, tt := range tests {
		if got := srv.quickFilterRedirect(tt.form); got != tt.want {
			t.Errorf("%s: redirect = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestQuickFilterBadgeEscapes(t *testing.T) {
	got := string(QuickFilter{Label: `<b>&`, Href: `/?a=1&q="x"`}.Badge())
	want := `<a class="badge bg-primary text-decoration-none" href="/?a=1&amp;q=&#34;x&#34;" data-quick-filter="0">&lt;b&gt;&amp;</a>`
	if got != want {
		t.Errorf("Badge = %s\nwant    %s", got, want)
	}

	got = string(QuickFilter{Label: `<b>`, Href: `/?a=1&b=2`, Active: true, Index: 3}.Badge())
	want = `<a class="badge bg-success text-decoration-none" href="/?a=1&amp;b=2" data-quick-filter="3" title="Remove this filter">&lt;b&gt; <i class="fa-solid fa-xmark"></i></a>`
	if got != want {
		t.Errorf("active Badge = %s\nwant           %s", got, want)
	}
}

func FuzzQuickFilterApply(f *testing.F) {
	for _, s := range []string{
		"alice rating:1", `+rating:>=3 -rating:1 rating : 2 bob`, `artist_kw:"alice b"`,
		`"open rating:1`, "rating:- 1", `a\:b rating:5^2`,
	} {
		f.Add(s)
	}
	rule := newQuickFilterRules([]config.QuickFilter{{Add: "-rating:5"}})[0]
	f.Fuzz(func(t *testing.T, q string) {
		got := rule.apply(q)

		// only the added clause is left on the filter's field
		var onField []string
		for _, c := range search.ParseQueryString(got) {
			if c.Field == "rating" {
				onField = append(onField, c.Raw)
			}
		}
		if len(onField) != 1 || onField[0] != "-rating:5" {
			t.Fatalf("apply(%q) = %q, rating clauses %q", q, got, onField)
		}

		// a valid search stays valid
		if _, err := bleve.NewQueryStringQuery(q).Parse(); err != nil {
			return
		}
		if _, err := bleve.NewQueryStringQuery(got).Parse(); err != nil {
			t.Fatalf("apply(%q) = %q, rejected by Bleve: %v", q, got, err)
		}
	})
}
