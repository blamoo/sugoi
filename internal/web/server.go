// Package web is the HTTP side of sugoi: routes, authentication, page
// rendering, thumbnails and the view helpers used by the templates.
package web

import (
	"html/template"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/blamoo/sugoi/internal/catalog"
	"github.com/blamoo/sugoi/internal/config"
)

// randomSampleSize is how many hits a random search draws one result from.
const randomSampleSize = 1000

type Server struct {
	cfg       *config.Config
	cat       *catalog.Catalog
	assetsDir string

	sessions *sessions.CookieStore

	// basicAuthOk remembers credentials that already passed bcrypt (see
	// checkPasswordHashCached).
	basicAuthOk sync.Map

	tmplMu    sync.Mutex
	tmplStore *template.Template
}

// NewServer creates the web server. assetsDir holds templates/, pages/ and
// static/ (the repository root, or wherever they were deployed).
func NewServer(cfg *config.Config, cat *catalog.Catalog, assetsDir string) *Server {
	srv := &Server{cfg: cfg, cat: cat, assetsDir: assetsDir}

	srv.sessions = sessions.NewCookieStore(cfg.SessionCookieKey)
	srv.sessions.Options.Secure = false
	srv.sessions.Options.SameSite = http.SameSiteStrictMode
	srv.sessions.MaxAge(cfg.SessionCookieMaxAge)

	return srv
}

// Handler returns the router with every route, the static files and a limit
// of MaxConcurrency requests handled at once.
func (srv *Server) Handler() http.Handler {
	router := mux.NewRouter()

	semaphore := make(chan struct{}, max(srv.cfg.MaxConcurrency, 1))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			next.ServeHTTP(w, r)
		})
	})

	srv.routes(router)

	static := http.FileServer(http.Dir(filepath.Join(srv.assetsDir, "static")))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", static))

	return router
}
