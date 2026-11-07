// Package library knows which archives exist (db/files.txt), where each one
// lives on disk and where its metadata goes, and how to read its pages.
package library

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/blamoo/sugoi/internal/config"
)

// File is one archive of the library. Its identity is the sha1 of the raw
// line in files.txt, used as the id in every URL.
type File struct {
	Key      string
	Hash     string
	PathKey  string
	MetaPath string

	realPath string
	mu       sync.Mutex
}

// Library is an immutable snapshot of files.txt. A reload builds a new one.
type Library struct {
	List   []*File
	ByHash map[string]*File

	cacheDir string
}

// Load reads <DatabaseDir>/files.txt: one archive per line, blank lines and
// lines starting with #, // or ; ignored. Paths may use {{VAR}} placeholders
// from the config's DirVars.
func Load(cfg *config.Config) (*Library, error) {
	f, err := os.Open(path.Join(cfg.DatabaseDir, "files.txt"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var keys []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, ";") {
			continue
		}
		keys = append(keys, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return New(cfg, keys), nil
}

// New builds a library from files.txt lines.
func New(cfg *config.Config, keys []string) *Library {
	lib := &Library{
		List:     make([]*File, 0, len(keys)),
		ByHash:   make(map[string]*File, len(keys)),
		cacheDir: cfg.CacheDir,
	}

	for _, key := range keys {
		f := NewFile(cfg, key)
		lib.List = append(lib.List, f)
		lib.ByHash[f.Hash] = f
	}

	return lib
}

// Find returns the file with the given hash.
func (lib *Library) Find(hash string) (*File, error) {
	f, ok := lib.ByHash[hash]
	if !ok {
		return nil, fmt.Errorf("file %s not found", hash)
	}
	return f, nil
}

var dirVarRe = regexp.MustCompile(`{{.*?}}`)

func NewFile(cfg *config.Config, key string) *File {
	f := &File{Key: key}
	f.Hash = fmt.Sprintf("%x", sha1.Sum([]byte(key)))
	f.PathKey = buildPathKey(key)
	f.MetaPath = path.Join(cfg.DatabaseDir, "meta", f.PathKey)

	real := key
	for name, val := range cfg.DirVars {
		real = strings.ReplaceAll(real, "{{"+name+"}}", val)
	}
	f.realPath = path.Clean(real)

	return f
}

// buildPathKey turns "{{LIB}}/a/b.cbz" into "lib/a/b.cbz", the metadata
// directory name.
func buildPathKey(key string) string {
	p := dirVarRe.ReplaceAllStringFunc(key, func(s string) string {
		s = strings.TrimPrefix(s, "{{")
		s = strings.TrimSuffix(s, "}}")
		return strings.ToLower(s)
	})
	return path.Clean(p)
}

// Lock guards read-modify-write of this file's metadata.
func (f *File) Lock()   { f.mu.Lock() }
func (f *File) Unlock() { f.mu.Unlock() }

// RealLocation is the archive path with DirVars expanded.
func (f *File) RealLocation() string { return f.realPath }

func (f *File) StaticMetaPath() string  { return path.Join(f.MetaPath, "static.json") }
func (f *File) DynamicMetaPath() string { return path.Join(f.MetaPath, "dynamic.json") }

// PlaceholderCollection is the parent directory name.
func (f *File) PlaceholderCollection() string {
	return path.Base(path.Dir(f.PathKey))
}

var fixName = regexp.MustCompile(`^(\[.*?\] )?(.*?)( \(.*?\))?$`)

// PlaceholderTitle is the file name without extension, "[Artist] " prefix and
// " (Magazine)" suffix.
func (f *File) PlaceholderTitle() string {
	name := path.Base(f.PathKey)
	name = strings.TrimSuffix(name, path.Ext(name))
	return fixName.ReplaceAllString(name, "$2")
}
