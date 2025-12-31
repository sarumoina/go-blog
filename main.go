package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Configuration
const (
	InputDir  = "./content"
	OutputDir = "./public"
)

// Data Structures
type Page struct {
	Title    string
	Link     string
	Content  template.HTML
	AllPages []Link
}

type Link struct {
	Title string
	Url   string
}

func main() {
	// 1. Setup Markdown Parser (Dracula Theme for code)
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("dracula"),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)

	// 2. Setup Output Folder
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	// 3. Scan Root Level Pages Only
	files, _ := os.ReadDir(InputDir)
	var links []Link

	// Build Navigation (First pass)
	for _, file := range files {
		// Only process .md files
		if filepath.Ext(file.Name()) == ".md" {
			name := strings.TrimSuffix(file.Name(), ".md")
			
			// Format Title
			title := strings.Title(strings.ReplaceAll(name, "-", " "))
			if name == "index" { title = "Home" }
			
			links = append(links, Link{Title: title, Url: name + ".html"})
		}
	}

	// 4. Render Pages (Second pass)
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".md" { continue }

		srcPath := filepath.Join(InputDir, file.Name())
		source, _ := os.ReadFile(srcPath)

		var buf bytes.Buffer
		md.Convert(source, &buf)

		name := strings.TrimSuffix(file.Name(), ".md")
		title := strings.Title(strings.ReplaceAll(name, "-", " "))
		if name == "index" { title = "Home" }

		pg := Page{
			Title:    title,
			Link:     name + ".html",
			Content:  template.HTML(buf.String()),
			AllPages: links,
		}

		generateHTML(filepath.Join(OutputDir, name+".html"), pg)
		fmt.Println("Generated:", name+".html")
	}
}

// 5. HTML Template with TOP NAVBAR
func generateHTML(path string, p Page) {
	const tpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .Title }}</title>
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>
        tailwind.config = { theme: { extend: { colors: { dracula: '#282a36' } } } }
    </script>
    <style>pre { border-radius: 0.5rem; }</style>
</head>
<body class="bg-gray-50 text-gray-900 min-h-screen flex flex-col">

    <nav class="bg-white border-b border-gray-200 sticky top-0 z-50 shadow-sm">
        <div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex justify-between h-16">
                <div class="flex items-center">
                    <span class="font-bold text-xl tracking-tight text-gray-900 mr-8">My Docs</span>
                    
                    <div class="hidden md:flex space-x-4">
                        {{ range .AllPages }}
                        <a href="{{ .Url }}" 
                           class="px-3 py-2 rounded-md text-sm font-medium transition-colors
                                  {{ if eq .Url $.Link }} bg-blue-50 text-blue-700 {{ else }} text-gray-600 hover:text-gray-900 hover:bg-gray-100 {{ end }}">
                           {{ .Title }}
                        </a>
                        {{ end }}
                    </div>
                </div>
            </div>
        </div>
        
        <div class="md:hidden overflow-x-auto whitespace-nowrap border-t border-gray-100 py-2 px-4 bg-gray-50">
             {{ range .AllPages }}
            <a href="{{ .Url }}" 
               class="inline-block mr-4 px-3 py-1 rounded-full text-sm font-medium 
                      {{ if eq .Url $.Link }} bg-blue-100 text-blue-800 {{ else }} text-gray-600 {{ end }}">
               {{ .Title }}
            </a>
            {{ end }}
        </div>
    </nav>

    <main class="flex-1 w-full max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <article class="prose prose-lg prose-slate max-w-none prose-pre:bg-[#282a36] prose-pre:p-0">
            <h1 class="mb-8 text-3xl font-bold">{{ .Title }}</h1>
            
            {{ .Content }}
        </article>
    </main>

    <footer class="bg-white border-t border-gray-200 mt-12">
        <div class="max-w-5xl mx-auto py-6 px-4 text-center text-gray-400 text-sm">
            &copy; 2025 Generated with Go
        </div>
    </footer>

</body>
</html>`

	t, _ := template.New("page").Parse(tpl)
	f, _ := os.Create(path)
	defer f.Close()
	t.Execute(f, p)
}