package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"

	"github.com/blamoo/sugoi/internal/fsutil"
)

func RenderThumbCache(w http.ResponseWriter, r *http.Request, hash string, file string) bool {
	if !config.CacheThumbnails {
		return false
	}

	fname := config.CacheFile("thing/file", hash, file)
	os.MkdirAll(path.Dir(fname), 0755)

	f, err := os.Open(fname)

	if os.IsNotExist(err) {
		slog.Debug("thumbnail cache miss", "file", fname)
		return false
	}

	if err != nil {
		slog.Warn("thumbnail cache read", "file", fname, "err", err)
		return false
	}
	defer f.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	SetCacheHeader(w, r, 31536000)

	_, err = io.Copy(w, f)
	if err != nil {
		slog.Debug("thumbnail cache send", "file", fname, "err", err)
		return false

	}

	slog.Debug("thumbnail cache hit", "file", fname)
	return true
}

// ThumbCacheWrite stores an encoded thumbnail. Written atomically so a
// concurrent request never serves a half-written image.
func ThumbCacheWrite(hash string, file string, data []byte) {
	fname := config.CacheFile("thing/file", hash, file)

	err := fsutil.WriteFileAtomic(fname, 0644, false, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
	if err != nil {
		slog.Warn("thumbnail cache write", "file", fname, "err", err)
		return
	}

	slog.Debug("thumbnail cached", "file", fname)
}
