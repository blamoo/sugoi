package main

import (
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type HentagV1VaultSearchRequest struct {
	Title    string `json:"title,omitempty"`
	Language string `json:"language,omitempty"`
}

type HentagV1VaultSearchResponse []HentagV1Work
type HentagV1Work struct {
	Title         string   `json:"title"`
	CoverImageURL string   `json:"coverImageUrl"`
	Parodies      []string `json:"parodies,omitempty"`
	Circles       []string `json:"circles,omitempty"`
	Artists       []string `json:"artists,omitempty"`
	MaleTags      []string `json:"maleTags,omitempty"`
	FemaleTags    []string `json:"femaleTags,omitempty"`
	OtherTags     []string `json:"otherTags,omitempty"`
	Language      string   `json:"language"`
	Category      string   `json:"category"`
	PublishedOn   int64    `json:"publishedOn,omitempty"`
	CreatedAt     int64    `json:"createdAt"`
	LastModified  int64    `json:"lastModified"`
	Locations     []string `json:"locations,omitempty"`
	Characters    []string `json:"characters,omitempty"`
}

func (v HentagV1Work) ToTags() map[string][]string {
	ret := make(map[string][]string, 0)

	ret["Parodies"] = append(ret["Parodies"], v.Parodies...)
	ret["Circles"] = append(ret["Circles"], v.Circles...)
	ret["Artists"] = append(ret["Artists"], v.Artists...)
	ret["Characters"] = append(ret["Characters"], v.Characters...)
	ret["Male Tags"] = append(ret["Male Tags"], v.MaleTags...)
	ret["Female Tags"] = append(ret["Female Tags"], v.FemaleTags...)
	ret["Other Tags"] = append(ret["Other Tags"], v.OtherTags...)

	return ret
}

func (w HentagV1Work) FillMetadata(ret *FileMetadataStatic) {
	caser := cases.Title(language.English)

	ret.Title = w.Title

	ret.Parody = []string{}
	for _, tag := range w.Parodies {
		ret.Parody = append(ret.Parody, caser.String(tag))
	}

	ret.Artist = []string{}
	for _, tag := range w.Artists {
		ret.Artist = append(ret.Artist, caser.String(tag))
	}

	ret.Tags = []string{}
	for _, tag := range w.Characters {
		ret.Tags = append(ret.Tags, caser.String(tag))
	}
	for _, tag := range w.MaleTags {
		ret.Tags = append(ret.Tags, caser.String(tag))
	}
	for _, tag := range w.FemaleTags {
		ret.Tags = append(ret.Tags, caser.String(tag))
	}
	for _, tag := range w.OtherTags {
		ret.Tags = append(ret.Tags, caser.String(tag))
	}

	ret.Circle = []string{}
	for _, tag := range w.Circles {
		ret.Circle = append(ret.Circle, caser.String(tag))
	}

	if w.PublishedOn > 0 {
		ret.CreatedAt = time.UnixMilli(w.PublishedOn)
	} else if w.CreatedAt > 0 {
		ret.CreatedAt = time.UnixMilli(w.CreatedAt)
	}

	for _, location := range w.Locations {
		if strings.HasPrefix(location, "https://hentag.com/") {
			ret.Urls = append(ret.Urls, location)
		}
	}
}

var HentagSearchLanguages = map[int]string{
	1:  "English",
	2:  "Japanese",
	3:  "Spanish",
	4:  "Turkish",
	5:  "Persian",
	6:  "French",
	7:  "German",
	8:  "Russian",
	9:  "Portuguese",
	10: "Vietnamese",
	11: "Chinese",
	12: "Arabic",
	13: "Italian",
	14: "Polish",
	15: "Greek",
	16: "Indonesian",
	17: "Dutch",
	18: "Korean",
	19: "Thai",
	20: "Czech",
	21: "Ukrainian",
	22: "Hebrew",
	23: "Swedish",
	24: "Romanian",
	25: "Hungarian",
	26: "Danish",
	27: "Serbian",
	28: "Slovak",
	29: "Bulgarian",
	30: "Finnish",
	31: "Croatian",
	32: "Lithuanian",
	33: "Norwegian",
	34: "Hindi",
	35: "Slovenian",
	36: "Latvian",
	37: "Estonian",
	38: "Filipino",
	-1: "Unknown",
}
