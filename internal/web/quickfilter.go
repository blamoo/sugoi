package web

import (
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/search"
)

// quickFilterRule is a config.QuickFilter prepared once at startup.
type quickFilterRule struct {
	label   string
	add     string
	clauses []search.Clause // Add's clauses
	fields  []string        // fields of Add's clauses, replaced in the search
	remove  *regexp.Regexp  // optional extra removal
}

func newQuickFilterRules(filters []config.QuickFilter) []quickFilterRule {
	rules := make([]quickFilterRule, 0, len(filters))
	for _, f := range filters {
		rule := quickFilterRule{label: f.Label, add: strings.TrimSpace(f.Add)}
		if rule.label == "" {
			rule.label = rule.add
		}
		rule.clauses = search.ParseQueryString(rule.add)
		for _, c := range rule.clauses {
			if c.Field != "" && !slices.Contains(rule.fields, c.Field) {
				rule.fields = append(rule.fields, c.Field)
			}
		}
		if f.Remove != "" {
			re, err := regexp.Compile(f.Remove)
			if err != nil { // config.Load rejects this; only reachable with a hand-built config
				slog.Error("invalid quick filter Remove regexp", "label", rule.label, "regexp", f.Remove, "err", err)
			} else {
				rule.remove = re
			}
		}
		rules = append(rules, rule)
	}
	return rules
}

// apply returns q with the rule's fields (and Remove matches) taken out and
// its Add clauses appended. Malformed clauses (a dangling "-" or "field:",
// or a stray ":x" or "^2" that would join the clause before it once its
// neighbor is gone; all invalid for Bleve anyway) are dropped, and the last
// clause is rewritten, or dropped as a last resort, if it would swallow the
// added ones (an unterminated phrase, a trailing backslash).
func (rule quickFilterRule) apply(q string) string {
	clauses := search.ParseQueryString(q)
	kept := clauses[:0]
	for _, c := range clauses {
		if !slices.Contains(rule.fields, c.Field) {
			kept = append(kept, c)
		}
	}

	if rule.remove != nil {
		q = rule.remove.ReplaceAllString(search.JoinClauses(kept), "")
		kept = search.ParseQueryString(q)
	}

	clauses = kept[:0]
	for _, c := range kept {
		if (c.Value != "" || c.Phrase) && strings.IndexByte(":^~", c.Raw[0]) < 0 {
			clauses = append(clauses, c)
		}
	}

	join := func() string { return strings.TrimSpace(search.JoinClauses(clauses) + " " + rule.add) }
	ret := join()
	if n := len(clauses); n > 0 && !rule.endsWithAdd(ret) {
		clauses[n-1].Raw = clauses[n-1].String()
		if ret = join(); !rule.endsWithAdd(ret) {
			clauses = clauses[:n-1]
			ret = join()
		}
	}
	return ret
}

// endsWithAdd reports whether q parses with the rule's Add clauses intact at
// its end.
func (rule quickFilterRule) endsWithAdd(q string) bool {
	clauses := search.ParseQueryString(q)
	if len(clauses) < len(rule.clauses) {
		return false
	}
	for i, c := range clauses[len(clauses)-len(rule.clauses):] {
		if c.Raw != rule.clauses[i].Raw {
			return false
		}
	}
	return true
}

// sameClause reports whether two clauses mean the same, however written.
// Unprefixed clauses are required (see search.ShouldToMust), so "" and "+"
// are the same prefix.
func sameClause(a, b search.Clause) bool {
	required := func(c search.Clause) bool { return c.Prefix != "-" }
	return required(a) == required(b) && a.Field == b.Field && a.Op == b.Op &&
		a.Value == b.Value && a.Phrase == b.Phrase && a.Suffix == b.Suffix
}

// isActive reports whether every Add clause is already in q.
func (rule quickFilterRule) isActive(q string) bool {
	if len(rule.clauses) == 0 {
		return false
	}
	clauses := search.ParseQueryString(q)
	for _, want := range rule.clauses {
		if !slices.ContainsFunc(clauses, func(c search.Clause) bool { return sameClause(c, want) }) {
			return false
		}
	}
	return true
}

// unapply returns q without the rule's Add clauses, leaving any other clause
// on the same fields.
func (rule quickFilterRule) unapply(q string) string {
	clauses := search.ParseQueryString(q)
	kept := clauses[:0]
	for _, c := range clauses {
		if !slices.ContainsFunc(rule.clauses, func(want search.Clause) bool { return sameClause(c, want) }) {
			kept = append(kept, c)
		}
	}
	return search.JoinClauses(kept)
}

// QuickFilter is a quick filter badge ready to render. Active filters are
// already in the search, and their link removes them.
type QuickFilter struct {
	Label  string
	Href   string
	Active bool
}

// quickFilters builds the badges for the home page, each linking to the
// current search (form) with that filter applied, or removed if active.
func (srv *Server) quickFilters(form url.Values) []QuickFilter {
	params := url.Values{}
	for k, v := range form {
		switch k {
		case "page", "random", "refresh", "q":
		default:
			params[k] = v
		}
	}
	q := form.Get("q")

	ret := make([]QuickFilter, 0, len(srv.quickFilterRules))
	for _, rule := range srv.quickFilterRules {
		active := rule.isActive(q)
		newQ := rule.apply(q)
		if active {
			newQ = rule.unapply(q)
		}
		if newQ != "" {
			params.Set("q", newQ)
		} else {
			params.Del("q")
		}
		u := url.URL{Path: "/", RawQuery: params.Encode()}
		ret = append(ret, QuickFilter{Label: rule.label, Href: u.String(), Active: active})
	}
	return ret
}

func (t QuickFilter) Badge() template.HTML {
	if t.Active {
		return template.HTML(fmt.Sprintf(`<a class="badge bg-success text-decoration-none" href="%s" title="Remove this filter">%s <i class="fa-solid fa-xmark"></i></a>`, html.EscapeString(t.Href), html.EscapeString(t.Label)))
	}
	return template.HTML(fmt.Sprintf(`<a class="badge bg-primary text-decoration-none" href="%s">%s</a>`, html.EscapeString(t.Href), html.EscapeString(t.Label)))
}
