package main

import "math"

type ReaderData struct {
	Hash          string   `json:"hash"`
	Pages         []string `json:"pages"`
	Prefix        string   `json:"prefix"`
	BackUrl       string   `json:"backUrl"`
	Title         string   `json:"title"`
	Page          int      `json:"page"`
	ReadThreshold int      `json:"readThreshold"`
	Marks         int      `json:"marks"`
	SearchTags    []string `json:"searchTags"`
}

func NewReaderData(thing *Thing, page int) (ReaderData, error) {
	files, err := thing.ListFiles()
	if err != nil {
		return ReaderData{}, err
	}

	searchTags := []string{}
	for _, tag := range thing.SearchTags() {
		searchTags = append(searchTags, string(tag.Badge()))
	}

	return ReaderData{
		Hash:          thing.Hash(),
		Pages:         files,
		Prefix:        thing.ReadUrl(),
		BackUrl:       thing.DetailsUrl(),
		Title:         thing.Title,
		Page:          page,
		ReadThreshold: min(24, int(math.Ceil(float64(len(files))/3.0))),
		Marks:         thing.Marks,
		SearchTags:    searchTags,
	}, nil
}
