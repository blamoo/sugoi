package web

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/blamoo/sugoi/internal/catalog"
	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"
	"github.com/blamoo/sugoi/internal/search"
)

const (
	testUser     = "user"
	testPassword = "secret123"
	bookKey      = "{{LIB}}/Some Collection/[bar] A Book.cbz"
)

type testServer struct {
	t       *testing.T
	cfg     *config.Config
	cat     *catalog.Catalog
	handler http.Handler
	hash    string
}

// writeBook creates a .cbz with two PNG pages, 300px wide.
func writeBook(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for i, c := range []color.RGBA{{200, 30, 30, 255}, {30, 30, 200, 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 300, 400))
		for x := 0; x < 300; x++ {
			for y := 0; y < 400; y++ {
				img.Set(x, y, c)
			}
		}
		w, err := zw.Create(fmt.Sprintf("%d.png", i+1))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(w, img); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T, debug bool) *testServer {
	t.Helper()

	dir := t.TempDir()
	writeBook(t, filepath.Join(dir, "library", "Some Collection", "[bar] A Book.cbz"))

	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Debug:               debug,
		CacheThumbnails:     true,
		CacheDir:            t.TempDir(),
		DatabaseDir:         dir,
		MaxConcurrency:      2,
		DirVars:             map[string]string{"LIB": filepath.Join(dir, "library")},
		SessionCookieName:   "sugoi",
		SessionCookieMaxAge: 60,
		SessionCookieKey:    []byte("0123456789abcdef0123456789abcdef"),
		Users:               map[string]string{testUser: string(hash)},
		QuickFilters:        []config.QuickFilter{{Label: "rating:5", Add: "+rating:5", Remove: `\s*[+-]?rating:\d+`}},
		SlideshowPresets:    []config.SlideshowPreset{{Seconds: 5, Default: true}},
	}

	lib := library.New(cfg, []string{bookKey})
	index, err := search.OpenMem()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { index.Close() })

	cat := catalog.New(cfg, lib, index)
	file := lib.List[0]

	static := metadata.Static{Artist: metadata.StringArray{"bar"}, Tags: metadata.StringArray{"Glasses"}, Title: "A Book"}
	if err := metadata.Write(file.StaticMetaPath(), static); err != nil {
		t.Fatal(err)
	}
	if err := cat.Reindex.Start(); err != nil {
		t.Fatal(err)
	}
	if !cat.Reindex.Wait(10 * time.Second) {
		t.Fatal("reindex didn't finish")
	}

	// the templates and static files live at the repository root
	srv := NewServer(cfg, cat, "../..")
	return &testServer{t: t, cfg: cfg, cat: cat, handler: srv.Handler(), hash: file.Hash}
}

// do sends a request; auth adds HTTP Basic credentials.
func (ts *testServer) do(method, target string, form url.Values, auth bool) *httptest.ResponseRecorder {
	ts.t.Helper()

	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if auth {
		req.SetBasicAuth(testUser, testPassword)
	}

	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func (ts *testServer) get(target string) *httptest.ResponseRecorder {
	return ts.do(http.MethodGet, target, nil, true)
}

func (ts *testServer) post(target string, form url.Values) *httptest.ResponseRecorder {
	if form == nil {
		form = url.Values{}
	}
	return ts.do(http.MethodPost, target, form, true)
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body: %.300s", rec.Code, want, rec.Body.String())
	}
}

func expectBody(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("body doesn't contain %q: %.500s", want, rec.Body.String())
	}
}

// bothModes runs a test with templates parsed once (Debug off) and parsed per
// request (Debug on): the details page once panicked only in Debug mode.
func bothModes(t *testing.T, fn func(t *testing.T, ts *testServer)) {
	for _, debug := range []bool{false, true} {
		t.Run(fmt.Sprintf("debug=%v", debug), func(t *testing.T) {
			fn(t, newTestServer(t, debug))
		})
	}
}

