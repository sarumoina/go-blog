package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

// Config
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
	TOC      []TOCEntry // <--- New Field
}

type Link struct {
	Title string
	Url   string
}

type TOCEntry struct {
	Title string
	ID    string
	Level int // 1 for H1, 2 for H2, etc.
}

func main() {
	// 1. Setup Markdown Parser
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

	// 2. Prepare Output
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	// 3. Scan Files
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

		// Read File
		srcPath := filepath.Join(InputDir, file.Name())
		source, _ := os.ReadFile(srcPath)

		// --- START AST PARSING (To get TOC) ---
		context := parser.NewContext()
		reader := text.NewReader(source)
		doc := md.Parser().Parse(reader, parser.WithContext(context))
		
		var toc []TOCEntry

		// Walk the tree to find Headings
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering { return ast.WalkContinue, nil }
			
			if heading, ok := n.(*ast.Heading); ok {
				// We found a heading! Extract text and ID
				id := string(heading.AttributeString("id"))
				if id == "" { return ast.WalkContinue, nil } // Skip if no ID

				// Extract text content of the heading
				title := string(heading.Text(source))
				
				toc = append(toc, TOCEntry{
					Title: title,
					ID:    id,
					Level: heading.Level,
				})
			}
			return ast.WalkContinue, nil
		})
		// --- END AST PARSING ---

		// Render HTML
		var buf bytes.Buffer
		md.Renderer().Render(&buf, source, doc)

		name := strings.TrimSuffix(file.Name(), ".md")
		title := strings.Title(strings.ReplaceAll(name, "-", " "))
		if name == "index" { title = "Home" }

		pg := Page{
			Title:    title,
			Link:     name + ".html",
			Content:  template.HTML(buf.String()),
			AllPages: links,
			TOC:      toc,
		}

		generateHTML(filepath.Join(OutputDir, name+".html"), pg)
		fmt.Println("Generated:", name+".html")
	}
}

func generateHTML(path string, p Page) {
	const tpl = `
<!DOCTYPE html>
<html lang="en" class="scroll-smooth">
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
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
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
    </nav>

    <div class="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <div class="lg:grid lg:grid-cols-12 lg:gap-8">
            
            <main class="lg:col-span-9">
                <article class="prose prose-lg prose-slate max-w-none prose-pre:bg-[#282a36] prose-pre:p-0 prose-h1:text-4xl">
                    <h1 class="mb-8 font-bold">{{ .Title }}</h1>
                    {{ .Content }}
                </article>
            </main>

            <aside class="hidden lg:block lg:col-span-3">
                <div class="sticky top-24 pl-4 border-l border-gray-200">
                    <p class="mb-4 text-sm font-bold tracking-wider text-gray-900 uppercase">On this page</p>
                    <nav class="flex flex-col space-y-2">
                        {{ range .TOC }}
                        <a href="#{{ .ID }}" 
                           class="text-sm text-gray-600 hover:text-blue-600 transition-colors {{ if eq .Level 3 }} pl-4 {{ end }}">
                           {{ .Title }}
                        </a>
                        {{ end }}
                    </nav>
                </div>
            </aside>

        </div>
    </div>
</body>
</html>`

	t, _ := template.New("page").Parse(tpl)
	f, _ := os.Create(path)
	defer f.Close()
	t.Execute(f, p)
}