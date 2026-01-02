package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	fmt.Println("--- BUILDING OPTIMIZED SITE ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	site := SiteData{
		Pages: make(map[string]PageData),
		Menu:  []*MenuItem{},
	}
	var xmlUrls []string

	err := filepath.WalkDir(InputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Calculate Slugs
		relPath, _ := filepath.Rel(InputDir, path)
		relPath = filepath.ToSlash(relPath)
		filename := strings.TrimSuffix(filepath.Base(path), ".md")
		dir := filepath.Dir(relPath)
		if dir == "." {
			dir = ""
		}

		var slug string
		if dir == "" && filename == "index" {
			slug = "/"
		} else if filename == "index" {
			slug = "/" + dir
		} else {
			slug = "/" + filepath.ToSlash(filepath.Join(dir, filename))
		}

		// Read & Process Content
		source, _ := os.ReadFile(path)
		result, err := ProcessMarkdown(source)
		if err != nil {
			return fmt.Errorf("failed to process %s: %w", path, err)
		}

		// Helper to safely get metadata
		getString := func(key string) string {
			if val, ok := result.Meta[key]; ok {
				return fmt.Sprintf("%v", val)
			}
			return ""
		}
		getInt := func(key string) int {
			if val, ok := result.Meta[key]; ok {
				if i, ok := val.(int); ok {
					return i
				}
				if f, ok := val.(float64); ok {
					return int(f)
				}
			}
			return 0
		}

		published := getString("published on")
		updated := getString("updated on")
		category := getString("category")
		title := getString("title")
		weight := getInt("weight")

		if title == "" {
			title = strings.Title(strings.ReplaceAll(filename, "-", " "))
			if slug == "/" {
				title = "Home"
			}
		}

		// Build Site Data
		site.Pages[slug] = PageData{
			Title:       title,
			Content:     result.HTML,
			TOC:         result.TOC,
			Published:   published,
			Updated:     updated,
			Category:    category,
			Description: result.Description,
			Weight:      weight,
		}

		parts := strings.Split(strings.TrimSuffix(relPath, ".md"), "/")
		site.Menu = addMenuItem(site.Menu, parts, slug, title, weight)
		xmlUrls = append(xmlUrls, slug)
		return nil
	})

	if err != nil {
		fmt.Println("Error walking directory:", err)
		return
	}

	// Output Generation
	if err := GenerateXMLSitemap(xmlUrls); err != nil {
		fmt.Println("Error generating sitemap:", err)
	}

	jsonBytes, _ := json.Marshal(site)
	if err := os.WriteFile(filepath.Join(OutputDir, "db.json"), jsonBytes, 0644); err != nil {
		fmt.Println("Error writing db.json:", err)
	}

	if err := WriteAppShell(filepath.Join(OutputDir, "index.html")); err != nil {
		fmt.Println("Error writing index.html:", err)
	}

	fmt.Println("--- DONE ---")
}

// Logic for building the nested menu structure
func addMenuItem(nodes []*MenuItem, parts []string, slug, finalTitle string, weight int) []*MenuItem {
	if len(parts) == 0 {
		return nodes
	}
	currentPart := parts[0]
	isLast := len(parts) == 1
	var foundNode *MenuItem

	for _, node := range nodes {
		if node.Title == strings.Title(strings.ReplaceAll(currentPart, "-", " ")) && node.IsFolder == !isLast {
			foundNode = node
			break
		}
	}

	if foundNode == nil {
		title := strings.Title(strings.ReplaceAll(currentPart, "-", " "))
		if isLast {
			title = finalTitle
		}

		newNode := &MenuItem{Title: title, IsFolder: !isLast, Children: []*MenuItem{}}
		if isLast {
			newNode.Slug = slug
			newNode.Weight = weight
		} else {
			newNode.Weight = 0
		}

		nodes = append(nodes, newNode)
		foundNode = newNode

		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Slug == "/" {
				return true
			}
			if nodes[j].Slug == "/" {
				return false
			}

			if nodes[i].Weight != nodes[j].Weight {
				return nodes[i].Weight < nodes[j].Weight
			}
			if nodes[i].IsFolder != nodes[j].IsFolder {
				return nodes[i].IsFolder // Folders first
			}
			return nodes[i].Title < nodes[j].Title
		})
	}

	if !isLast {
		foundNode.Children = addMenuItem(foundNode.Children, parts[1:], slug, finalTitle, weight)
	}
	return nodes
}