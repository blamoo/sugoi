package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"time"
)

type FileMetadataStatic struct {
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

type FileMetadataStaticSub struct {
	Id map[string]string `json:"id" schema:"id"`
}

type FileMetadataDynamic struct {
	Cover       string    `json:"cover"`
	UpdatedAt   time.Time `json:"updated_at"`
	Rating      int       `json:"rating"`
	Marks       int       `json:"marks"`
	ReadCount   int       `json:"read_count"`
	FirstReadAt time.Time `json:"first_read_at,omitempty"`
	LastReadAt  time.Time `json:"last_read_at,omitempty"`
}

var metadataKeywordsFields = []string{
	"tags",
	"artist",
	"parody",
	"magazine",
	"collection",
}

type KeywordsMetadata struct {
	CollectionKw string      `json:"collection_kw" schema:"collection_kw"`
	TagsKw       StringArray `json:"tags_kw" schema:"tags_kw"`
	ArtistKw     StringArray `json:"artist_kw" schema:"artist_kw"`
	ParodyKw     StringArray `json:"parody_kw" schema:"parody_kw"`
	MagazineKw   StringArray `json:"magazine_kw" schema:"magazine_kw"`
}

type FileMetadata struct {
	FileMetadataStatic
	FileMetadataDynamic
	KeywordsMetadata
	Random map[string]int `json:"random,omitempty"`
}

func CreateMetadataEmptyFile(file string) error {
	var err error

	dir := path.Dir(file)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("{}")
	if err != nil {
		return err
	}

	log.Printf("Created empty metadata file %s\n", file)
	return nil
}

func NewFileMetadataStaticFromFile(file string) (*FileMetadataStatic, error) {
	var err error
	var stat fs.FileInfo
	var mode fs.FileMode

	stat, err = os.Stat(file)

	if errors.Is(err, os.ErrNotExist) {
		err = CreateMetadataEmptyFile(file)
		if err != nil {
			return nil, err
		}

		return &FileMetadataStatic{}, nil
	}

	if err != nil {
		return nil, err
	}

	mode = stat.Mode()
	if !mode.IsRegular() {
		return nil, fmt.Errorf("'%s' is not a file", file)
	}

	reader, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var ret FileMetadataStatic

	err = json.Unmarshal(bytes, &ret)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func NewFileMetadataStaticFromForm(form url.Values) (FileMetadataStatic, error) {
	var ret FileMetadataStatic

	for key := range form {
		switch key {
		case "language":
			ret.Language = form.Get(key)

		case "artistText":
			ret.Artist.SetFromTextArea(form.Get(key))

		case "magazineText":
			ret.Magazine.SetFromTextArea(form.Get(key))

		case "publisher":
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
				return ret, err
			}

		case "tagsText":
			ret.Tags.SetFromTextArea(form.Get(key))

		case "urlsText":
			ret.Urls.SetFromTextArea(form.Get(key))

		case "seriesText":
			ret.Series.SetFromTextArea(form.Get(key))
		}
	}

	return ret, nil
}

func NewFileMetadataDynamicFromFile(file string) (*FileMetadataDynamic, error) {
	var err error
	var stat fs.FileInfo
	var mode fs.FileMode

	stat, err = os.Stat(file)

	if errors.Is(err, os.ErrNotExist) {
		err = CreateMetadataEmptyFile(file)
		if err != nil {
			return nil, err
		}

		return &FileMetadataDynamic{}, nil
	}

	if err != nil {
		return nil, err
	}

	mode = stat.Mode()
	if !mode.IsRegular() {
		return nil, fmt.Errorf("'%s' is not a file", file)
	}

	reader, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var ret FileMetadataDynamic
	err = json.Unmarshal(bytes, &ret)
	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (fms *FileMetadataStatic) FillEmptyFields(file *FilePointer) {
	if file == nil {
		return
	}

	if len(fms.Title) == 0 {
		fms.Title = file.PlaceholderTitle()
	}

	if len(fms.Collection) == 0 {
		fms.Collection = file.PlaceholderCollection()
	}

	if fms.Pages == 0 {
		p, ok := filePointers.ByHash[file.Hash]

		if ok {
			t := Thing{File: p}
			f, err := t.ListFilesRaw()
			if err == nil {
				// log.Printf("Dynamic page count for %s\n", file.Key)
				fms.Pages = len(f)
			}
		}
	}
}

func (fms *FileMetadataDynamic) FillEmptyFields(file *FilePointer) {
	// if len(this.Cover) == 0 {
	// 	this.Cover = config.DefaultCoverFileName
	// }
}
