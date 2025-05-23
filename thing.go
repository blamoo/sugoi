package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/facette/natsort"
	"github.com/mholt/archives"
)

type Thing struct {
	File *FilePointer
	FileMetadataDynamic
	FileMetadataStatic
}

func NewThingFromHash(hash string) (*Thing, error) {
	file, found := filePointers.ByHash[hash]
	if !found {
		return nil, fmt.Errorf("file %s not found", hash)
	}

	ret := Thing{}
	ret.File = file

	var err error
	static, err := NewFileMetadataStaticFromFile(file.StaticMetaPath())
	if err != nil {
		log.Println(err)
		ret.FileMetadataStatic = FileMetadataStatic{}
	} else {
		ret.FileMetadataStatic = *static
	}
	ret.FileMetadataStatic.FillEmptyFields(file)

	dynamic, err := NewFileMetadataDynamicFromFile(file.DynamicMetaPath())
	if err != nil {
		log.Println(err)
		ret.FileMetadataDynamic = FileMetadataDynamic{}
	} else {
		ret.FileMetadataDynamic = *dynamic
	}
	ret.FileMetadataDynamic.FillEmptyFields(file)

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

func (t *Thing) BuildPathKey() string {
	p := t.Key()

	var re = regexp.MustCompile(`{{.*?}}`)

	p = re.ReplaceAllStringFunc(p, func(s string) string {
		s = strings.Replace(s, "{{", "", -1)
		s = strings.Replace(s, "}}", "", -1)
		return strings.ToLower(s)
	})

	return path.Clean(p)
}

func (t *Thing) TrySaveDynamic() error {
	var err error
	metaFilePath := t.File.DynamicMetaPath()
	_, err = os.Stat(metaFilePath)
	if os.IsNotExist(err) {
		os.MkdirAll(path.Dir(metaFilePath), 0755)
	}

	f, err := os.OpenFile(metaFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "\t")

	err = e.Encode(t.FileMetadataDynamic)
	if err != nil {
		return err
	}

	return nil
}

func (t *Thing) TrySaveRating(rating int) error {
	old := t.FileMetadataDynamic
	t.Rating = rating
	t.UpdatedAt = time.Now()

	err := t.TrySaveDynamic()
	if err != nil {
		t.FileMetadataDynamic = old
		return err
	}
	t.File.Reindex()

	return nil
}

func (t *Thing) AddMark() error {
	old := t.FileMetadataDynamic
	t.Marks++

	err := t.TrySaveDynamic()
	if err != nil {
		t.FileMetadataDynamic = old
		return err
	}
	t.File.Reindex()

	return nil
}

func (t *Thing) PushRead() error {
	old := t.FileMetadataDynamic
	t.ReadCount++

	now := time.Now()
	if t.FileMetadataDynamic.FirstReadAt.IsZero() {
		t.FileMetadataDynamic.FirstReadAt = now
	}
	t.FileMetadataDynamic.LastReadAt = now

	err := t.TrySaveDynamic()
	if err != nil {
		t.FileMetadataDynamic = old
		return err
	}
	t.File.Reindex()

	return nil
}

func (t *Thing) SubMark() error {
	old := t.FileMetadataDynamic
	t.Marks--

	err := t.TrySaveDynamic()
	if err != nil {
		t.FileMetadataDynamic = old
		return err
	}
	t.File.Reindex()

	return nil
}

func (t *Thing) TrySaveCover(file string, isUpdate bool) error {
	prefix := t.FileUrlPrefix()
	realLocation := t.File.RealLocation()
	newCover, _ := filepath.Rel(prefix, file)

	files, err := t.ListFiles()
	if err != nil {
		return err
	}

	for _, cfile := range files {
		if cfile == file {
			old := t.FileMetadataDynamic
			t.Cover = newCover
			if isUpdate {
				t.UpdatedAt = time.Now()
			}

			err := t.TrySaveDynamic()
			if err != nil {
				t.FileMetadataDynamic = old
				return err
			}
			t.File.Reindex()

			return nil
		}
	}

	return fmt.Errorf("file %s doesn't exists in %s", newCover, realLocation)
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

func (t *Thing) SortedTags() map[string][]SearchTerm {
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

	err := templateStore.ExecuteTemplate(&b, "e_details_table.gohtml", data)
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

		if strings.HasPrefix(url, "https://hentag.com/") {
			img = `<img class="source-icon" src="/static/source-icons/hentag.png" alt="Hentag" />`
			title = "Hentag"
		} else if strings.HasPrefix(url, "https://schale.network/") {
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
	q.Set("q", BuildBleveSearchTerm("Collection", t.Collection))
	u.RawQuery = q.Encode()
	return u.String()
}

func (t *Thing) SearchMetadataUrl() string {
	return fmt.Sprintf("/thing/searchMetadata/%s", t.Hash())
}

func (t *Thing) SaveMetadataUrl() string {
	return fmt.Sprintf("/thing/saveMetadata/%s", t.Hash())
}

func (t *Thing) EditMetadataUrl() string {
	return fmt.Sprintf("/thing/editMetadata/%s", t.Hash())
}

func (t *Thing) FilledStarsRepeat(str string) string {
	i := t.Rating

	if i > 5 {
		i = 5
	}

	if i < 0 {
		i = 0
	}

	return strings.Repeat(str, i)
}

func (t *Thing) EmptyStarsRepeat(str string) string {
	i := t.Rating

	if i > 5 {
		i = 5
	}

	if i < 0 {
		i = 0
	}

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

	var files []string

	fname := config.CacheFile("thing/file", t.Hash(), ".files")
	os.MkdirAll(path.Dir(fname), 0755)

	f, err := os.Open(fname)

	if err != nil {
		defer f.Close()
	}

	if !os.IsNotExist(err) {
		b, err := io.ReadAll(f)
		if err == nil {
			split := strings.Split(string(b), "\n")

			for _, line := range split {
				line = strings.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				files = append(files, line)
			}
		}
		return files, nil
	}
	debugPrintf("File list cache miss: %s\n", fname)

	compressedFileName := t.File.RealLocation()

	fsys, err := archives.FileSystem(context.Background(), compressedFileName, nil)

	if err != nil {
		return nil, err
	}

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		extension := filepath.Ext(path)
		// #TODO move this list of extensions to config file
		allowed := []string{
			".png",
			".jpg",
			".jpeg",
			".webp",
		}

		if !slices.Contains(allowed, extension) {
			return nil
		}

		files = append(files, path)
		return nil
	})

	natsort.Sort(files)
	// sort.Strings(files)

	f.Close()

	if len(files) > 0 {
		f, err = os.Create(fname)
		if err == nil {
			defer f.Close()

			filesJoin := strings.Join(files, "\n")
			n, err := io.WriteString(f, filesJoin)
			if err == nil {
				debugPrintf("File list cache write (%d bytes): %s\n", n, fname)
			}
		}
	} else {
		debugPrintf("Not writing empty file list cache: %s\n", fname)
	}

	return files, nil
}

func (t *Thing) getFileReader(file string) (io.Reader, MultiCloser, error) {
	var closers MultiCloser

	if len(file) > 0 && file[len(file)-1] != '/' {
		compressedFileName := path.Clean(path.Join(t.File.RealLocation()))

		fsys, err := archives.FileSystem(context.Background(), compressedFileName, nil)
		if err != nil {
			return nil, closers, err
		}

		ret, err := fsys.Open(file)

		if err != nil {
			return nil, closers, fmt.Errorf("couldn't read file %s from %s", file, compressedFileName)
		}
		closers = append(closers, ret)

		return ret, closers, nil
	}

	return nil, closers, fmt.Errorf("invalid file: %s", file)
}

func (t *Thing) TrySaveStatic() error {
	var err error
	metaFilePath := t.File.StaticMetaPath()
	_, err = os.Stat(metaFilePath)
	if os.IsNotExist(err) {
		os.MkdirAll(path.Dir(metaFilePath), 0755)
	}

	f, err := os.OpenFile(metaFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	e := json.NewEncoder(f)
	e.SetIndent("", "\t")

	err = e.Encode(t.FileMetadataStatic)
	if err != nil {
		return err
	}

	return nil
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
