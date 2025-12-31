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
	BaseURL   = "https://mysite.com" // Update this for sitemap
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
}

type MenuItem struct {
	Title    string      `json:"title"`
	Slug     string      `json:"slug"`
	IsFolder bool        `json:"is_folder"`
	Children []*MenuItem `json:"children,omitempty"`
}

type TOCEntry struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	Level int    `json:"level"`
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func main() {
	fmt.Println("--- BUILDING FEATURE-PACKED SITE ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	// Note: We use "dracula" style. In Dark mode it fits perfectly.
	// In Light mode, it still looks good as a contrast block.
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

		published := getString("published on")
		updated := getString("updated on")
		category := getString("category")
		title := getString("title")

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

		site.Pages[slug] = PageData{
			Title:       title,
			Content:     buf.String(),
			TOC:         toc,
			Published:   published,
			Updated:     updated,
			Category:    category,
			Description: description,
		}

		parts := strings.Split(strings.TrimSuffix(relPath, ".md"), "/")
		site.Menu = addToTree(site.Menu, parts, slug, title)
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

func addToTree(nodes []*MenuItem, parts []string, slug, finalTitle string) []*MenuItem {
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
		}
		nodes = append(nodes, newNode)
		foundNode = newNode
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsFolder != nodes[j].IsFolder {
				return nodes[i].IsFolder
			}
			return nodes[i].Title < nodes[j].Title
		})
	}

	if !isLast {
		foundNode.Children = addToTree(foundNode.Children, parts[1:], slug, finalTitle)
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
	// 1. ADDED: tailwind darkMode: 'class'
	// 2. ADDED: Script to transform Admonitions and Copy Buttons
	// 3. ADDED: Search Component logic
	// 4. UPDATE: Removed Sitemap link, added fixed Home link
	const html = `<!DOCTYPE html>
<html lang="en" class="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docs</title>
    <meta name="description" content="Documentation">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>
        tailwind.config = { 
            darkMode: 'class', // Enable class-based dark mode
            theme: { extend: { fontFamily: { sans: ['Inter', 'sans-serif'] } } } 
        }
    </script>
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    <style>
        /* CSS for Admonitions */
        .admonition { border-left-width: 4px; padding: 1rem; margin-bottom: 1.5rem; border-radius: 0.375rem; background-color: #f9fafb; }
        .dark .admonition { background-color: #1f2937; }
        .admonition-title { font-weight: 700; margin-bottom: 0.5rem; display: flex; align-items: center; }
        
        .admonition-note { border-color: #3b82f6; } /* Blue */
        .admonition-note .admonition-title { color: #2563eb; }
        
        .admonition-tip { border-color: #10b981; } /* Green */
        .admonition-tip .admonition-title { color: #059669; }
        
        .admonition-warning { border-color: #f59e0b; } /* Orange */
        .admonition-warning .admonition-title { color: #d97706; }
        
        .admonition-important { border-color: #8b5cf6; } /* Purple */
        .admonition-important .admonition-title { color: #7c3aed; }
        
        .admonition-caution { border-color: #ef4444; } /* Red */
        .admonition-caution .admonition-title { color: #dc2626; }

        /* Dark mode typography adjustments */
        .dark .prose { color: #d1d5db; }
        .dark .prose h1, .dark .prose h2, .dark .prose h3, .dark .prose h4 { color: #f3f4f6; }
        .dark .prose a { color: #60a5fa; }
        .dark .prose strong { color: #f3f4f6; }
        .dark .prose code { color: #fca5a5; }
        .prose h1:first-of-type { display: none; }
        
        /* Copy Button Styles */
        .code-wrapper { position: relative; }
        .copy-btn { 
            position: absolute; top: 0.5rem; right: 0.5rem; 
            padding: 0.25rem 0.5rem; font-size: 0.75rem; 
            background: rgba(255,255,255,0.1); border: 1px solid rgba(255,255,255,0.2); 
            border-radius: 0.25rem; color: #fff; cursor: pointer; opacity: 0; transition: opacity 0.2s;
        }
        .code-wrapper:hover .copy-btn { opacity: 1; }

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
                <router-link to="/" class="font-bold text-lg tracking-tight text-slate-900 dark:text-white">Docs</router-link>
                <button @click="toggleSidebar" class="md:hidden text-gray-500 dark:text-gray-400">✕</button>
            </div>
            
            <div class="p-3 border-b border-gray-200 dark:border-gray-700">
                <input v-model="searchQuery" type="text" placeholder="Search pages..." 
                    class="w-full px-3 py-2 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-white placeholder-gray-400">
            </div>

            <div v-if="searchQuery" class="flex-1 overflow-y-auto p-3 bg-white dark:bg-gray-800">
                <div v-if="filteredPages.length === 0" class="text-sm text-gray-500 text-center py-4">No results found.</div>
                <ul v-else class="space-y-1">
                    <li v-for="page in filteredPages" :key="page.slug">
                        <router-link :to="page.slug" @click="searchQuery = ''" class="block px-2 py-1.5 text-sm text-slate-700 dark:text-gray-300 hover:bg-blue-50 dark:hover:bg-gray-700 hover:text-blue-600 rounded-md">
                            <div class="font-medium">{{ page.title }}</div>
                            <div class="text-xs text-gray-400 truncate">{{ page.slug }}</div>
                        </router-link>
                    </li>
                </ul>
            </div>

            <nav v-else class="flex-1 overflow-y-auto p-3">
                 <div class="mb-1">
                    <router-link to="/" class="block px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-200 flex items-center" 
                        :class="$route.path === '/' ? 'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm border border-gray-100 dark:border-gray-700' : 'text-slate-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-slate-900 dark:hover:text-gray-200'">
                        Home
                    </router-link>
                </div>
                <sidebar-item v-for="item in menu" :key="item.title" :item="item" v-if="item.slug !== '/'"></sidebar-item>
            </nav>
        </aside>

        <div class="flex-1 flex flex-col h-full overflow-hidden w-full relative bg-white dark:bg-gray-900">
            <header class="h-14 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between px-4 flex-shrink-0 bg-white/80 dark:bg-gray-900/80 backdrop-blur-sm z-10">
                <div class="flex items-center">
                    <button @click="toggleSidebar" class="p-2 -ml-2 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                    </button>
                    <div class="ml-4 font-medium text-slate-400 text-sm truncate">/ {{ currentPage.title }}</div>
                </div>

                <button @click="toggleDarkMode" class="p-2 text-gray-400 hover:text-yellow-500 dark:hover:text-yellow-300 transition-colors">
                    <span v-if="isDark">☀️</span>
                    <span v-else>🌙</span>
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
                                <component :is="Component" :data="currentPage" :menu="menu" />
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

        // --- SIDEBAR ITEM ---
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
                        '<div class="flex items-center"><svg class="w-4 h-4 mr-2 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg><span>{{ item.title }}</span></div>' +
                        '<span class="text-gray-400 text-[10px] transform transition-transform duration-200" :class="isOpen ? \'rotate-90\' : \'\'">▶</span>' +
                    '</button>' +
                    '<div v-if="isOpen" class="pl-2 mt-1 ml-2 border-l border-gray-200 dark:border-gray-700 space-y-0.5"><sidebar-item v-for="child in item.children" :key="child.title" :item="child"></sidebar-item></div>' +
                '</div>' +
                '<router-link v-else :to="item.slug" class="block px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-200 flex items-center" :class="$route.path === item.slug ? \'bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm border border-gray-100 dark:border-gray-700\' : \'text-slate-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-slate-900 dark:hover:text-gray-200\'">{{ item.title }}</router-link>' +
            '</div>'
        };

        // --- PAGE VIEW (With Admonition & Copy Logic) ---
        const PageView = {
            props: ['data'],
            setup(props) {
                // Post-process HTML for Admonitions
                const processedContent = computed(() => {
                    if (!props.data.content) return '';
                    let html = props.data.content;
                    
                    // Regex for Admonitions: <blockquote><p>[!TYPE] ...</p></blockquote>
                    const regex = /<blockquote>\s*<p>\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*(.*?)<\/p>\s*(.*?)<\/blockquote>/gs;
                    
                    html = html.replace(regex, (match, type, titleLine, body) => {
                        const typeLower = type.toLowerCase();
                        const title = titleLine.trim() || type;
                        return '<div class="admonition admonition-' + typeLower + '">' +
                               '<div class="admonition-title">' + title + '</div>' +
                               '<div>' + body + '</div>' +
                               '</div>';
                    });
                    return html;
                });

                // Add Copy Buttons after render
                onMounted(() => injectCopyButtons());
                watch(() => props.data.content, () => nextTick(injectCopyButtons));

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

                return { processedContent };
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
            '</div>'
        };

        const SitemapView = {
            props: ['menu'],
            template: '<div><h1 class="text-4xl font-bold mb-8 dark:text-white">Site Index</h1><div class="grid grid-cols-1 md:grid-cols-2 gap-8"><div v-for="item in menu" :key="item.title"><h3 class="font-bold text-lg mb-2 text-slate-800 dark:text-gray-200">{{ item.title }}</h3><ul class="space-y-1"><li v-if="!item.is_folder"><router-link :to="item.slug" class="text-blue-600 dark:text-blue-400 hover:underline">{{ item.title }}</router-link></li><li v-else v-for="child in item.children" :key="child.title" class="ml-4 list-disc marker:text-slate-300 dark:marker:text-gray-600"><router-link :to="child.slug" class="text-blue-600 dark:text-blue-400 hover:underline">{{ child.title }}</router-link></li></ul></div></div></div>'
        };

        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
                const sidebarOpen = ref(window.innerWidth > 1024);
                const route = useRoute();
                const mainScroll = ref(null);
                
                // Dark Mode Logic
                const isDark = ref(localStorage.getItem('theme') === 'dark');
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

                // Search Logic
                const searchQuery = ref('');
                const allPagesList = ref([]);
                const filteredPages = computed(() => {
                    if (!searchQuery.value) return [];
                    const q = searchQuery.value.toLowerCase();
                    return allPagesList.value.filter(p => p.title.toLowerCase().includes(q));
                });

                fetch('db.json').then(res => res.json()).then(data => {
                    window.siteData = data;
                    menu.value = data.menu;
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

                return { loading, menu, currentPage, sidebarOpen, toggleSidebar, mainScroll, scrollToHeader, isDark, toggleDarkMode, searchQuery, filteredPages };
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
