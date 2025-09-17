package main

import (
	"fmt"
	"html"
	"html/template"
	"log"
	"net/url"
	"regexp"
	"strings"
)

type QuickFilter struct {
	Label         string
	Add           string
	Remove        string
	currentSearch string
}

func ParseQuickFilters(currentSearch string) []QuickFilter {
	ret := []QuickFilter{}

	for _, filter := range config.QuickFilters {
		tmp := filter
		tmp.currentSearch = currentSearch

		tmp.Label = filter.Label

		ret = append(ret, tmp)
	}

	return ret
}

func (t QuickFilter) Url() string {
	u := new(url.URL)
	u.Path = "/"
	q := u.Query()

	term := strings.TrimSpace(t.currentSearch)

	z, err := regexp.Compile(t.Remove)
	if err != nil {
		log.Println(err)
	} else {
		term = z.ReplaceAllString(term, "")
	}

	term = term + " " + t.Add
	term = strings.TrimSpace(term)

	q.Set("q", term)

	u.RawQuery = q.Encode()
	return u.String()
}

func (t QuickFilter) Badge() template.HTML {
	return template.HTML(fmt.Sprintf(`<a class="badge bg-primary text-decoration-none" href="%s">%s</a>`, html.EscapeString(t.Url()), html.EscapeString(t.Label)))
}
