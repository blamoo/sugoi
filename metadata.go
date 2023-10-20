package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

type FileMetadataStatic struct {
	Id              int               `json:"id" schema:"id"`
	Collection      string            `json:"collection" schema:"collection"`
	Title           string            `json:"title" schema:"title"`
	Type            string            `json:"type" schema:"type"`
	Tags            []string          `json:"tags" schema:"tags"`
	Language        string            `json:"language" schema:"language"`
	Artist          string            `json:"artist" schema:"artist"`
	CreatedAt       time.Time         `json:"created_at" schema:"created_at"`
	Parody          string            `json:"parody" schema:"parody"`
	Magazine        string            `json:"magazine" schema:"magazine"`
	Publisher       string            `json:"publisher" schema:"publisher"`
	Description     string            `json:"description" schema:"description"`
	Pages           int               `json:"pages" schema:"pages"`
	Thumbnail       int               `json:"thumbnail" schema:"thumbnail"`
	MetadataSources map[string]string `json:"metadataSources" schema:"metadataSources"`
}

type FileMetadataDynamic struct {
	Cover     string    `json:"cover"`
	UpdatedAt time.Time `json:"updated_at"`
	Rating    int       `json:"rating"`
	Marks     int       `json:"marks"`
}

type FileMetadata struct {
	FileMetadataStatic
	FileMetadataDynamic
}

func NewFileMetadataStaticFromFile(file string) (*FileMetadataStatic, error) {
	var err error
	var stat fs.FileInfo
	var mode fs.FileMode

	stat, err = os.Stat(file)
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

		case "artist":
			ret.Artist = form.Get(key)

		case "magazine":
			ret.Magazine = form.Get(key)

		case "publisher":
			ret.Publisher = form.Get(key)

		case "collection":
			ret.Collection = form.Get(key)

		case "parody":
			ret.Parody = form.Get(key)

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
			tags := strings.Split(form.Get(key), "\n")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)

				if len(tag) == 0 {
					continue
				}

				ret.Tags = append(ret.Tags, tag)
			}

		case "metadataText":
			tags := strings.Split(form.Get(key), "\n")
			ret.MetadataSources = make(map[string]string)

			for _, kv := range tags {
				kv = strings.TrimSpace(kv)

				if len(kv) == 0 {
					continue
				}

				split := strings.SplitN(kv, ":", 2)
				if len(split) != 2 {
					return ret, fmt.Errorf("Invalid metadata %s (expected format: 'key:value')", kv)
				}

				split[0] = strings.TrimSpace(split[0])
				split[1] = strings.TrimSpace(split[1])

				ret.MetadataSources[split[0]] = split[1]
			}
		}
	}

	return ret, nil
}

func NewFileMetadataDynamicFromFile(file string) (*FileMetadataDynamic, error) {
	var err error
	var stat fs.FileInfo
	var mode fs.FileMode

	stat, err = os.Stat(file)
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

func (this *FileMetadataStatic) FillEmptyFields(file *FilePointer) {
	if file == nil {
		return
	}

	if len(this.Title) == 0 {
		this.Title = file.PathKey
	}

	if len(this.Collection) == 0 {
		this.Collection = fmt.Sprintf("No Collection (%s)", file.DirHash())
	}

	if this.Pages == 0 {
		p, ok := filePointers.ByHash[file.Hash]

		if ok {
			t := Thing{File: p}
			f, err := t.ListFilesRaw()
			if err == nil {
				log.Printf("Dynamic page count for %s\n", file.Key)
				this.Pages = len(f)
			}
		}
	}
}

func (this *FileMetadataDynamic) FillEmptyFields(file *FilePointer) {
	// if len(this.Cover) == 0 {
	// 	this.Cover = config.DefaultCoverFileName
	// }
}
