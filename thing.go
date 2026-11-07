package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/blamoo/sugoi/internal/metadata"

	"github.com/blamoo/sugoi/internal/fsutil"
)

type Thing struct {
	File *FilePointer
	FileMetadataDynamic
	FileMetadataStatic
}

func NewThingFromHash(hash string) (*Thing, error) {
	file, err := filePointers.Find(hash)
	if err != nil {
		return nil, err
	}

	ret := Thing{}
	ret.File = file

	static, err := metadata.ReadStatic(file.StaticMetaPath())
	if err != nil {
		slog.Error("reading static metadata", "file", file.StaticMetaPath(), "err", err)
		ret.FileMetadataStatic = FileMetadataStatic{}
	} else {
		ret.FileMetadataStatic = *static
	}
	fillStaticPlaceholders(&ret.FileMetadataStatic, file)

	dynamic, err := metadata.ReadDynamic(file.DynamicMetaPath())
	if err != nil {
		slog.Error("reading dynamic metadata", "file", file.DynamicMetaPath(), "err", err)
		ret.FileMetadataDynamic = FileMetadataDynamic{}
	} else {
		ret.FileMetadataDynamic = *dynamic
	}

	return &ret, nil
}

func (t *Thing) FillEmptyFields(file *FilePointer) {
	if len(t.Title) == 0 {
		t.Title = file.PlaceholderTitle()
	}

	if len(t.Collection) == 0 {
		t.Collection = file.PlaceholderCollection()
	}

	// if len(this.Cover) == 0 {
	// 	this.Cover = config.DefaultCoverFileName
	// }
}

func (t *Thing) Key() string {
	return t.File.Key
}

func (t *Thing) Hash() string {
	return t.File.Hash
}

// updateDynamic reloads dynamic.json from disk under the item's lock, applies
// fn and saves, so concurrent requests (two tabs, rating + mark) don't
// overwrite each other's changes.
// ErrReindexRunning is returned by metadata writes while a full reindex runs:
// the reindex may have already queued an older copy of the item, which would
// overwrite the new one in the index (the file on disk would stay correct).
var ErrReindexRunning = errors.New("a full reindex is running, try again when it finishes")

