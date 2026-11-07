package web

import (
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
)

type QuickFilter struct {
	Label              string
	Add                string
	Remove             string
	CurrentQueryString string
}

func (srv *Server) quickFilters(currentQs url.Values) []QuickFilter {
	ret := []QuickFilter{}

	for _, filter := range srv.cfg.QuickFilters {
		ret = append(ret, QuickFilter{
			Label:              filter.Label,
			Add:                filter.Add,
			Remove:             filter.Remove,
			CurrentQueryString: currentQs.Encode(),
		})
	}

	return ret
}

func (t QuickFilter) Url() string {
	u := new(url.URL)
	u.Path = "/"
	queryString, _ := url.ParseQuery(t.CurrentQueryString)
	delete(queryString, "page")

	q := ""
	if vals, ok := queryString["q"]; ok && len(vals) > 0 {
		q = vals[0]
	}
	q = strings.TrimSpace(q)

	z, err := regexp.Compile(t.Remove)
	if err != nil {
		slog.Error("invalid quick filter Remove regexp", "label", t.Label, "regexp", t.Remove, "err", err)
	} else {
		q = z.ReplaceAllString(q, "")
	}

	q = strings.TrimSpace(q + " " + t.Add)

	if q != "" {
		queryString.Set("q", q)
	} else {
		queryString.Del("q")
	}

	u.RawQuery = queryString.Encode()
	return u.String()
}

func (t QuickFilter) Badge() template.HTML {
	return template.HTML(fmt.Sprintf(`<a class="badge bg-primary text-decoration-none" href="%s">%s</a>`, html.EscapeString(t.Url()), html.EscapeString(t.Label)))
}
