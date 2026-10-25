package main

import (
	"html/template"
	"time"
)

type Resource struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

type Project struct {
	ID           string
	Slug         string
	Title        string
	Tagline      string
	Problem      string
	Approach     string
	Impact       string
	ImpactMetric string
	Tools        []string
	RepoURL      string
	DemoURL      string
	ImageURL     string
	Featured     bool
	SortOrder    int
	CreatedAt    time.Time
	BodyMarkdown string
	Resources    []Resource
	DataURL      string
	Kind         string

	BodyHTML template.HTML
}

type Post struct {
	ID           string
	Slug         string
	Title        string
	Excerpt      string
	BodyMarkdown string
	ImageURL     string
	PublishedAt  time.Time

	BodyHTML template.HTML
}

type Testimonial struct {
	ID        string
	Author    string
	Role      string
	Quote     string
	Source    string
	CreatedAt time.Time
}