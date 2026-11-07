package main

import (
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestApplyFormKeepsFieldsNotInTheForm(t *testing.T) {
	meta := FileMetadataStatic{
		Id:        []int{123},
		IdSource:  IdArray{"/item/123/abc"},
		Circle:    StringArray{"Circle X"},
		Thumbnail: 2,
		Pages:     30,
		Files:     StringArray{"01.png", "02.png"},
		Publisher: StringArray{"Old Publisher"},
		Tags:      StringArray{"old tag"},
	}

	form := url.Values{
		"title":         {"New Title"},
		"publisherText": {"New Publisher"},
		"tagsText":      {"a\nb"},
		"artistText":    {""},
		"created_at":    {"2005-04-01T00:00:00Z"},
	}

	if err := meta.ApplyForm(form); err != nil {
		t.Fatal(err)
	}

	if meta.Title != "New Title" {
		t.Errorf("title = %q", meta.Title)
	}
	if !reflect.DeepEqual([]string(meta.Publisher), []string{"New Publisher"}) {
		t.Errorf("publisher = %q (form field is publisherText)", meta.Publisher)
	}
	if !reflect.DeepEqual([]string(meta.Tags), []string{"a", "b"}) {
		t.Errorf("tags = %q, want replaced", meta.Tags)
	}
	if len(meta.Artist) != 0 {
		t.Errorf("artist = %q, want cleared", meta.Artist)
	}
	if meta.CreatedAt.Year() != 2005 {
		t.Errorf("created_at = %v", meta.CreatedAt)
	}

	// not part of the form: untouched
	if !reflect.DeepEqual(meta.Id, []int{123}) || meta.IdSource[0] != "/item/123/abc" ||
		meta.Circle[0] != "Circle X" || meta.Thumbnail != 2 || meta.Pages != 30 || len(meta.Files) != 2 {
		t.Errorf("fields outside the form changed: %+v", meta)
	}
}

func TestApplyFormRejectsBadDate(t *testing.T) {
	var meta FileMetadataStatic
	err := meta.ApplyForm(url.Values{"created_at": {"yesterday"}})
	if err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestMissingMetadataFileIsCreated(t *testing.T) {
	name := t.TempDir() + "/meta/item/static.json"

	meta, err := NewFileMetadataStaticFromFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != "" {
		t.Errorf("expected empty metadata, got %+v", meta)
	}

	b, err := os.ReadFile(name)
	if err != nil || string(b) != "{}" {
		t.Fatalf("file = %q, %v; want {}", b, err)
	}
}
