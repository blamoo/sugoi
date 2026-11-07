package web

import (
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"
)

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

func (srv *Server) parseTemplates() (*template.Template, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseGlob(filepath.Join(srv.assetsDir, "templates", "*.gohtml"))
	if err != nil {
		return nil, err
	}

	tmpl, err = tmpl.ParseGlob(filepath.Join(srv.assetsDir, "pages", "*.gohtml"))
	if err != nil {
		return nil, err
	}

	slog.Debug("templates parsed", "templates", tmpl.DefinedTemplates())
	return tmpl, nil
}

// getTemplates re-parses on every call in debug mode (edit templates without
// restarting) and parses once otherwise. Either way the result is kept in
// srv.tmplStore for currentTemplates.
func (srv *Server) getTemplates() (*template.Template, error) {
	if srv.cfg.Debug {
		tmpl, err := srv.parseTemplates()
		if err != nil {
			return nil, err
		}

		srv.tmplMu.Lock()
		srv.tmplStore = tmpl
		srv.tmplMu.Unlock()
		return tmpl, nil
	}

	srv.tmplMu.Lock()
	defer srv.tmplMu.Unlock()

	if srv.tmplStore == nil {
		tmpl, err := srv.parseTemplates()
		if err != nil {
			return nil, err
		}
		srv.tmplStore = tmpl
	}

	return srv.tmplStore, nil
}

// currentTemplates returns the last parsed templates without re-parsing, for
// partials rendered from inside a page (Thing.HtmlTable).
func (srv *Server) currentTemplates() (*template.Template, error) {
	srv.tmplMu.Lock()
	tmpl := srv.tmplStore
	srv.tmplMu.Unlock()

	if tmpl != nil {
		return tmpl, nil
	}
	return srv.getTemplates()
}

func (srv *Server) renderPage(w http.ResponseWriter, r *http.Request, name string, data any) {
	tmpl, err := srv.getTemplates()
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

func (srv *Server) renderError(w http.ResponseWriter, r *http.Request, message string) {
	data := struct {
		Title string
		Error string
	}{}

	data.Error = message
	data.Title = "Error"
	slog.Debug("error page", "path", r.URL.Path, "message", message)
	w.Header().Set("Content-Type", "text/html")
	// Some callers set long-lived cache headers before failing; never let an
	// error page be cached in place of the real resource.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Del("Expires")
	w.WriteHeader(400)
	srv.renderPage(w, r, "error.gohtml", data)
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
