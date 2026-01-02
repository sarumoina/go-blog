package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func GenerateXMLSitemap(slugs []string) error {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	today := time.Now().Format("2006-01-02")
	for _, slug := range slugs {
		fullUrl := BaseURL + "/#" + slug
		if slug == "/" {
			fullUrl = BaseURL + "/"
		}
		buf.WriteString("  <url>\n")
		buf.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", fullUrl))
		buf.WriteString(fmt.Sprintf("    <lastmod>%s</lastmod>\n", today))
		buf.WriteString("    <changefreq>weekly</changefreq>\n")
		buf.WriteString("  </url>\n")
	}
	buf.WriteString(`</urlset>`)
	return os.WriteFile(filepath.Join(OutputDir, "sitemap.xml"), buf.Bytes(), 0644)
}