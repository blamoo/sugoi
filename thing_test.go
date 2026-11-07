package main

import "testing"

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
