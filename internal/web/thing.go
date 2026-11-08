package web

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/blamoo/sugoi/internal/search"

	"github.com/blamoo/sugoi/internal/catalog"
)

// Thing adds the HTML/URL helpers used by the templates to a catalog item.
type Thing struct {
	*catalog.Thing

	srv *Server
}

func (srv *Server) thing(hash string) (*Thing, error) {
	t, err := srv.cat.Get(hash)
	if err != nil {
		return nil, err
	}
	return &Thing{Thing: t, srv: srv}, nil
}

// TrySaveCover sets the cover from a page URL (as listed by ListFiles).
func (t *Thing) TrySaveCover(file string, isUpdate bool) error {
	newCover, _ := filepath.Rel(t.FileUrlPrefix(), file)

	files, err := t.ListFiles()
	if err != nil {
		return err
	}

	if !slices.Contains(files, file) {
		return fmt.Errorf("file %s doesn't exists in %s", newCover, t.File.RealLocation())
	}

	return t.SetCover(newCover, isUpdate)
}

func (t *Thing) CoverImageUrl() string {
	if len(t.Cover) > 0 {
		return t.Cover
	}

	f, err := t.PageFiles()
	if err != nil || len(f) == 0 {
		return "/static/empty.jpg"
	}

	if t.Thumbnail < 0 || t.Thumbnail >= len(f) {
		return "/static/empty.jpg"
	}

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

	tmpl, err := t.srv.currentTemplates()
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
	q.Set("q", search.Term(search.KeywordField("collection"), t.Collection))
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
	raw, err := t.PageFiles()
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(raw))
	for key, val := range raw {
		ret[key] = t.FileUrl(val)
	}
	return ret, nil
}

func (t *Thing) SeriesUrls() map[string]string {
	ret := make(map[string]string)

	for _, name := range t.Series {
		u := new(url.URL)
		u.Path = "/"
		q := u.Query()
		q.Set("q", search.Term("series", name))
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