func TestAuthentication(t *testing.T) {
	bothModes(t, func(t *testing.T, ts *testServer) {
		rec := ts.do(http.MethodGet, "/?q=x", nil, false)
		expectStatus(t, rec, http.StatusFound)
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login?return=") {
			t.Errorf("redirect to %q", loc)
		}

		rec = ts.do(http.MethodPost, "/thing/"+ts.hash+"/rating.json", url.Values{"rate": {"3"}}, false)
		expectStatus(t, rec, http.StatusForbidden)

		rec = ts.do(http.MethodGet, "/autocomplete.json?q=ba", nil, false)
		expectStatus(t, rec, http.StatusForbidden)

		// login page keeps the typed user
		rec = ts.do(http.MethodPost, "/login", url.Values{"user": {"user"}, "password": {"wrong"}}, false)
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, "Wrong or empty password")
		expectBody(t, rec, `value="user"`)

		// a successful login sets a session cookie that works without Basic
		rec = ts.do(http.MethodPost, "/login", url.Values{"user": {"USER"}, "password": {testPassword}, "return": {"/system"}}, false)
		expectStatus(t, rec, http.StatusFound)
		if loc := rec.Header().Get("Location"); loc != "/system" {
			t.Errorf("redirect to %q, want /system", loc)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("no session cookie")
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec = httptest.NewRecorder()
		ts.handler.ServeHTTP(rec, req)
		expectStatus(t, rec, http.StatusOK)

		// an open redirect is refused
		rec = ts.do(http.MethodPost, "/login", url.Values{"user": {testUser}, "password": {testPassword}, "return": {"https://example.com"}}, false)
		if loc := rec.Header().Get("Location"); loc != "/" {
			t.Errorf("external return redirected to %q", loc)
		}
	})
}

func TestPages(t *testing.T) {
	bothModes(t, func(t *testing.T, ts *testServer) {
		rec := ts.get("/")
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, "A Book")
		expectBody(t, rec, `data-autocomplete`)

		rec = ts.get("/?q=" + url.QueryEscape(`artist_kw:"bar" tags:glasses`))
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, "1 result")

		rec = ts.get("/?q=" + url.QueryEscape(`artist_kw:"bar" tags:nothing`))
		expectBody(t, rec, "No results")

		rec = ts.get("/?q=" + url.QueryEscape(`artist:^bar$`)) // syntax error
		expectStatus(t, rec, http.StatusBadRequest)

		for _, format := range []string{"covers", "table", "full"} {
			expectStatus(t, ts.get("/?format="+format), http.StatusOK)
		}

		rec = ts.get("/thing/details/" + ts.hash)
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, "Read count") // rendered by Thing.HtmlTable
		expectBody(t, rec, `artist_kw%3A%22bar%22`)

		rec = ts.get("/thing/editMetadata/" + ts.hash + "?format=edit")
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, `name="created_at"`)

		rec = ts.get("/thing/read/" + ts.hash + "/1")
		expectStatus(t, rec, http.StatusOK)
		expectBody(t, rec, "window.sugoi.reader.load(")

		rec = ts.get("/system")
		expectStatus(t, rec, http.StatusOK)

		expectStatus(t, ts.get("/thing/details/0000"), http.StatusBadRequest)
		expectStatus(t, ts.get("/static/main.css"), http.StatusOK)
		expectStatus(t, ts.get("/favicon.ico"), http.StatusOK)

		rec = ts.get("/?random=1")
		expectStatus(t, rec, http.StatusFound)
		if loc := rec.Header().Get("Location"); loc != "/thing/read/"+ts.hash {
			t.Errorf("random redirected to %q", loc)
		}
	})
}

func TestJSONEndpoints(t *testing.T) {
	ts := newTestServer(t, false)

	var details struct {
		File  struct{ Hash string }
		Title string `json:"title"`
		Pages int    `json:"pages"`
	}
	decodeJSON(t, ts.get("/thing/details/"+ts.hash+".json"), &details)
	if details.File.Hash != ts.hash || details.Title != "A Book" || details.Pages != 2 {
		t.Errorf("details = %+v", details)
	}

	var reader ReaderData
	decodeJSON(t, ts.get("/thing/read/"+ts.hash+"/1?json=1"), &reader)
	prefix := "/thing/file/" + ts.hash + "/"
	if len(reader.Pages) != 2 || reader.Pages[0] != prefix+"1.png" || reader.Page != 1 {
		t.Errorf("reader = %+v", reader)
	}

	var suggestions []search.AutocompleteEntry
	decodeJSON(t, ts.get("/autocomplete.json?q=ba"), &suggestions)
	if len(suggestions) == 0 || suggestions[0].Insert != `artist_kw:"bar"` {
		t.Errorf("suggestions = %+v", suggestions)
	}

	var files []struct{ Hash string }
	decodeJSON(t, ts.get("/files.json"), &files)
	if len(files) != 1 || files[0].Hash != ts.hash {
		t.Errorf("files = %+v", files)
	}
}

