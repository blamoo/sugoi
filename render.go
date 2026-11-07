package main

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"sync"
	"time"
)

var templateStore *template.Template
var templateMu sync.Mutex

// templateFuncs must not depend on the request: in non-debug mode templates
// are parsed once and shared by every request.
var templateFuncs = template.FuncMap{
	"asset": func(attrn string, filePath string) template.HTMLAttr {
		return template.HTMLAttr(fmt.Sprintf(`%s="%s"`, attrn, html.EscapeString(filePath)))
	},
	"noescape": func(str string) template.HTML {
		return template.HTML(str)
	},
	"getBuildTime": func() template.HTML {
		return template.HTML(buildStr)
	},
	"getBuildDiff": func() template.HTML {
		return template.HTML(BuildDiff())
	},
	"formatTime": func(t time.Time) template.HTML {
		b, _ := t.MarshalText()
		return template.HTML(b)
	},
}

func parseTemplates() (*template.Template, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseGlob("templates/*.gohtml")
	if err != nil {
		return nil, err
	}

	tmpl, err = tmpl.ParseGlob("pages/*.gohtml")
	if err != nil {
		return nil, err
	}

	debugPrintf("%s\n", tmpl.DefinedTemplates())
	return tmpl, nil
}

// getTemplates re-parses on every call in debug mode (edit templates without
// restarting) and parses once otherwise. Either way the result is kept in
// templateStore for currentTemplates.
func getTemplates() (*template.Template, error) {
	if config.Debug {
		tmpl, err := parseTemplates()
		if err != nil {
			return nil, err
		}

		templateMu.Lock()
		templateStore = tmpl
		templateMu.Unlock()
		return tmpl, nil
	}

	templateMu.Lock()
	defer templateMu.Unlock()

	if templateStore == nil {
		tmpl, err := parseTemplates()
		if err != nil {
			return nil, err
		}
		templateStore = tmpl
	}

	return templateStore, nil
}

// currentTemplates returns the last parsed templates without re-parsing, for
// partials rendered from inside a page (Thing.HtmlTable).
func currentTemplates() (*template.Template, error) {
	templateMu.Lock()
	tmpl := templateStore
	templateMu.Unlock()

	if tmpl != nil {
		return tmpl, nil
	}
	return getTemplates()
}

func RenderPage(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, err := getTemplates()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	err = tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
}

func RenderError(w http.ResponseWriter, r *http.Request, message string) {
	data := struct {
		Title string
		Error string
	}{}

	data.Error = message
	data.Title = "Error"
	debugPrintf("%s\n", message)
	w.Header().Set("Content-Type", "text/html")
	// Some callers set long-lived cache headers before failing; never let an
	// error page be cached in place of the real resource.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Del("Expires")
	w.WriteHeader(400)
	RenderPage(w, r, "error.gohtml", data)
}

func SetCacheHeader(w http.ResponseWriter, r *http.Request, age uint) {
	h := w.Header()

	h.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", age))

	ageD, _ := time.ParseDuration(fmt.Sprintf("%ds", age))
	cacheUntil := time.Now().Add(ageD)
	h.Set("Expires", cacheUntil.Format(http.TimeFormat))
}

type JsonResponse struct {
	Message string
}

type JsonError struct {
	Error string
}
