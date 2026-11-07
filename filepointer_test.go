package main

import (
	"crypto/sha1"
	"fmt"
	"testing"
)

func TestFilePointerPaths(t *testing.T) {
	setupTestEnv(t)
	config.DirVars = map[string]string{"LIB": "/mnt/comics"}

	key := "{{LIB}}/Some Collection/[Artist] Title (Magazine #1).cbz"
	fp, err := NewFilePointer(key)
	if err != nil {
		t.Fatal(err)
	}

	if want := fmt.Sprintf("%x", sha1.Sum([]byte(key))); fp.Hash != want {
		t.Errorf("hash = %s, want sha1 of the raw line %s", fp.Hash, want)
	}
	if want := "lib/Some Collection/[Artist] Title (Magazine #1).cbz"; fp.PathKey != want {
		t.Errorf("PathKey = %q, want %q", fp.PathKey, want)
	}
	if want := "/mnt/comics/Some Collection/[Artist] Title (Magazine #1).cbz"; fp.RealLocation() != want {
		t.Errorf("RealLocation = %q, want %q", fp.RealLocation(), want)
	}
	if got := fp.PlaceholderTitle(); got != "Title" {
		t.Errorf("PlaceholderTitle = %q, want %q", got, "Title")
	}
	if got := fp.PlaceholderCollection(); got != "Some Collection" {
		t.Errorf("PlaceholderCollection = %q", got)
	}
}