func TestWrites(t *testing.T) {
	ts := newTestServer(t, false)
	thing := "/thing/" + ts.hash

	var rating struct{ Rating int }
	decodeJSON(t, ts.post(thing+"/rating.json", url.Values{"rate": {"4"}}), &rating)
	if rating.Rating != 4 {
		t.Errorf("rating = %d", rating.Rating)
	}
	decodeJSON(t, ts.post(thing+"/rating.json", url.Values{"rate": {"4"}, "toggle": {"true"}}), &rating)
	if rating.Rating != 0 {
		t.Errorf("toggled rating = %d, want 0", rating.Rating)
	}
	expectStatus(t, ts.post(thing+"/rating.json", url.Values{"rate": {"9"}}), http.StatusBadRequest)
	expectStatus(t, ts.post(thing+"/rating.json", url.Values{"rate": {"x"}}), http.StatusBadRequest)

	var marks struct{ Marks int }
	decodeJSON(t, ts.post(thing+"/addMark.json", nil), &marks)
	decodeJSON(t, ts.post(thing+"/addMark.json", nil), &marks)
	decodeJSON(t, ts.post(thing+"/subMark.json", nil), &marks)
	if marks.Marks != 1 {
		t.Errorf("marks = %d, want 1", marks.Marks)
	}

	var count int
	decodeJSON(t, ts.post("/thing/pushRead/"+ts.hash, nil), &count)
	if count != 1 {
		t.Errorf("read count = %d", count)
	}

	var cover struct{ Cover string }
	page2 := "/thing/file/" + ts.hash + "/2.png"
	decodeJSON(t, ts.post(thing+"/cover.json", url.Values{"file": {page2}}), &cover)
	if cover.Cover != "2.png" {
		t.Errorf("cover = %q", cover.Cover)
	}
	expectStatus(t, ts.post(thing+"/cover.json", url.Values{"file": {"/thing/file/" + ts.hash + "/9.png"}}), http.StatusBadRequest)

	rec := ts.post("/thing/saveMetadata/"+ts.hash, url.Values{"title": {"Renamed"}, "tagsText": {"Glasses\nHat"}})
	expectStatus(t, rec, http.StatusFound)
	expectBody(t, ts.get("/?q="+url.QueryEscape(`tags_kw:"Hat"`)), "Renamed")
	expectStatus(t, ts.get("/thing/saveMetadata/"+ts.hash), http.StatusMethodNotAllowed)
}

func TestFilesAndThumbnails(t *testing.T) {
	ts := newTestServer(t, false)
	base := "/thing/file/" + ts.hash

	rec := ts.get(base + "/1.png")
	expectStatus(t, rec, http.StatusOK)
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content type = %q", ct)
	}

	for i, source := range []string{"generated", "cache"} {
		rec = ts.get(base + "/1.png?size=thumb")
		expectStatus(t, rec, http.StatusOK)
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Fatalf("%s: content type = %q", source, ct)
		}
		img, _, err := image.Decode(rec.Body)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if w := img.Bounds().Dx(); w != 256 {
			t.Errorf("%s: thumbnail width = %d, want 256", source, w)
		}
		if i == 0 {
			if _, err := os.Stat(filepath.Join(ts.cfg.CacheDir, "thing/file", ts.hash, "1.png")); err != nil {
				t.Fatalf("thumbnail not cached: %v", err)
			}
		}
	}

	// a failed thumbnail must not be cached by the browser
	rec = ts.get(base + "/missing.png?size=thumb")
	expectStatus(t, rec, http.StatusBadRequest)
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestReindexFromSystemPage(t *testing.T) {
	ts := newTestServer(t, false)

	rec := ts.post("/system", url.Values{"action": {"reindex"}})
	expectStatus(t, rec, http.StatusFound)
	if !ts.cat.Reindex.Wait(10 * time.Second) {
		t.Fatal("reindex didn't finish")
	}

	var status catalog.ReindexStatus
	decodeJSON(t, ts.get("/system?action=reindexStatus"), &status)
	if status.Running || status.Ok != 1 || status.Total != 1 {
		t.Errorf("status = %+v", status)
	}

	var reload JsonResponse
	if err := os.WriteFile(filepath.Join(ts.cfg.DatabaseDir, "files.txt"), []byte(bookKey+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	decodeJSON(t, ts.post("/system", url.Values{"action": {"reload"}}), &reload)
	if reload.Message != "1 files found (previously 1 files)" {
		t.Errorf("reload = %+v", reload)
	}

	expectStatus(t, ts.post("/system", url.Values{"action": {"nope"}}), http.StatusInternalServerError)
}
