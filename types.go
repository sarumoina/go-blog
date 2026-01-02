package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var (
	wikiLinkRegex = regexp.MustCompile(`\[\[(.*?)(?:\|(.*?))?\]\]`)
	refTagRegex   = regexp.MustCompile(`\{\{ref:(.*?)#(.*?)\}\}`)
	mdParser      goldmark.Markdown
)

func init() {
	mdParser = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.New(meta.WithStoresInDocument()),
			highlighting.NewHighlighting(highlighting.WithStyle("dracula")),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)
}

// RenderResult holds the processed data from a markdown file
type RenderResult struct {
	HTML        string
	Meta        map[string]interface{}
	TOC         []TOCEntry
	Description string
}

// ProcessMarkdown takes raw bytes and returns processed HTML and metadata
func ProcessMarkdown(source []byte) (*RenderResult, error) {
	context := parser.NewContext()
	doc := mdParser.Parser().Parse(text.NewReader(source), parser.WithContext(context))

	// 1. Extract Metadata
	metaData := meta.Get(context)

	// 2. Extract Description (first paragraph)
	var description string
	if desc, ok := metaData["description"].(string); ok && desc != "" {
		description = desc
	} else {
		child := doc.FirstChild()
		for child != nil {
			if child.Kind() == ast.KindParagraph {
				var buf bytes.Buffer
				for c := child.FirstChild(); c != nil; c = c.NextSibling() {
					if t, ok := c.(*ast.Text); ok {
						buf.Write(t.Segment.Value(source))
					}
				}
				description = string(buf.Bytes())
				if len(description) > 160 {
					description = description[:157] + "..."
				}
				break
			}
			child = child.NextSibling()
		}
	}

	// 3. Extract TOC
	var toc []TOCEntry
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := n.(*ast.Heading); ok {
			idVal, found := heading.Attribute([]byte("id"))
			if found {
				toc = append(toc, TOCEntry{
					Title: string(heading.Text(source)),
					ID:    string(idVal.([]byte)),
					Level: heading.Level,
				})
			}
		}
		return ast.WalkContinue, nil
	})

	// 4. Render HTML
	var buf bytes.Buffer
	if err := mdParser.Renderer().Render(&buf, source, doc); err != nil {
		return nil, err
	}
	htmlContent := buf.String()

	// 5. Post-process Custom Syntax
	htmlContent = processCustomSyntax(htmlContent)

	return &RenderResult{
		HTML:        htmlContent,
		Meta:        metaData,
		TOC:         toc,
		Description: description,
	}, nil
}

func processCustomSyntax(content string) string {
	// Wiki Links
	content = wikiLinkRegex.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2]
		parts := strings.SplitN(inner, "|", 2)
		linkSlug := strings.TrimSpace(parts[0])
		linkText := linkSlug
		if len(parts) > 1 {
			linkText = strings.TrimSpace(parts[1])
		}
		if !strings.HasPrefix(linkSlug, "/") {
			linkSlug = "/" + linkSlug
		}
		return fmt.Sprintf(`<a href="#%s" class="text-blue-600 dark:text-blue-400 font-medium transition-colors hover:text-blue-800 dark:hover:text-blue-300">%s</a>`, linkSlug, linkText)
	})

	// Ref Tags
	content = refTagRegex.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[6 : len(match)-2]
		parts := strings.SplitN(inner, "#", 2)
		if len(parts) != 2 {
			return `<span class="text-red-500">[Invalid Ref: ` + inner + `]</span>`
		}
		refSlug := strings.TrimSpace(parts[0])
		refID := strings.TrimSpace(parts[1])
		if !strings.HasPrefix(refSlug, "/") {
			refSlug = "/" + refSlug
		}
		return fmt.Sprintf(`<div class="transclusion-placeholder p-4 border-l-4 border-purple-500 bg-gray-50 dark:bg-gray-800 my-4" data-slug="%s" data-id="%s"><span class="text-gray-400 text-sm animate-pulse">Loading referenced content...</span></div>`, refSlug, refID)
	})

	return content
}