package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

type FilePointerList struct {
	List      []*FilePointer
	ByKey     map[string]*FilePointer
	ByPathKey map[string]*FilePointer
	ByHash    map[string]*FilePointer
}

func NewFilePointerList() FilePointerList {
	return FilePointerList{
		List:      make([]*FilePointer, 0),
		ByKey:     make(map[string]*FilePointer),
		ByPathKey: make(map[string]*FilePointer),
		ByHash:    make(map[string]*FilePointer),
	}
}

func (fpl *FilePointerList) Clear() {
	fpl.List = make([]*FilePointer, 0)
	fpl.ByKey = make(map[string]*FilePointer)
	fpl.ByPathKey = make(map[string]*FilePointer)
	fpl.ByHash = make(map[string]*FilePointer)
}

func (fpl *FilePointerList) Push(n *FilePointer) {
	fpl.List = append(fpl.List, n)
	fpl.ByKey[n.Key] = n
	fpl.ByPathKey[n.PathKey] = n
	fpl.ByHash[n.Hash] = n
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type FilePointer struct {
	Key      string
	Hash     string
	PathKey  string
	MetaPath string
}

func NewFilePointer(key string) (*FilePointer, error) {
	ret := FilePointer{}

	byteKey := []byte(key)
	byteHash := sha1.Sum(byteKey)

	ret.Key = key
	ret.Hash = fmt.Sprintf("%x", byteHash)
	ret.PathKey = ret.BuildPathKey()
	ret.MetaPath = path.Join(config.DatabaseDir, "meta", ret.PathKey)

	return &ret, nil
}

func (fp *FilePointer) BuildPathKey() string {
	p := fp.Key

	var re = regexp.MustCompile(`{{.*?}}`)

	p = re.ReplaceAllStringFunc(p, func(s string) string {
		s = strings.Replace(s, "{{", "", -1)
		s = strings.Replace(s, "}}", "", -1)
		return strings.ToLower(s)
	})

	return path.Clean(p)
}

func (fp *FilePointer) RealLocation() string {
	p := fp.Key
	for key, val := range config.DirVars {
		p = strings.ReplaceAll(p, fmt.Sprintf("{{%s}}", key), val)
	}
	return path.Clean(p)
}

func (fp *FilePointer) StaticMetaPath() string {
	return path.Join(fp.MetaPath, "static.json")
}

func (fp *FilePointer) DynamicMetaPath() string {
	return path.Join(fp.MetaPath, "dynamic.json")
}

func (fp *FilePointer) PlaceholderCollection() string {
	dir := path.Dir(fp.PathKey)
	return path.Base(dir)
}

var fixName = regexp.MustCompile(`^(\[.*?\] )?(.*?)( \(.*?\))?$`)

func (fp *FilePointer) PlaceholderTitle() string {
	name := path.Base(fp.PathKey)
	ext := path.Ext(name)
	name, _ = strings.CutSuffix(name, ext)
	name = fixName.ReplaceAllString(name, "$2")
	return name
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func InitializeFilePointers() error {
	file, err := os.Open(path.Join(config.DatabaseDir, "files.txt"))

	if err != nil {
		return err
	}

	r := bufio.NewReader(file)

	filePointers = NewFilePointerList()

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)

		if err == io.EOF && len(line) == 0 {
			return nil
		}

		if err != nil && err != io.EOF {
			filePointers.Clear()
			return err
		}

		if len(line) == 0 {
			continue
		}

		n, err := NewFilePointer(line)

		if err != nil {
			filePointers.Clear()
			return err
		}

		filePointers.Push(n)
	}
}

func (fp *FilePointer) ReindexIntoBatch(idx *bleve.Batch) error {
	doc := fp.BuildReindexDoc()

	err := idx.Index(fp.Hash, doc)
	if err != nil {
		return err
	}

	return nil
}

func (fp *FilePointer) Reindex() error {
	doc := fp.BuildReindexDoc()

	err := bleveIndex.Index(fp.Hash, doc)
	if err != nil {
		return err
	}

	return nil
}

func (fp *FilePointer) BuildReindexDoc() FileMetadata {
	var err error
	var file FileMetadata
	static, err := NewFileMetadataStaticFromFile(fp.StaticMetaPath())
	if err != nil {
		file.FileMetadataStatic = FileMetadataStatic{}
	} else {
		file.FileMetadataStatic = *static
	}
	file.FileMetadataStatic.FillEmptyFields(fp)

	dynamic, err := NewFileMetadataDynamicFromFile(fp.DynamicMetaPath())
	if err != nil {
		file.FileMetadataDynamic = FileMetadataDynamic{}
	} else {
		file.FileMetadataDynamic = *dynamic
	}
	file.FileMetadataDynamic.FillEmptyFields(fp)

	file.Random = make(map[string]int, RANDOM_POOL_SIZE)
	for i := 0; i < RANDOM_POOL_SIZE; i++ {
		k := strconv.Itoa(i)
		file.Random[k] = rand.Int()
	}

	file.CollectionKw = file.Collection
	file.TagsKw = file.Tags
	file.ArtistKw = file.Artist
	file.ParodyKw = file.Parody
	file.MagazineKw = file.Magazine

	return file
}
