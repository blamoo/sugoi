package library

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/facette/natsort"
	"github.com/mholt/archives"

	"github.com/blamoo/sugoi/internal/fsutil"
)

// imageExtensions are the archive entries shown as pages.
// #TODO move this list of extensions to config file
var imageExtensions = []string{".png", ".jpg", ".jpeg", ".webp"}

// isImageFile matches the extension case-insensitively: many archives use
// names like 001.JPG.
func isImageFile(name string) bool {
	return slices.Contains(imageExtensions, strings.ToLower(filepath.Ext(name)))
}

// Pages lists the image entries of the archive, naturally sorted. The list is
// cached in <CacheDir>/thing/file/<hash>/.files (empty lists aren't cached).
func (lib *Library) Pages(f *File) ([]string, error) {
	cacheFile := path.Join(lib.cacheDir, "thing/file", f.Hash, ".files")

	b, err := os.ReadFile(cacheFile)
	if err == nil {
		var files []string
		for line := range strings.SplitSeq(string(b), "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 0 {
				files = append(files, line)
			}
		}
		return files, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	slog.Debug("file list cache miss", "file", cacheFile)

	fsys, err := archives.FileSystem(context.Background(), f.RealLocation(), nil)
	if err != nil {
		return nil, err
	}

	var files []string
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isImageFile(p) {
			files = append(files, p)
		}
		return nil
	})

	natsort.Sort(files)

	if len(files) == 0 {
		slog.Debug("not caching empty file list", "file", cacheFile)
		return files, nil
	}

	joined := strings.Join(files, "\n")
	err = fsutil.WriteFileAtomic(cacheFile, 0644, false, func(w io.Writer) error {
		_, err := io.WriteString(w, joined)
		return err
	})
	if err != nil {
		slog.Warn("caching file list", "file", cacheFile, "err", err)
	} else {
		slog.Debug("file list cached", "file", cacheFile, "bytes", len(joined))
	}

	return files, nil
}

// Open opens one entry of the archive. Close the returned closers when done.
func (lib *Library) Open(f *File, name string) (io.Reader, fsutil.MultiCloser, error) {
	var closers fsutil.MultiCloser

	if len(name) == 0 || strings.HasSuffix(name, "/") {
		return nil, closers, fmt.Errorf("invalid file: %s", name)
	}

	fsys, err := archives.FileSystem(context.Background(), f.RealLocation(), nil)
	if err != nil {
		return nil, closers, err
	}

	entry, err := fsys.Open(name)
	if err != nil {
		return nil, closers, fmt.Errorf("couldn't read file %s from %s", name, f.RealLocation())
	}
	closers = append(closers, entry)

	return entry, closers, nil
}
