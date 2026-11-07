package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeString(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteFileAtomicCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "a", "b", "file.json")

	for _, content := range []string{`{"v":1}`, `{"v":2}`} {
		err := WriteFileAtomic(name, 0600, true, writeString(content))
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != content {
			t.Fatalf("content = %q, want %q", got, content)
		}
	}

	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("perm = %v, want 0600", info.Mode().Perm())
	}

	if names := dirEntries(t, filepath.Dir(name)); len(names) != 1 {
		t.Fatalf("leftover files: %v", names)
	}
}

func TestWriteFileAtomicKeepsOldContentOnError(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file.json")

	err := WriteFileAtomic(name, 0644, false, writeString("old"))
	if err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err = WriteFileAtomic(name, 0644, false, func(w io.Writer) error {
		io.WriteString(w, "partial")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	got, _ := os.ReadFile(name)
	if string(got) != "old" {
		t.Fatalf("content = %q, want old content kept", got)
	}

	if names := dirEntries(t, dir); len(names) != 1 {
		t.Fatalf("temporary file left behind: %v", names)
	}
}

func TestWriteJSON(t *testing.T) {
	name := filepath.Join(t.TempDir(), "dynamic.json")

	err := WriteJSON(name, map[string]int{"rating": 4})
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n\t\"rating\": 4\n}\n"; string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
