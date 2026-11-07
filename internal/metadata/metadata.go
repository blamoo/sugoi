// Package metadata holds the per-item metadata stored on disk: static.json
// (descriptive fields, mostly produced by an importer and edited through the
// edit form) and dynamic.json (user state such as rating and read counts).
package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/blamoo/sugoi/internal/fsutil"
)

// Static is the content of static.json.
type Static struct {
	Id          []int       `json:"id" schema:"id"`
	IdSource    IdArray     `json:"id_source" schema:"id_source"`
	Collection  string      `json:"collection" schema:"collection"`
	Title       string      `json:"title" schema:"title"`
	Tags        StringArray `json:"tags" schema:"tags"`
	Language    string      `json:"language" schema:"language"`
	Artist      StringArray `json:"artist" schema:"artist"`
	Circle      StringArray `json:"circle" schema:"circle"`
	Series      StringArray `json:"series" schema:"series"`
	CreatedAt   time.Time   `json:"created_at" schema:"created_at"`
	Parody      StringArray `json:"parody" schema:"parody"`
	Magazine    StringArray `json:"magazine" schema:"magazine"`
	Publisher   StringArray `json:"publisher" schema:"publisher"`
	Description string      `json:"description" schema:"description"`
	Pages       int         `json:"pages" schema:"pages"`
	Thumbnail   int         `json:"thumbnail" schema:"thumbnail"`
	Urls        StringArray `json:"urls" schema:"urls"`
	Files       StringArray `json:"files" schema:"files"`
}

// Dynamic is the content of dynamic.json.
type Dynamic struct {
	Cover       string    `json:"cover"`
	UpdatedAt   time.Time `json:"updated_at"`
	Rating      int       `json:"rating"`
	Marks       int       `json:"marks"`
	ReadCount   int       `json:"read_count"`
	FirstReadAt time.Time `json:"first_read_at"`
	LastReadAt  time.Time `json:"last_read_at"`
}

// ReadStatic reads a static.json. A missing file reads as empty metadata.
func ReadStatic(file string) (*Static, error) {
	var ret Static
	return &ret, readJSON(file, &ret)
}

// ReadDynamic reads a dynamic.json. A missing file reads as empty metadata.
func ReadDynamic(file string) (*Dynamic, error) {
	var ret Dynamic
	return &ret, readJSON(file, &ret)
}

// Write atomically saves static or dynamic metadata.
func Write(file string, v any) error {
	return fsutil.WriteJSON(file, v)
}

// readJSON never creates the file: reads happen outside the item lock, so
// creating it here could clobber a concurrent write, and an empty static.json
// would make the importer (theplan) skip the item.
func readJSON(file string, v any) error {
	stat, err := os.Stat(file)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if !stat.Mode().IsRegular() {
		return fmt.Errorf("'%s' is not a file", file)
	}

	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, v)
}

// ApplyForm overwrites the fields present in the edit form and keeps every
// other field (id, circle, pages, files, ...) as it is.
func (ret *Static) ApplyForm(form url.Values) error {
	for key := range form {
		switch key {
		case "language":
			ret.Language = form.Get(key)

		case "artistText":
			ret.Artist.SetFromTextArea(form.Get(key))

		case "magazineText":
			ret.Magazine.SetFromTextArea(form.Get(key))

		case "publisherText":
			ret.Publisher.SetFromTextArea(form.Get(key))

		case "collection":
			ret.Collection = form.Get(key)

		case "parodyText":
			ret.Parody.SetFromTextArea(form.Get(key))

		case "title":
			ret.Title = form.Get(key)

		case "description":
			ret.Description = form.Get(key)

		case "created_at":
			err := ret.CreatedAt.UnmarshalText([]byte(form.Get(key)))
			if err != nil {
				return err
			}

		case "tagsText":
			ret.Tags.SetFromTextArea(form.Get(key))

		case "urlsText":
			ret.Urls.SetFromTextArea(form.Get(key))

		case "seriesText":
			ret.Series.SetFromTextArea(form.Get(key))
		}
	}

	return nil
}
