package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 bytes

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "sugoi.json")
	if err := os.WriteFile(name, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestLoadDefaultsAndSaveRoundTrip(t *testing.T) {
	name := writeConfig(t, `{"MaxConcurrency":2,"SessionCookieMaxAge":60,"SessionCookieKey":"`+validKey+`",
		"QuickFilters":[{"Label":"r5","Add":"+rating:5","Remove":"rating:\\d"}]}`)

	c, err := Load(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SlideshowPresets) != 3 || c.DefaultSlideshowPreset().Seconds != 5 {
		t.Errorf("default slideshow presets not applied: %+v", c.SlideshowPresets)
	}
	if c.QuickFilters[0].Add != "+rating:5" {
		t.Errorf("quick filters = %+v", c.QuickFilters)
	}

	out := filepath.Join(t.TempDir(), "saved.json")
	if err := c.Save(out); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(out)
	if info.Mode().Perm() != 0600 {
		t.Errorf("saved with %v, want 0600", info.Mode().Perm())
	}
	again, err := Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(again.SessionCookieKey) != string(c.SessionCookieKey) {
		t.Error("session key changed on save")
	}
}

func TestLoadValidation(t *testing.T) {
	cases := map[string]string{
		"MaxConcurrency":      `{"MaxConcurrency":0,"SessionCookieMaxAge":60,"SessionCookieKey":"` + validKey + `"}`,
		"SessionCookieMaxAge": `{"MaxConcurrency":1,"SessionCookieMaxAge":0,"SessionCookieKey":"` + validKey + `"}`,
	}
	for field, body := range cases {
		_, err := Load(writeConfig(t, body))
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s: err = %v", field, err)
		}
	}
}

func TestLoadSuggestsSessionKey(t *testing.T) {
	_, err := Load(writeConfig(t, `{"MaxConcurrency":1,"SessionCookieMaxAge":60}`))

	var keyErr SessionKeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("err = %v, want SessionKeyError", err)
	}
	if len(keyErr.Suggested) < 44 {
		t.Errorf("suggested key %q is too short", keyErr.Suggested)
	}
}