func (t *Thing) updateDynamic(fn func(d *FileMetadataDynamic) error) error {
	t.File.Lock()
	defer t.File.Unlock()

	if reindexJob.Running {
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

	t.FileMetadataDynamic = *current
	return reindexFile(t.File)
}

// SaveStaticFromForm applies the edit form onto the static.json stored on disk
// (not the in-memory copy, which has placeholder title/pages filled in).
func (t *Thing) SaveStaticFromForm(form url.Values) error {
	t.File.Lock()
	defer t.File.Unlock()

	if reindexJob.Running {
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

	t.FileMetadataStatic = *current
	fillStaticPlaceholders(&t.FileMetadataStatic, t.File)
	return reindexFile(t.File)
}

// TrySaveRating sets the rating; with toggle, setting the current rating again
// clears it.
func (t *Thing) TrySaveRating(rating int, toggle bool) error {
	return t.updateDynamic(func(d *FileMetadataDynamic) error {
		if toggle && d.Rating == rating {
			rating = 0
		}
		d.Rating = rating
		d.UpdatedAt = time.Now()
		return nil
	})
}

func (t *Thing) AddMark() error {
	return t.updateDynamic(func(d *FileMetadataDynamic) error {
		d.Marks++
		return nil
	})
}

func (t *Thing) PushRead() error {
	return t.updateDynamic(func(d *FileMetadataDynamic) error {
		d.ReadCount++

		now := time.Now()
		if d.FirstReadAt.IsZero() {
			d.FirstReadAt = now
		}
		d.LastReadAt = now
		return nil
	})
}

func (t *Thing) SubMark() error {
	return t.updateDynamic(func(d *FileMetadataDynamic) error {
		d.Marks--
		return nil
	})
}

func (t *Thing) TrySaveCover(file string, isUpdate bool) error {
	prefix := t.FileUrlPrefix()
	realLocation := t.File.RealLocation()
	newCover, _ := filepath.Rel(prefix, file)

	files, err := t.ListFiles()
	if err != nil {
		return err
	}

	if !slices.Contains(files, file) {
		return fmt.Errorf("file %s doesn't exists in %s", newCover, realLocation)
	}

	return t.updateDynamic(func(d *FileMetadataDynamic) error {
		d.Cover = newCover
		if isUpdate {
			d.UpdatedAt = time.Now()
		}
		return nil
	})
}

func (t *Thing) CoverImageUrl() string {
	if len(t.Cover) > 0 {
		return t.Cover
	}

	f, err := t.ListFilesRaw()
	if err != nil || len(f) == 0 {
		return "/static/empty.jpg"
	}

	if t.Thumbnail < 0 || t.Thumbnail >= len(f) {
		return "/static/empty.jpg"
	}

	t.TrySaveCover(f[t.Thumbnail], false)
	return f[t.Thumbnail]
}

func (t *Thing) FileUrlPrefix() string {
	return fmt.Sprintf("/thing/file/%s", t.Hash())
}

func (t *Thing) FileUrl(f string) string {
	if f == "/static/empty.jpg" {
		return "/static/empty.jpg"
	}
	return fmt.Sprintf("%s/%s", t.FileUrlPrefix(), url.PathEscape(strings.TrimLeft(f, "/")))
}

func (t *Thing) ReadFileUrl(i int) string {
	return fmt.Sprintf("%s/%d", t.ReadUrl(), i)
}

func (t *Thing) ThumbUrl(f string) string {
	if len(f) > 0 {
		return fmt.Sprintf("%s?size=thumb", t.FileUrl(f))
	}
	return "/static/empty-256.jpg"
}

func (t *Thing) DetailsUrl() string {
	return fmt.Sprintf("/thing/details/%s", t.Hash())
}

func (t *Thing) ReadUrl() string {
	return fmt.Sprintf("/thing/read/%s", t.Hash())
}

func (t *Thing) SortedTags(options ...string) map[string][]SearchTerm {
	ret := make(map[string][]SearchTerm)

	for _, id := range t.Id {
		ret["Id"] = append(ret["Id"], NewSearchTermInt("id", id))
	}

	for _, artist := range t.Artist {
		if len(artist) != 0 {
			ret["Artist"] = append(ret["Artist"], NewSearchTerm("artist", artist))
		}
	}

	for _, circle := range t.Circle {
		if len(circle) != 0 {
			ret["Circle"] = append(ret["Circle"], NewSearchTerm("circle", circle))
		}
	}

	if len(t.Language) != 0 {
		ret["Language"] = append(ret["Language"], NewSearchTerm("language", t.Language))
	}

	for _, parody := range t.Parody {
		if len(parody) != 0 {
			ret["Parody"] = append(ret["Parody"], NewSearchTerm("parody", parody))
		}
	}

	for _, magazine := range t.Magazine {
		if len(magazine) != 0 {
			ret["Magazine"] = append(ret["Magazine"], NewSearchTerm("magazine", magazine))
		}
	}

	for _, publisher := range t.Publisher {
		if len(publisher) != 0 {
			ret["Publisher"] = append(ret["Publisher"], NewSearchTerm("publisher", publisher))
		}
	}

	for _, tag := range t.Tags {
		if len(tag) != 0 {
			ret["Tags"] = append(ret["Tags"], NewSearchTerm("tags", tag))
		}
	}

	if slices.Contains(options, "all") {
		if len(t.Collection) != 0 {
			ret["Collection"] = append(ret["Collection"], NewSearchTerm("collection", t.Collection))
		}
	}

	return ret
}

func (t *Thing) HtmlTable(options ...string) template.HTML {
	var b bytes.Buffer

	data := struct {
		Thing   *Thing
		Series  map[string]string
		NoMarks bool
	}{
		Thing: t,
	}

	if slices.Contains(options, "nomarks") {
		data.NoMarks = true
	}

	tmpl, err := currentTemplates()
	if err != nil {
		return template.HTML(err.Error())
	}

	err = tmpl.ExecuteTemplate(&b, "e_details_table.gohtml", data)
	if err != nil {
		return template.HTML(err.Error())
	}
	return template.HTML(b.String())
}

func (t *Thing) UrlButtons() []template.HTML {
	var ret []template.HTML

	for _, url := range t.Urls {
		var img string
		var title string

		if strings.HasPrefix(url, "https://schale.network/") {
			img = `<img class="source-icon" src="/static/source-icons/schale.png" alt="Schale" />`
			title = "Schale"
		} else if strings.HasPrefix(url, "https://hentainexus.com/") {
			img = `<img class="source-icon" src="/static/source-icons/hentainexus.png" alt="HentaiNexus" />`
			title = "Schale"
		} else if strings.HasPrefix(url, "https://www.fakku.net/") {
			img = `<img class="source-icon" src="/static/source-icons/fakku.png" alt="Fakku" />`
			title = "Fakku"
		} else if strings.HasPrefix(url, "https://doujin.io/") {
			img = `<img class="source-icon" src="/static/source-icons/j18.png" alt="J18" />`
			title = "J18"
		} else if strings.HasPrefix(url, "https://irodoricomics.com/") {
			img = `<img class="source-icon" src="/static/source-icons/irodori.png" alt="Irodori" />`
			title = "Irodori"
		} else if strings.HasPrefix(url, "https://www.projecthentai.com/") {
			img = `<img class="source-icon" src="/static/source-icons/projecthentai.png" alt="Project Hentai" />`
			title = "Project Hentai"
		} else if strings.HasPrefix(url, "https://raw.githubusercontent.com/ccdc06/metadata/") {
			img = `<img class="source-icon" src="/static/source-icons/ccdc06.png" alt="CCDC06" />`
			title = "CCDC06"
		} else {
			img = `<i class="fa-solid fa-link fa-lg"></i>`
			title = "url"
		}

		ret = append(ret, template.HTML(fmt.Sprintf(`<a target="_blank" class="badge bg-secondary text-decoration-none p-1" title="%s" href="%s">%s</a>`, html.EscapeString(title), html.EscapeString(url), img)))
	}

	return ret
}

func (t *Thing) CollectionDetailsUrl() string {
	u := new(url.URL)
	u.Path = "/"
	q := u.Query()
	q.Set("q", BuildBleveSearchTerm(searchField("collection"), t.Collection))
	u.RawQuery = q.Encode()
	return u.String()
}

func (t *Thing) SaveMetadataUrl() string {
	return fmt.Sprintf("/thing/saveMetadata/%s", t.Hash())
}

func (t *Thing) EditMetadataUrl() string {
	return fmt.Sprintf("/thing/editMetadata/%s", t.Hash())
}

func (t *Thing) FilledStarsRepeat(str string) string {
	i := max(min(t.Rating, 5), 0)

	return strings.Repeat(str, i)
}

func (t *Thing) EmptyStarsRepeat(str string) string {
	i := max(min(t.Rating, 5), 0)

	return strings.Repeat(str, 5-i)
}

func (t *Thing) ListFiles() ([]string, error) {
	raw, err := t.ListFilesRaw()
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(raw))
	for key, val := range raw {
		ret[key] = t.FileUrl(val)
	}
	return ret, nil
}

func (t *Thing) ListFilesRaw() ([]string, error) {
	if len(t.FileMetadataStatic.Files) > 0 {
		return t.FileMetadataStatic.Files, nil
	}

	return filePointers.Pages(t.File)
}

func (t *Thing) getFileReader(file string) (io.Reader, fsutil.MultiCloser, error) {
	return filePointers.Open(t.File, file)
}

func (t *Thing) SeriesUrls() map[string]string {
	ret := make(map[string]string)

	for _, name := range t.Series {
		u := new(url.URL)
		u.Path = "/"
		q := u.Query()
		q.Set("q", BuildBleveSearchTerm("series", name))
		u.RawQuery = q.Encode()

		ret[u.String()] = name
	}

	return ret
}

func (t *Thing) UpdatedAtRelative() string {
	if t.UpdatedAt.IsZero() {
		return "-"
	}

	return fmt.Sprintf("%s (%s)", t.UpdatedAt.Format("02/01/2006 03:04"), RelativeTime(t.UpdatedAt))
}

func (t *Thing) SearchTags() []SearchTerm {
	var ret []SearchTerm

	for _, tags := range t.SortedTags("all") {
		for _, tag := range tags {
			if tag.Type == TYPE_TEXT {
				ret = append(ret, NewSearchTerm(tag.Key, tag.Label))
			}
		}
	}

	return ret
}
