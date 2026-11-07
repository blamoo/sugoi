package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes a file so that readers (and a crash or power loss)
// see either the old content or the new one, never a truncated mix: the data
// goes to a temporary file in the same directory, which then replaces the
// target with a rename. Parent directories are created as needed.
//
// durable also fsyncs the data before the rename; use it for files that
// can't be regenerated (metadata, config), not for caches.
func WriteFileAtomic(name string, perm os.FileMode, durable bool, write func(w io.Writer) error) error {
	dir := filepath.Dir(name)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(name)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Any early return leaves no temporary file behind.
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	err = write(tmp)
	if err != nil {
		return err
	}

	if durable {
		err = tmp.Sync()
		if err != nil {
			return err
		}
	}

	err = tmp.Close()
	if err != nil {
		return err
	}

	err = os.Chmod(tmpName, perm)
	if err != nil {
		return err
	}

	err = os.Rename(tmpName, name)
	if err != nil {
		return err
	}
	committed = true

	if durable {
		syncDir(dir)
	}
	return nil
}

// syncDir makes the rename itself durable. Best effort: not every platform
// allows opening and syncing a directory.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	d.Sync()
	d.Close()
}

// writeJsonFile atomically writes v as tab-indented JSON.
func writeJsonFile(file string, v any) error {
	return WriteFileAtomic(file, 0644, true, func(w io.Writer) error {
		e := json.NewEncoder(w)
		e.SetIndent("", "\t")
		return e.Encode(v)
	})
}
