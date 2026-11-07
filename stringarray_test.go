package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStringArrayUnmarshal(t *testing.T) {
	cases := map[string][]string{
		`"one"`:          {"one"},
		`["a","b"]`:      {"a", "b"},
		`null`:           {},
		`[]`:             {},
		`["only one"]`:   {"only one"},
		`"with \"esc\""`: {`with "esc"`},
	}

	for in, want := range cases {
		var got StringArray
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual([]string(got), want)) {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}

	var bad StringArray
	if err := json.Unmarshal([]byte(`123`), &bad); err == nil {
		t.Error("number: expected an error")
	}
}

func TestSetFromTextAreaReplaces(t *testing.T) {
	s := StringArray{"old"}
	s.SetFromTextArea("  Long Hair \n\n Glasses\r\n")

	want := []string{"Long Hair", "Glasses"}
	if !reflect.DeepEqual([]string(s), want) {
		t.Fatalf("got %q, want %q", s, want)
	}

	s.SetFromTextArea("")
	if len(s) != 0 {
		t.Fatalf("empty text area should clear, got %q", s)
	}
}

func TestIdArray(t *testing.T) {
	cases := map[string][]string{
		// array, as written by theplan
		`["/item/1970/abc123"]`: {"/item/1970/abc123"},
		// older object form, sorted by source
		`{"SourceB":"/item/1","SourceA":"/view/2"}`: {"SourceA//view/2", "SourceB//item/1"},
		`null`: nil,
		`[]`:   {},
	}

	for in, want := range cases {
		var got IdArray
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual([]string(got), want)) {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}

	var bad IdArray
	if err := json.Unmarshal([]byte(`"just a string"`), &bad); err == nil {
		t.Error("string: expected an error instead of silently dropping the ids")
	}
}

func TestIdArrayRoundTrip(t *testing.T) {
	in := []byte(`{"id_source":["/item/1970/abc123","/view/5"]}`)

	var meta FileMetadataStatic
	if err := json.Unmarshal(in, &meta); err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}

	var again FileMetadataStatic
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.IdSource, meta.IdSource) {
		t.Fatalf("ids changed on save: %q -> %q", meta.IdSource, again.IdSource)
	}
}
