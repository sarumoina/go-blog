package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

const (
	InputDir  = "./content"
	OutputDir = "./public"
	BaseURL   = "https://mysite.com"
)

type SiteData struct {
	Pages map[string]PageData `json:"pages"`
	Menu  []*MenuItem         `json:"menu"`
}

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

type MenuItem struct {
	Title    string      `json:"title"`
	Slug     string      `json:"slug"`
	IsFolder bool        `json:"is_folder"`
	Weight   int         `json:"weight"`
	Children []*MenuItem `json:"children,omitempty"`
}

type TOCEntry struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	Level int    `json:"level"`
}

var wikiLinkRegex = regexp.MustCompile(`\[\[(.*?)(?:\|(.*?))?\]\]`)
var refTagRegex = regexp.MustCompile(`\{\{ref:(.*?)#(.*?)\}\}`)

func main() {
	fmt.Println("--- BUILDING OPTIMIZED SITE ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.New(meta.WithStoresInDocument()),
			highlighting.NewHighlighting(highlighting.WithStyle("dracula")),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)

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

		source, _ := os.ReadFile(path)
		context := parser.NewContext()
		doc := markdown.Parser().Parse(text.NewReader(source), parser.WithContext(context))

		metaData := meta.Get(context)
		getString := func(key string) string {
			if val, ok := metaData[key]; ok {
				return fmt.Sprintf("%v", val)
			}
			return ""
		}
		getInt := func(key string) int {
			if val, ok := metaData[key]; ok {
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

		var description string
		if desc := getString("description"); desc != "" {
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

		var buf bytes.Buffer
		markdown.Renderer().Render(&buf, source, doc)
		htmlContent := buf.String()

		// Wiki Links (Updated: Removed hover:underline)
		htmlContent = wikiLinkRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
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
		htmlContent = refTagRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
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

		site.Pages[slug] = PageData{
			Title:       title,
			Content:     htmlContent,
			TOC:         toc,
			Published:   published,
			Updated:     updated,
			Category:    category,
			Description: description,
			Weight:      weight,
		}

		parts := strings.Split(strings.TrimSuffix(relPath, ".md"), "/")
		site.Menu = addToTree(site.Menu, parts, slug, title, weight)
		xmlUrls = append(xmlUrls, slug)
		return nil
	})

	if err != nil {
		fmt.Println("Error walking directory:", err)
		return
	}

	generateXMLSitemap(xmlUrls)
	jsonBytes, _ := json.Marshal(site)
	os.WriteFile(filepath.Join(OutputDir, "db.json"), jsonBytes, 0644)
	writeAppShell(filepath.Join(OutputDir, "index.html"))
	fmt.Println("--- DONE ---")
}

func addToTree(nodes []*MenuItem, parts []string, slug, finalTitle string, weight int) []*MenuItem {
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

		// SORT LOGIC UPDATE:
		// 1. Home (/) always first
		// 2. Weight (Lowest first)
		// 3. Folders first
		// 4. Alphabetical
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
		foundNode.Children = addToTree(foundNode.Children, parts[1:], slug, finalTitle, weight)
	}
	return nodes
}

func generateXMLSitemap(slugs []string) {
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
	os.WriteFile(filepath.Join(OutputDir, "sitemap.xml"), buf.Bytes(), 0644)
}

