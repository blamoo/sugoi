package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blamoo/sugoi/internal/metadata"
)

func TestGetFillsPlaceholders(t *testing.T) {
	c := newTestCatalog(t, testKey)

	thing := c.mustGet(t, testKey)
	if thing.Title != "Some Title" || thing.Collection != "collection" {
		t.Fatalf("placeholders: title=%q collection=%q", thing.Title, thing.Collection)
	}

	// placeholders are never written to disk
	if _, err := os.Stat(thing.File.StaticMetaPath()); !os.IsNotExist(err) {
		t.Fatalf("static.json written on read: %v", err)
	}

	if _, err := c.Get("unknown"); err == nil {
		t.Fatal("Get of an unknown hash should fail")
	}
}

func TestReindexIndexesEveryFile(t *testing.T) {
	var keys []string
	for i := 0; i < 12; i++ {
		keys = append(keys, fmt.Sprintf("{{LIB}}/col/item %02d.cbz", i))
	}
	c := newTestCatalog(t, keys...)

	// metadata written straight to disk, as an importer would
	first := c.mustGet(t, keys[0])
	if err := metadata.Write(first.File.StaticMetaPath(), metadata.Static{Title: "Imported", Tags: metadata.StringArray{"Glasses"}}); err != nil {
		t.Fatal(err)
	}

	if err := c.Reindex.Start(); err != nil {
		t.Fatal(err)
	}
	if err := c.Reindex.Start(); err == nil {
		t.Error("a second Start while running should fail")
	}
	if !c.Reindex.Wait(10 * time.Second) {
		t.Fatal("reindex didn't finish")
	}

	s := c.Reindex.Status()
	if s.Running || s.Ok != 12 || s.Error != 0 || s.Total != 12 || s.FinishTime.IsZero() {
		t.Fatalf("status = %+v", s)
	}
	if s.Log[len(s.Log)-1] != "Reindex finished" {
		t.Errorf("last log line = %q", s.Log[len(s.Log)-1])
	}

	if n := c.searchCount(t, `tags_kw:"Glasses"`); n != 1 {
		t.Errorf("imported tag matched %d documents, want 1", n)
	}
	if n := c.searchCount(t, `collection_kw:"col"`); n != 12 {
		t.Errorf("collection matched %d documents, want 12", n)
	}

	// writes work again once it finished
	if err := first.AddMark(); err != nil {
		t.Fatal(err)
	}
}

func TestReindexCancel(t *testing.T) {
	var keys []string
	for i := 0; i < 2000; i++ {
		keys = append(keys, fmt.Sprintf("{{LIB}}/col/item %04d.cbz", i))
	}
	c := newTestCatalog(t, keys...)

	if err := c.Reindex.Start(); err != nil {
		t.Fatal(err)
	}
	c.Reindex.Cancel()
	if !c.Reindex.Wait(10 * time.Second) {
		t.Fatal("cancelled reindex didn't stop")
	}

	s := c.Reindex.Status()
	if s.Processed == s.Total {
		t.Skip("reindex finished before the cancel was seen")
	}
	for _, line := range s.Log {
		if line == "Reindex finished" {
			t.Errorf("cancelled job logged %q", line)
		}
	}
}

func TestReloadKeepsListOnError(t *testing.T) {
	c := newTestCatalog(t, testKey)
	files := filepath.Join(c.cfg.DatabaseDir, "files.txt")

	// missing files.txt: error, old list kept
	if _, _, err := c.Reload(); err == nil {
		t.Fatal("expected an error without files.txt")
	}
	if len(c.Library().List) != 1 {
		t.Fatalf("list changed after a failed reload: %d files", len(c.Library().List))
	}

	if err := os.WriteFile(files, []byte(testKey+"\n{{LIB}}/other.cbz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	prev, cur, err := c.Reload()
	if err != nil || prev != 1 || cur != 2 {
		t.Fatalf("Reload = %d, %d, %v", prev, cur, err)
	}
}
