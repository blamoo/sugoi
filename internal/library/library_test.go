package library

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blamoo/sugoi/internal/config"
)

func TestFilePaths(t *testing.T) {
	cfg := &config.Config{DatabaseDir: "db", DirVars: map[string]string{"LIB": "/mnt/comics"}}

	key := "{{LIB}}/Some Collection/[Artist] Title (Magazine #1).cbz"
	f := NewFile(cfg, key)

	if want := fmt.Sprintf("%x", sha1.Sum([]byte(key))); f.Hash != want {
		t.Errorf("hash = %s, want sha1 of the raw line %s", f.Hash, want)
	}
	if want := "lib/Some Collection/[Artist] Title (Magazine #1).cbz"; f.PathKey != want {
		t.Errorf("PathKey = %q, want %q", f.PathKey, want)
	}
	if want := "db/meta/lib/Some Collection/[Artist] Title (Magazine #1).cbz/static.json"; f.StaticMetaPath() != want {
		t.Errorf("StaticMetaPath = %q", f.StaticMetaPath())
	}
	if want := "/mnt/comics/Some Collection/[Artist] Title (Magazine #1).cbz"; f.RealLocation() != want {
		t.Errorf("RealLocation = %q, want %q", f.RealLocation(), want)
	}
	if got := f.PlaceholderTitle(); got != "Title" {
		t.Errorf("PlaceholderTitle = %q, want %q", got, "Title")
	}
	if got := f.PlaceholderCollection(); got != "Some Collection" {
		t.Errorf("PlaceholderCollection = %q", got)
	}
}

func TestLoadSkipsCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\n// another\n; and another\n\n  {{A}}/one.cbz  \n{{A}}/two.zip\n"
	if err := os.WriteFile(filepath.Join(dir, "files.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lib, err := Load(&config.Config{DatabaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.List) != 2 || lib.List[0].Key != "{{A}}/one.cbz" || lib.List[1].Key != "{{A}}/two.zip" {
		t.Fatalf("got %+v", lib.List)
	}
	if _, err := lib.Find(lib.List[1].Hash); err != nil {
		t.Error(err)
	}
	if _, err := lib.Find("nope"); err == nil {
		t.Error("Find of an unknown hash should fail")
	}
}

func TestLoadMissingFileFails(t *testing.T) {
	if _, err := Load(&config.Config{DatabaseDir: t.TempDir()}); err == nil {
		t.Fatal("expected an error for a missing files.txt")
	}
}
