package main

// Configuration constants
const (
	InputDir  = "./content"
	OutputDir = "./public"
	BaseURL   = "https://mysite.com"
)

// SiteData represents the entire database of the site
type SiteData struct {
	Pages map[string]PageData `json:"pages"`
	Menu  []*MenuItem         `json:"menu"`
}

// PageData represents a single page's content and metadata
type PageData struct {
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	TOC         []TOCEntry `json:"toc"`
	Published   string     `json:"published"`
	Updated     string     `json:"updated"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	Weight      int        `json:"weight"`
}

// MenuItem represents a node in the navigation tree
type MenuItem struct {
	Title    string      `json:"title"`
	Slug     string      `json:"slug"`
	IsFolder bool        `json:"is_folder"`
	Weight   int         `json:"weight"`
	Children []*MenuItem `json:"children,omitempty"`
}

// TOCEntry represents a header in the Table of Contents
type TOCEntry struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	Level int    `json:"level"`
}