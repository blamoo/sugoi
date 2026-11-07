package catalog

import (
	"errors"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/blamoo/sugoi/internal/fsutil"
	"github.com/blamoo/sugoi/internal/library"
	"github.com/blamoo/sugoi/internal/metadata"
)

// Thing is one item: the archive plus its metadata, as loaded by Get.
type Thing struct {
	File *library.File
	metadata.Dynamic
	metadata.Static

	catalog *Catalog
}

// ErrReindexRunning is returned by metadata writes while a full reindex runs:
// the reindex may have already queued an older copy of the item, which would
// overwrite the new one in the index (the file on disk would stay correct).
var ErrReindexRunning = errors.New("a full reindex is running, try again when it finishes")

func (t *Thing) Key() string  { return t.File.Key }
func (t *Thing) Hash() string { return t.File.Hash }

// Pages lists the item's page images: the explicit "files" of static.json
// when present, otherwise the images found in the archive.
func (t *Thing) Pages() ([]string, error) {
	if len(t.Files) > 0 {
		return t.Files, nil
	}
	return t.catalog.Library().Pages(t.File)
}

// Open opens one entry of the item's archive.
func (t *Thing) Open(name string) (io.Reader, fsutil.MultiCloser, error) {
	return t.catalog.Library().Open(t.File, name)
}

// updateDynamic reloads dynamic.json from disk under the item's lock, applies
// fn and saves, so concurrent requests (two tabs, rating + mark) don't
// overwrite each other's changes.
func (t *Thing) updateDynamic(fn func(d *metadata.Dynamic) error) error {
	t.File.Lock()
	defer t.File.Unlock()

	if t.catalog.Reindex.Running() {
		return ErrReindexRunning
	}

	current, err := metadata.ReadDynamic(t.File.DynamicMetaPath())
	if err != nil {
		return err
	}

	err = fn(current)
	if err != nil {
		return err
	}

	err = metadata.Write(t.File.DynamicMetaPath(), current)
	if err != nil {
		return err
	}

	t.Dynamic = *current
	return t.catalog.reindex(t.File)
}

// SaveStaticFromForm applies the edit form onto the static.json stored on disk
// (not the in-memory copy, which has placeholder title/pages filled in).
func (t *Thing) SaveStaticFromForm(form url.Values) error {
	t.File.Lock()
	defer t.File.Unlock()

	if t.catalog.Reindex.Running() {
		return ErrReindexRunning
	}

	current, err := metadata.ReadStatic(t.File.StaticMetaPath())
	if err != nil {
		return err
	}

	err = current.ApplyForm(form)
	if err != nil {
		return err
	}

	err = metadata.Write(t.File.StaticMetaPath(), current)
	if err != nil {
		return err
	}

	t.Static = *current
	t.catalog.fillPlaceholders(&t.Static, t.File)
	return t.catalog.reindex(t.File)
}

// TrySaveRating sets the rating; with toggle, setting the current rating again
// clears it.
func (t *Thing) TrySaveRating(rating int, toggle bool) error {
	return t.updateDynamic(func(d *metadata.Dynamic) error {
		if toggle && d.Rating == rating {
			rating = 0
		}
		d.Rating = rating
		d.UpdatedAt = time.Now()
		return nil
	})
}

func (t *Thing) AddMark() error {
	return t.updateDynamic(func(d *metadata.Dynamic) error {
		d.Marks++
		return nil
	})
}

func (t *Thing) SubMark() error {
	return t.updateDynamic(func(d *metadata.Dynamic) error {
		d.Marks--
		return nil
	})
}

func (t *Thing) PushRead() error {
	return t.updateDynamic(func(d *metadata.Dynamic) error {
		d.ReadCount++

		now := time.Now()
		if d.FirstReadAt.IsZero() {
			d.FirstReadAt = now
		}
		d.LastReadAt = now
		return nil
	})
}

// SetCover stores the cover (a page path relative to the archive).
func (t *Thing) SetCover(cover string, isUpdate bool) error {
	return t.updateDynamic(func(d *metadata.Dynamic) error {
		d.Cover = cover
		if isUpdate {
			d.UpdatedAt = time.Now()
		}
		return nil
	})
}

func logMetadataError(file string, err error) {
	slog.Error("reading metadata", "file", file, "err", err)
}