func writeAppShell(path string) {
	const html = `<!DOCTYPE html>
<html lang="en" class="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docs</title>
    <meta name="description" content="Documentation">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="https://cdn.lineicons.com/4.0/lineicons.css" />
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>
        tailwind.config = { 
            darkMode: 'class', 
            theme: { extend: { fontFamily: { sans: ['Inter', 'sans-serif'] } } } 
        }
    </script>
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    <style>
        .admonition { border-left-width: 4px; padding: 1rem; margin-bottom: 1.5rem; border-radius: 0.375rem; background-color: #f9fafb; }
        .dark .admonition { background-color: #1f2937; }
        .admonition-title { font-weight: 700; margin-bottom: 0.5rem; display: flex; align-items: center; }
        .admonition-title i { font-size: 1.25rem; margin-right: 0.5rem; }
        .admonition-note { border-color: #3b82f6; } .admonition-note .admonition-title { color: #2563eb; }
        .admonition-tip { border-color: #10b981; } .admonition-tip .admonition-title { color: #059669; }
        .admonition-warning { border-color: #f59e0b; } .admonition-warning .admonition-title { color: #d97706; }
        .admonition-important { border-color: #8b5cf6; } .admonition-important .admonition-title { color: #7c3aed; }
        .admonition-caution { border-color: #ef4444; } .admonition-caution .admonition-title { color: #dc2626; }
        .dark .prose { color: #d1d5db; }
        .dark .prose h1, .dark .prose h2, .dark .prose h3, .dark .prose h4 { color: #f3f4f6; }
        .dark .prose a { color: #60a5fa; }
        .dark .prose strong { color: #f3f4f6; }
        .dark .prose code { color: #fca5a5; }
        .prose h1:first-of-type { display: none; }
        .code-wrapper { position: relative; }
        .copy-btn { 
            position: absolute; top: 0.5rem; right: 0.5rem; 
            padding: 0.25rem 0.5rem; font-size: 0.75rem; 
            background: rgba(255,255,255,0.1); border: 1px solid rgba(255,255,255,0.2); 
            border-radius: 0.25rem; color: #fff; cursor: pointer; opacity: 0; transition: opacity 0.2s;
        }
        .code-wrapper:hover .copy-btn { opacity: 1; }
        .transclusion-placeholder h1, .transclusion-placeholder h2, .transclusion-placeholder h3 { margin-top: 0 !important; font-size: 1.2em; }
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-thumb { background: #cbd5e1; border-radius: 3px; }
        .dark ::-webkit-scrollbar-thumb { background: #4b5563; }
        html { scroll-behavior: smooth; }
    </style>
</head>
<body class="bg-white dark:bg-gray-900 text-slate-800 dark:text-gray-200 h-screen overflow-hidden flex antialiased transition-colors duration-200">
    <div id="app" class="w-full h-full flex relative">
        <aside class="bg-gray-50 dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 w-64 flex-shrink-0 flex flex-col transition-all duration-300 absolute md:relative z-20 h-full"
            :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full md:w-0 md:overflow-hidden md:border-none'">
            <div class="p-5 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center bg-gray-50 dark:bg-gray-800">
                <router-link to="/" class="font-bold text-lg tracking-tight text-slate-900 dark:text-white flex items-center">
                    <i class="lni lni-library mr-2 text-blue-600"></i> Docs
                </router-link>
                <button @click="toggleSidebar" class="md:hidden text-gray-500 dark:text-gray-400">
                    <i class="lni lni-close"></i>
                </button>
            </div>
            <div class="p-3 border-b border-gray-200 dark:border-gray-700">
                <div class="relative">
                    <i class="lni lni-search-alt absolute left-3 top-2.5 text-gray-400"></i>
                    <input v-model="searchQuery" type="text" placeholder="Search..." 
                        class="w-full pl-9 pr-3 py-2 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-white">
                </div>
            </div>
            <div v-if="searchQuery" class="flex-1 overflow-y-auto p-3 bg-white dark:bg-gray-800">
                <ul v-if="filteredPages.length > 0" class="space-y-1">
                    <li v-for="page in filteredPages" :key="page.slug">
                        <router-link :to="page.slug" @click="searchQuery = ''" class="block px-2 py-1.5 text-sm text-slate-700 dark:text-gray-300 hover:bg-blue-50 dark:hover:bg-gray-700 hover:text-blue-600 rounded-md">
                            <div class="font-medium">{{ page.title }}</div>
                        </router-link>
                    </li>
                </ul>
                <div v-else class="text-sm text-gray-500 text-center py-4">No results.</div>
            </div>
            <nav v-else class="flex-1 overflow-y-auto p-3">
                 <div class="mb-1">
                    <router-link to="/" class="block px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-200 flex items-center" 
                        :class="$route.path === '/' ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm border border-gray-100 dark:border-gray-700' : 'text-slate-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-slate-900 dark:hover:text-gray-200'">
                        <i class="lni lni-home mr-2"></i> Home
                    </router-link>
                </div>
                <sidebar-item v-for="item in filteredMenu" :key="item.title" :item="item"></sidebar-item>
            </nav>
        </aside>

        <div class="flex-1 flex flex-col h-full overflow-hidden w-full relative bg-white dark:bg-gray-900">
            <header class="h-14 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between px-4 flex-shrink-0 bg-white/80 dark:bg-gray-900/80 backdrop-blur-sm z-10">
                <div class="flex items-center">
                    <button @click="toggleSidebar" class="p-2 -ml-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800">
                        <i class="lni lni-menu text-xl"></i>
                    </button>
                    <div class="ml-4 font-medium text-slate-400 text-sm truncate">/ {{ currentPage.title }}</div>
                </div>
                <button @click="toggleDarkMode" class="p-2 text-gray-400 hover:text-yellow-500 dark:hover:text-yellow-300 transition-colors">
                    <i v-if="isDark" class="lni lni-sun text-lg"></i>
                    <i v-else class="lni lni-night text-lg"></i>
                </button>
            </header>

            <div v-if="loading" class="flex-1 flex items-center justify-center">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>

            <div v-else class="flex-1 overflow-hidden flex">
                <main class="flex-1 overflow-y-auto p-8 lg:p-12 scroll-smooth" ref="mainScroll">
                    <div class="max-w-3xl mx-auto">
                        <router-view v-slot="{ Component }">
                            <transition name="fade" mode="out-in">
                                <component :is="Component" :data="currentPage" :menu="menu" :flat-menu="flatMenu" />
                            </transition>
                        </router-view>
                    </div>
                </main>
                <aside v-if="currentPage.toc && currentPage.toc.length > 0" class="hidden xl:block w-64 border-l border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900 flex-shrink-0 overflow-y-auto p-8">
                    <div class="sticky top-0">
                        <h5 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-4">On this page</h5>
                        <nav class="space-y-2 relative border-l border-gray-100 dark:border-gray-800 ml-1">
                            <a v-for="link in currentPage.toc" :key="link.id" @click.prevent="scrollToHeader(link.id)" href="#"
                               class="block text-sm text-slate-500 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors truncate cursor-pointer pl-4 -ml-px border-l-2 border-transparent hover:border-blue-500"
                               :class="{ 'pl-8': link.level === 3 }">
                               {{ link.title }}
                            </a>
                        </nav>
                    </div>
                </aside>
            </div>
        </div>
        <div v-if="sidebarOpen" @click="toggleSidebar" class="md:hidden fixed inset-0 bg-gray-900 bg-opacity-20 z-10 backdrop-blur-sm"></div>
    </div>

    <script>
        const { createApp, ref, computed, watch, onMounted, nextTick } = Vue;
        const { createRouter, createWebHashHistory, useRoute } = VueRouter;

        const SidebarItem = {
            name: 'SidebarItem',
            props: ['item'],
            setup(props) {
                const route = useRoute();
                const isOpen = ref(false);
                const hasActiveChild = (item, currentPath) => {
                    if (item.slug === currentPath) return true;
                    if (item.children) return item.children.some(child => hasActiveChild(child, currentPath));
                    return false;
                };
                watch(() => route.path, (newPath) => {
                    if (props.item.is_folder && hasActiveChild(props.item, newPath)) isOpen.value = true;
                }, { immediate: true });
                return { isOpen, toggle: () => isOpen.value = !isOpen.value };
            },
            template: '<div class="mb-1 select-none">' +
                '<div v-if="item.is_folder">' +
                    '<button @click="toggle" class="w-full flex items-center justify-between px-2 py-1.5 text-sm font-semibold text-slate-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors">' +
                        '<div class="flex items-center"><i class="lni lni-folder mr-2 text-slate-400"></i><span>{{ item.title }}</span></div>' +
                        '<i class="lni lni-chevron-right text-xs text-gray-400 transform transition-transform duration-200" :class="isOpen ? \'rotate-90\' : \'\'"></i>' +
                    '</button>' +
                    '<div v-if="isOpen" class="pl-2 mt-1 ml-2 border-l border-gray-200 dark:border-gray-700 space-y-0.5"><sidebar-item v-for="child in item.children" :key="child.title" :item="child"></sidebar-item></div>' +
                '</div>' +
                '<router-link v-else :to="item.slug" class="block px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-200 flex items-center" :class="$route.path === item.slug ? \'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm border border-gray-100 dark:border-gray-700\' : \'text-slate-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-slate-900 dark:hover:text-gray-200\'">{{ item.title }}</router-link>' +
            '</div>'
        };

        const PageView = {
            props: ['data', 'flatMenu'],
            setup(props) {
                const route = useRoute();
                const processedContent = computed(() => {
                    if (!props.data.content) return '';
                    let html = props.data.content;
                    const icons = {
                        note: '<i class="lni lni-notepad"></i>',
                        tip: '<i class="lni lni-bulb"></i>',
                        important: '<i class="lni lni-bookmark"></i>',
                        warning: '<i class="lni lni-warning"></i>',
                        caution: '<i class="lni lni-ban"></i>'
                    };
                    const regex = /<blockquote>\s*<p>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*?)<\/p>\s*(.*?)<\/blockquote>/gs;
                    html = html.replace(regex, (match, type, titleLine, body) => {
                        const typeLower = type.toLowerCase();
                        const title = titleLine.trim() || type;
                        const icon = icons[typeLower] || icons.note;
                        return '<div class="admonition admonition-' + typeLower + '"><div class="admonition-title">' + icon + ' ' + title + '</div><div>' + body + '</div></div>';
                    });
                    return html;
                });

                const navLinks = computed(() => {
                    if (!props.flatMenu || props.flatMenu.length === 0) return { prev: null, next: null };
                    const currentIndex = props.flatMenu.findIndex(p => p.slug === route.path);
                    if (currentIndex === -1) return { prev: null, next: null };
                    
                    // Logic updated: Home (index 0) gets no Prev. Last gets no Next.
                    return {
                        prev: currentIndex > 0 ? props.flatMenu[currentIndex - 1] : null,
                        next: currentIndex < props.flatMenu.length - 1 ? props.flatMenu[currentIndex + 1] : null
                    };
                });

                onMounted(() => { injectCopyButtons(); resolveTransclusions(); });
                watch(() => props.data.content, () => nextTick(() => { injectCopyButtons(); resolveTransclusions(); }));

                function injectCopyButtons() {
                    document.querySelectorAll('pre').forEach(pre => {
                        if (pre.parentNode.classList.contains('code-wrapper')) return;
                        const wrapper = document.createElement('div');
                        wrapper.className = 'code-wrapper';
                        pre.parentNode.insertBefore(wrapper, pre);
                        wrapper.appendChild(pre);
                        const btn = document.createElement('button');
                        btn.className = 'copy-btn';
                        btn.textContent = 'Copy';
                        btn.onclick = () => {
                            navigator.clipboard.writeText(pre.innerText).then(() => {
                                btn.textContent = 'Copied!';
                                setTimeout(() => btn.textContent = 'Copy', 2000);
                            });
                        };
                        wrapper.appendChild(btn);
                    });
                }
                function resolveTransclusions() {
                     const placeholders = document.querySelectorAll('.transclusion-placeholder');
                    if (placeholders.length === 0) return;
                    placeholders.forEach(el => {
                        const slug = el.getAttribute('data-slug');
                        const id = el.getAttribute('data-id');
                        if (window.siteData && window.siteData.pages[slug]) {
                            const rawHtml = window.siteData.pages[slug].content;
                            const tempDiv = document.createElement('div');
                            tempDiv.innerHTML = rawHtml;
                            const startNode = tempDiv.querySelector('#' + id);
                            if (startNode) {
                                let content = '';
                                const startLevel = parseInt(startNode.tagName.substring(1));
                                let currentNode = startNode;
                                content += currentNode.outerHTML; 
                                while (currentNode.nextElementSibling) {
                                    currentNode = currentNode.nextElementSibling;
                                    const tagName = currentNode.tagName;
                                    if (/^H[1-6]$/.test(tagName)) {
                                        const currentLevel = parseInt(tagName.substring(1));
                                        if (currentLevel <= startLevel) break;
                                    }
                                    content += currentNode.outerHTML;
                                }
                                el.innerHTML = content;
                                el.classList.remove('animate-pulse');
                                el.classList.add('bg-opacity-50');
                            } else {
                                el.innerHTML = '<span class="text-red-500 text-sm">Error: Section #'+id+' not found in '+slug+'</span>';
                            }
                        } else {
                            el.innerHTML = '<span class="text-red-500 text-sm">Error: Page '+slug+' not found</span>';
                        }
                    });
                }

                return { processedContent, navLinks };
            },
            template: '<div>' +
                '<h1 class="text-4xl font-bold text-slate-900 dark:text-white mb-4 tracking-tight">{{ data.title }}</h1>' +
                '<div class="flex items-center flex-wrap gap-4 text-sm text-slate-500 dark:text-gray-400 mb-8 pb-6 border-b border-gray-100 dark:border-gray-800">' +
                    '<span v-if="data.category" class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 dark:bg-blue-900 text-blue-700 dark:text-blue-200 border border-blue-100 dark:border-blue-800">{{ data.category }}</span>' +
                    '<div v-if="data.published || data.updated" class="flex items-center space-x-3 ml-1">' +
                        '<span v-if="data.published">Published: <span class="text-slate-700 dark:text-gray-300 font-medium">{{ data.published }}</span></span>' +
                        '<span v-if="data.published && data.updated" class="text-gray-300 dark:text-gray-600">•</span>' +
                        '<span v-if="data.updated">Updated: <span class="text-slate-700 dark:text-gray-300 font-medium">{{ data.updated }}</span></span>' +
                    '</div>' +
                '</div>' +
                '<article class="prose prose-slate dark:prose-invert prose-lg max-w-none prose-headings:font-semibold prose-a:text-blue-600 prose-a:no-underline hover:prose-a:underline" v-html="processedContent"></article>' +
                '<div class="mt-16 pt-8 border-t border-gray-100 dark:border-gray-800 flex flex-col md:flex-row justify-between gap-4">' +
                    '<div v-if="navLinks.prev">' +
                        '<div class="text-xs text-gray-500 mb-1">Previous</div>' +
                        '<router-link :to="navLinks.prev.slug" class="text-blue-600 dark:text-blue-400 font-medium transition-colors hover:text-blue-800 dark:hover:text-blue-300 flex items-center">' +
                            '<i class="lni lni-arrow-left mr-2"></i> {{ navLinks.prev.title }}' +
                        '</router-link>' +
                    '</div>' +
                    '<div v-else class="flex-1"></div>' +
                    '<div v-if="navLinks.next" class="text-right">' +
                        '<div class="text-xs text-gray-500 mb-1">Next</div>' +
                        '<router-link :to="navLinks.next.slug" class="text-blue-600 dark:text-blue-400 font-medium transition-colors hover:text-blue-800 dark:hover:text-blue-300 flex items-center justify-end">' +
                            '{{ navLinks.next.title }} <i class="lni lni-arrow-right ml-2"></i>' +
                        '</router-link>' +
                    '</div>' +
                '</div>' +
            '</div>'
        };

        const SitemapView = {
            props: ['menu'],
            template: '<div><h1 class="text-4xl font-bold mb-8 dark:text-white">Site Index</h1><div class="grid grid-cols-1 md:grid-cols-2 gap-8"><div v-for="item in menu" :key="item.title"><h3 class="font-bold text-lg mb-2 text-slate-800 dark:text-gray-200">{{ item.title }}</h3><ul class="space-y-1"><li v-if="!item.is_folder"><router-link :to="item.slug" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300">{{ item.title }}</router-link></li><li v-else v-for="child in item.children" :key="child.title" class="ml-4 list-disc marker:text-slate-300 dark:marker:text-gray-600"><router-link :to="child.slug" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300">{{ child.title }}</router-link></li></ul></div></div></div>'
        };

        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
                const flatMenu = ref([]);
                const sidebarOpen = ref(window.innerWidth > 1024);
                const route = useRoute();
                const mainScroll = ref(null);
                const isDark = ref(localStorage.getItem('theme') === 'dark');
                const filteredMenu = computed(() => { return menu.value.filter(item => item.slug !== '/'); });

                const toggleDarkMode = () => {
                    isDark.value = !isDark.value;
                    if (isDark.value) {
                        document.documentElement.classList.add('dark');
                        localStorage.setItem('theme', 'dark');
                    } else {
                        document.documentElement.classList.remove('dark');
                        localStorage.setItem('theme', 'light');
                    }
                };
                if (isDark.value) document.documentElement.classList.add('dark');
                
                const searchQuery = ref('');
                const allPagesList = ref([]);
                const filteredPages = computed(() => {
                    if (!searchQuery.value) return [];
                    const q = searchQuery.value.toLowerCase();
                    return allPagesList.value.filter(p => p.title.toLowerCase().includes(q));
                });

                const flattenMenuTree = (items) => {
                    let flat = [];
                    items.forEach(item => {
                        if (!item.is_folder) flat.push(item);
                        if (item.children) flat = flat.concat(flattenMenuTree(item.children));
                    });
                    return flat;
                };
                
                fetch('db.json').then(res => res.json()).then(data => {
                    window.siteData = data;
                    menu.value = data.menu;
                    flatMenu.value = flattenMenuTree(data.menu);
                    allPagesList.value = Object.keys(data.pages).map(slug => ({
                        slug, ...data.pages[slug]
                    }));
                    loading.value = false;
                });
                
                const currentPage = computed(() => {
                    if (loading.value || !window.siteData) return { toc: [] };
                    return window.siteData.pages[route.path] || { title: '404', content: "<h1 class='text-red-500'>404 Not Found</h1>", toc: [] };
                });
                
                watch(() => currentPage.value, (page) => {
                    document.title = page.title ? page.title : 'Docs';
                    const metaDesc = document.querySelector('meta[name="description"]');
                    if (metaDesc) metaDesc.setAttribute("content", page.description || "Documentation");
                });
                
                watch(() => route.path, () => {
                    if(mainScroll.value) mainScroll.value.scrollTop = 0;
                    if(window.innerWidth < 1024) sidebarOpen.value = false;
                });
                
                const toggleSidebar = () => sidebarOpen.value = !sidebarOpen.value;
                const scrollToHeader = (id) => {
                    const el = document.getElementById(id);
                    if(el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
                };
                
                return { loading, menu, flatMenu, filteredMenu, currentPage, sidebarOpen, toggleSidebar, mainScroll, scrollToHeader, isDark, toggleDarkMode, searchQuery, filteredPages };
            }
        });

        app.component('sidebar-item', SidebarItem);
        app.use(createRouter({
            history: createWebHashHistory(),
            routes: [ { path: '/sitemap', component: SitemapView }, { path: '/:pathMatch(.*)*', component: PageView } ]
        }));
        app.mount('#app');
    </script>
</body>
</html>`
	os.WriteFile(path, []byte(html), 0644)
}
