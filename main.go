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

const (
	InputDir  = "./content"
	OutputDir = "./public"
)

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

	// 3. Scan Pages
	files, _ := os.ReadDir(InputDir)
	var links []Link

	// Build Navigation
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".md" {
			name := strings.TrimSuffix(file.Name(), ".md")
			title := strings.Title(strings.ReplaceAll(name, "-", " "))
			if name == "index" { title = "Home" }
			links = append(links, Link{Title: title, Url: name + ".html"})
		}
	}

	// 4. Render Pages
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

// 5. HTML Template with TAILWIND CSS
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
        tailwind.config = {
            theme: {
                extend: {
                    colors: {
                        dracula: '#282a36',
                        sidebar: '#f3f4f6',
                    }
                }
            }
        }
    </script>
    
    <style>
        /* Small override for the code blocks to look perfect with Goldmark */
        pre { border-radius: 0.5rem; }
    </style>
</head>
<body class="bg-white text-gray-900 h-screen flex flex-col md:flex-row overflow-hidden">

    <aside class="w-full md:w-64 bg-sidebar border-r border-gray-200 flex flex-col flex-shrink-0">
        <div class="p-6 border-b border-gray-200">
            <h1 class="font-bold text-xl tracking-tight text-gray-800">My Docs</h1>
        </div>
        <nav class="flex-1 overflow-y-auto p-4 space-y-1">
            {{ range .AllPages }}
            <a href="{{ .Url }}" 
               class="block px-4 py-2 rounded-md text-sm font-medium transition-colors 
                      {{ if eq .Url $.Link }} bg-blue-600 text-white shadow-sm {{ else }} text-gray-700 hover:bg-gray-200 {{ end }}">
               {{ .Title }}
            </a>
            {{ end }}
        </nav>
    </aside>

    <main class="flex-1 overflow-y-auto bg-white">
        <div class="max-w-4xl mx-auto px-8 py-12">
            
            <h1 class="text-4xl font-extrabold text-gray-900 mb-8 border-b pb-4">{{ .Title }}</h1>
            
            <article class="prose prose-lg prose-slate max-w-none prose-pre:bg-[#282a36] prose-pre:p-0">
                {{ .Content }}
            </article>

        </div>
    </main>

</body>
</html>`

	t, _ := template.New("page").Parse(tpl)
	f, _ := os.Create(path)
	defer f.Close()
	t.Execute(f, p)
}