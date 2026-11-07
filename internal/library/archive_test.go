package library

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blamoo/sugoi/internal/config"
	"github.com/blamoo/sugoi/internal/fsutil"
)

func TestIsImageFile(t *testing.T) {
	cases := map[string]bool{
		"001.jpg":         true,
		"001.JPG":         true,
		"dir/002.Jpeg":    true,
		"cover.PNG":       true,
		"page.webp":       true,
		"ComicInfo.xml":   false,
		"notes.txt":       false,
		"jpg":             false,
		"archive.jpg.zip": false,
	}

	for name, want := range cases {
		if got := isImageFile(name); got != want {
			t.Errorf("isImageFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func writeZip(t *testing.T, name string, entries ...string) {
	t.Helper()
	out, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for _, e := range entries {
		w, err := zw.Create(e)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, "content of "+e)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	out.Close()
}

func TestPagesAndOpen(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "book.cbz"), "10.jpg", "2.JPG", "sub/1.png", "ComicInfo.xml", "notes.txt")

	cfg := &config.Config{DatabaseDir: dir, CacheDir: t.TempDir(), DirVars: map[string]string{"D": dir}}
	lib := New(cfg, []string{"{{D}}/book.cbz"})
	f := lib.List[0]

	pages, err := lib.Pages(f)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2.JPG", "10.jpg", "sub/1.png"}; !reflect.DeepEqual(pages, want) {
		t.Fatalf("pages = %q, want %q (natural sort, images only)", pages, want)
	}

	// second call comes from the cache file
	cached := filepath.Join(cfg.CacheDir, "thing/file", f.Hash, ".files")
	if _, err := os.Stat(cached); err != nil {
		t.Fatalf("page list not cached: %v", err)
	}
	again, err := lib.Pages(f)
	if err != nil || !reflect.DeepEqual(again, pages) {
		t.Fatalf("cached pages = %q, %v", again, err)
	}

	r, closers, err := lib.Open(f, "sub/1.png")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	fsutil.MultiClose(closers)
	if string(b) != "content of sub/1.png" {
		t.Fatalf("read %q", b)
	}

	if _, _, err := lib.Open(f, "missing.png"); err == nil {
		t.Error("opening a missing entry should fail")
	}
	if _, _, err := lib.Open(f, "sub/"); err == nil {
		t.Error("opening a directory should fail")
	}
}
