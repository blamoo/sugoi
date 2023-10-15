package main

import (
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type HentagVaultSearch struct {
	Page        int                     `json:"page"`
	PageSize    int                     `json:"pageSize"`
	Works       []HentagVaultSearchWork `json:"works"`
	Total       int                     `json:"total"`
	RequestedAt int64                   `json:"requestedAt"`
}

type HentagVaultSearchWork struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Parodies          []Tag    `json:"parodies"`
	Circles           []Tag    `json:"circles"`
	Artists           []Tag    `json:"artists"`
	Characters        []Tag    `json:"characters"`
	MaleTags          []Tag    `json:"maleTags"`
	FemaleTags        []Tag    `json:"femaleTags"`
	OtherTags         []Tag    `json:"otherTags"`
	Language          int      `json:"language"`
	Category          int      `json:"category"`
	Locations         []string `json:"locations"`
	CreatedAt         int64    `json:"createdAt"`
	LastModified      int64    `json:"lastModified"`
	PublishedOn       int64    `json:"publishedOn"`
	CoverImageURL     string   `json:"coverImageUrl"`
	Favorite          bool     `json:"favorite"`
	IsControversial   bool     `json:"isControversial"`
	IsDead            bool     `json:"isDead"`
	IsPendingApproval bool     `json:"isPendingApproval"`
}

func (v HentagVaultSearchWork) ToTags() map[string][]string {
	ret := make(map[string][]string, 0)

	for _, tag := range v.Parodies {
		ret["Parodies"] = append(ret["Parodies"], tag.Name)
	}
	for _, tag := range v.Circles {
		ret["Circles"] = append(ret["Circles"], tag.Name)
	}
	for _, tag := range v.Artists {
		ret["Artists"] = append(ret["Artists"], tag.Name)
	}
	for _, tag := range v.Characters {
		ret["Characters"] = append(ret["Characters"], tag.Name)
	}
	for _, tag := range v.MaleTags {
		ret["Male Tags"] = append(ret["Male Tags"], tag.Name)
	}
	for _, tag := range v.FemaleTags {
		ret["Female Tags"] = append(ret["Female Tags"], tag.Name)
	}
	for _, tag := range v.OtherTags {
		ret["Other Tags"] = append(ret["Other Tags"], tag.Name)
	}

	return ret
}

func (v HentagVaultSearchWork) FillMetadata(ret *FileMetadataStatic) {
	caser := cases.Title(language.English)

	ret.Title = v.Title

	ret.Tags = []string{}
	for _, tag := range v.Parodies {
		ret.Parody = caser.String(tag.Name)
		break
	}
	for _, tag := range v.Artists {
		ret.Artist = caser.String(tag.Name)
		break
	}
	for _, tag := range v.Characters {
		ret.Tags = append(ret.Tags, caser.String(tag.Name))
	}
	for _, tag := range v.MaleTags {
		ret.Tags = append(ret.Tags, caser.String(tag.Name))
	}
	for _, tag := range v.FemaleTags {
		ret.Tags = append(ret.Tags, caser.String(tag.Name))
	}
	for _, tag := range v.OtherTags {
		ret.Tags = append(ret.Tags, caser.String(tag.Name))
	}

	if v.PublishedOn > 0 {
		ret.CreatedAt = time.UnixMilli(v.PublishedOn)
	} else if v.CreatedAt > 0 {
		ret.CreatedAt = time.UnixMilli(v.CreatedAt)
	}

	ret.MetadataSources = map[string]string{
		"Hentag": v.ID,
	}
}
