package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func writeAsset(t *testing.T, dir, name, content string, mod time.Time) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestAssetUrl(t *testing.T) {
	dir := t.TempDir()
	mod := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	writeAsset(t, dir, "js/app.js", "one", mod)
	av := newAssetVersions(dir)

	versioned := regexp.MustCompile(`^/static/js/app\.js\?v=[0-9a-f]{12}$`)
	first := av.url("/static/js/app.js")
	if !versioned.MatchString(first) {
		t.Fatalf("url = %q", first)
	}
	if again := av.url("/static/js/app.js"); again != first {
		t.Errorf("unchanged file got a new url: %q vs %q", again, first)
	}

	// same size, new mtime: rehashed, new version
	writeAsset(t, dir, "js/app.js", "two", mod.Add(time.Second))
	changed := av.url("/static/js/app.js")
	if changed == first || !versioned.MatchString(changed) {
		t.Errorf("changed file: url = %q (was %q)", changed, first)
	}

	// content back to "one": same version as before, it's the content hash
	writeAsset(t, dir, "js/app.js", "one", mod.Add(2*time.Second))
	if back := av.url("/static/js/app.js"); back != first {
		t.Errorf("same content, different version: %q vs %q", back, first)
	}

	for _, path := range []string{"/static/missing.js", "/static/js", "/other/app.js", "/static/js/app.js?x=1"} {
		if got := av.url(path); got != path {
			t.Errorf("url(%q) = %q, want unchanged", path, got)
		}
	}

	if got := string(av.attr("src", `/static/a"b.js`)); got != `src="/static/a&#34;b.js"` {
		t.Errorf("attr = %s", got)
	}
}

func TestAssetCacheHeaders(t *testing.T) {
	dir := t.TempDir()
	writeAsset(t, dir, "app.css", "body{}", time.Now())
	av := newAssetVersions(dir)
	h := av.handler()
	current := strings.TrimPrefix(av.url("/static/app.css"), "/static/app.css")

	tests := []struct {
		target string
		status int
		cache  string
	}{
		{"/static/app.css" + current, http.StatusOK, "public, max-age=31536000, immutable"},
		{"/static/app.css", http.StatusOK, "no-cache"},
		{"/static/app.css?v=000000000000", http.StatusOK, "no-cache"}, // stale version
		{"/static/missing.css?v=abc", http.StatusNotFound, ""},        // http.Error drops Cache-Control
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))
		if rec.Code != tt.status || rec.Header().Get("Cache-Control") != tt.cache {
			t.Errorf("%s: %d %q, want %d %q", tt.target, rec.Code, rec.Header().Get("Cache-Control"), tt.status, tt.cache)
		}
	}
}
