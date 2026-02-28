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
	Label              string
	Add                string
	Remove             string
	CurrentQueryString string
}

func ParseQuickFilters(currentQs url.Values) []QuickFilter {
	ret := []QuickFilter{}

	for _, filter := range config.QuickFilters {
		tmp := filter
		tmp.CurrentQueryString = currentQs.Encode()

		tmp.Label = filter.Label

		ret = append(ret, tmp)
	}

	return ret
}

func (t QuickFilter) Url() string {
	u := new(url.URL)
	u.Path = "/"
	queryString, _ := url.ParseQuery(t.CurrentQueryString)
	delete(queryString, "page")

	if q, ok := queryString["q"]; ok {
		q[0] = strings.TrimSpace(q[0])

		z, err := regexp.Compile(t.Remove)
		if err != nil {
			log.Println(err)
		} else {
			q[0] = z.ReplaceAllString(q[0], "")
		}

		q[0] = q[0] + " " + t.Add
		q[0] = strings.TrimSpace(q[0])
	}

	u.RawQuery = queryString.Encode()
	return u.String()
}

func (t QuickFilter) Badge() template.HTML {
	return template.HTML(fmt.Sprintf(`<a class="badge bg-primary text-decoration-none" href="%s">%s</a>`, html.EscapeString(t.Url()), html.EscapeString(t.Label)))
}
