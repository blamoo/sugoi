package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// assetVersions maps files under static/ to a short hash of their content,
// used as ?v= in their URLs so browsers can cache them forever and still get
// a new copy as soon as a file changes. Hashes are recomputed when a file's
// size or modification time changes, so this also works in Debug mode while
// editing.
type assetVersions struct {
	dir string // the static directory

	mu      sync.Mutex
	entries map[string]assetVersion
}

type assetVersion struct {
	size    int64
	modTime time.Time
	version string
}

func newAssetVersions(dir string) *assetVersions {
	return &assetVersions{dir: dir, entries: make(map[string]assetVersion)}
}

// version returns the content hash of the file at rel (relative to the static
// directory), or "" if it can't be read.
func (av *assetVersions) version(rel string) string {
	name := filepath.Join(av.dir, filepath.FromSlash(rel))
	info, err := os.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}

	av.mu.Lock()
	e, ok := av.entries[rel]
	av.mu.Unlock()
	if ok && e.size == info.Size() && e.modTime.Equal(info.ModTime()) {
		return e.version
	}

	f, err := os.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		slog.Warn("hashing asset", "file", name, "err", err)
		return ""
	}
	e = assetVersion{size: info.Size(), modTime: info.ModTime(), version: hex.EncodeToString(h.Sum(nil))[:12]}

	av.mu.Lock()
	av.entries[rel] = e
	av.mu.Unlock()
	return e.version
}

// url adds ?v=<version> to a /static/ path. Other paths, and files that
// can't be read, are returned unchanged.
func (av *assetVersions) url(path string) string {
	rel, ok := strings.CutPrefix(path, "/static/")
	if !ok || strings.Contains(path, "?") {
		return path
	}
	if v := av.version(rel); v != "" {
		return path + "?v=" + v
	}
	return path
}

// attr renders an HTML attribute with the versioned URL, for the "asset"
// template func: {{ asset "src" "/static/main.js" }}.
func (av *assetVersions) attr(name, path string) template.HTMLAttr {
	return template.HTMLAttr(fmt.Sprintf(`%s="%s"`, name, html.EscapeString(av.url(path))))
}

// handler serves the static directory. A request with the file's current
// ?v= may be cached forever; anything else (no version, an old one, files
// referenced from CSS) must be revalidated, which costs a 304 when unchanged.
func (av *assetVersions) handler() http.Handler {
	files := http.StripPrefix("/static/", http.FileServer(http.Dir(av.dir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/static/")
		if v := r.URL.Query().Get("v"); v != "" && v == av.version(rel) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
