// Package config loads and saves sugoi's JSON configuration file.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/blamoo/sugoi/internal/fsutil"
)

type Config struct {
	Debug               bool
	CacheThumbnails     bool
	CacheDir            string
	DatabaseDir         string
	ServerHost          string
	ServerPort          int
	MaxConcurrency      int
	DirVars             map[string]string
	SessionCookieName   string
	SessionCookieMaxAge int
	SessionCookieKey    []byte
	Users               map[string]string
	QuickFilters        []QuickFilter
	SlideshowPresets    []SlideshowPreset
}

// QuickFilter is a badge on the home page that adds Add to the search and
// first removes whatever matches the Remove regexp.
type QuickFilter struct {
	Label  string
	Add    string
	Remove string
}

// SessionKeyError is returned when SessionCookieKey is missing or too short.
// Suggested is a freshly generated valid value.
type SessionKeyError struct {
	Suggested string
}

func (e SessionKeyError) Error() string {
	return "SessionCookieKey should be a base64 encoded secret byte array with at least 32 bytes"
}

// Load reads and validates the configuration file.
func Load(file string) (*Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var c Config
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}

	if c.MaxConcurrency <= 0 {
		return nil, fmt.Errorf("MaxConcurrency should be greater than 0")
	}

	if c.SessionCookieMaxAge <= 0 {
		return nil, fmt.Errorf("SessionCookieMaxAge should be greater than 0")
	}

	if len(c.SlideshowPresets) == 0 {
		c.SlideshowPresets = []SlideshowPreset{
			{Seconds: 5, Default: true},
			{Seconds: 10},
			{Seconds: 30},
		}
	}

	if len(c.SessionCookieKey) < 32 {
		key := make([]byte, 32)
		rand.Read(key)
		return &c, SessionKeyError{Suggested: base64.StdEncoding.EncodeToString(key)}
	}

	return &c, nil
}

func (c Config) DefaultSlideshowPreset() SlideshowPreset {
	for _, v := range c.SlideshowPresets {
		if v.Default {
			return v
		}
	}

	return c.SlideshowPresets[0]
}

// CacheFile returns a path inside CacheDir.
func (c Config) CacheFile(elem ...string) string {
	params := []string{c.CacheDir}
	params = append(params, elem...)
	return path.Join(params...)
}

func (c Config) Save(to string) error {
	b, err := c.Export()
	if err != nil {
		return err
	}

	// 0600: the file holds the session key and password hashes.
	return fsutil.WriteFileAtomic(to, 0600, true, func(w io.Writer) error {
		_, err := io.WriteString(w, b)
		return err
	})
}

func (c Config) Export() (string, error) {
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return "", err
	}

	return string(b), nil
}
