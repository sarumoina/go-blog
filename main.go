package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
)

const (
	InputDir  = "./content"
	OutputDir = "./public"
)

// --- DATA STRUCTURES ---

type SiteData struct {
	Pages map[string]PageData `json:"pages"`
	Menu  []*MenuItem         `json:"menu"`
}

type PageData struct {
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	TOC       []TOCEntry `json:"toc"`
	Published string     `json:"published"`
	Updated   string     `json:"updated"`
	Category  string     `json:"category"`
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

func main() {
	fmt.Println("--- BUILDING FIXED SPA ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			highlighting.NewHighlighting(highlighting.WithStyle("github")),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)

	site := SiteData{
		Pages: make(map[string]PageData),
		Menu:  []*MenuItem{},
	}

	err := filepath.WalkDir(InputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return err }
		if d.IsDir() { return nil }
		if filepath.Ext(path) != ".md" { return nil }

		relPath, _ := filepath.Rel(InputDir, path)
		relPath = filepath.ToSlash(relPath)
		filename := strings.TrimSuffix(filepath.Base(path), ".md")
		dir := filepath.Dir(relPath)
		if dir == "." { dir = "" }

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
		reader := text.NewReader(source)
		doc := markdown.Parser().Parse(reader, parser.WithContext(context))

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
			if slug == "/" { title = "Home" }
		}

		var toc []TOCEntry
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering { return ast.WalkContinue, nil }
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
			Title:     title,
			Content:   buf.String(),
			TOC:       toc,
			Published: published,
			Updated:   updated,
			Category:  category,
		}

		parts := strings.Split(strings.TrimSuffix(relPath, ".md"), "/")
		site.Menu = addToTree(site.Menu, parts, slug, title)

		return nil
	})

	if err != nil {
		fmt.Println("Error walking directory:", err)
		return
	}

	jsonBytes, _ := json.Marshal(site)
	os.WriteFile(filepath.Join(OutputDir, "db.json"), jsonBytes, 0644)
	
	writeAppShell(filepath.Join(OutputDir, "index.html"))
	fmt.Println("--- DONE ---")
}

func addToTree(nodes []*MenuItem, parts []string, slug, finalTitle string) []*MenuItem {
	if len(parts) == 0 { return nodes }
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
		if isLast { title = finalTitle }
		
		newNode := &MenuItem{
			Title:    title,
			IsFolder: !isLast,
			Children: []*MenuItem{},
		}
		if isLast { newNode.Slug = slug }
		nodes = append(nodes, newNode)
		foundNode = newNode
		
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].IsFolder != nodes[j].IsFolder { return nodes[i].IsFolder }
			return nodes[i].Title < nodes[j].Title
		})
	}

	if !isLast {
		foundNode.Children = addToTree(foundNode.Children, parts[1:], slug, finalTitle)
	}
	return nodes
}

func writeAppShell(path string) {
	// Note: We use single quotes for the Vue templates below to avoid breaking the Go backticks.
	const html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docs</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>
        tailwind.config = { theme: { extend: { fontFamily: { sans: ['Inter', 'sans-serif'] } } } }
    </script>
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    <style>
        .prose pre { background-color: #f6f8fa !important; color: #24292e !important; border-radius: 6px; }
        .prose code { color: #d73a49; font-weight: 500; }
        .prose pre code { color: inherit; font-weight: normal; }
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-thumb { background: #cbd5e1; border-radius: 3px; }
        html { scroll-behavior: smooth; }
    </style>
</head>
<body class="bg-white text-slate-800 h-screen overflow-hidden flex antialiased">
    <div id="app" class="w-full h-full flex relative">

        <aside class="bg-gray-50 border-r border-gray-200 w-64 flex-shrink-0 flex flex-col transition-all duration-300 absolute md:relative z-20 h-full"
            :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full md:w-0 md:overflow-hidden md:border-none'">
            <div class="p-5 border-b border-gray-200 flex justify-between items-center bg-white">
                <span class="font-bold text-lg tracking-tight text-slate-900">Docs</span>
                <button @click="toggleSidebar" class="md:hidden text-gray-500">✕</button>
            </div>
            <nav class="flex-1 overflow-y-auto p-3">
                <sidebar-item v-for="item in menu" :key="item.title" :item="item"></sidebar-item>
            </nav>
        </aside>

        <div class="flex-1 flex flex-col h-full overflow-hidden w-full relative bg-white">
            <header class="h-14 border-b border-gray-100 flex items-center px-4 flex-shrink-0 bg-white/80 backdrop-blur-sm z-10">
                <button @click="toggleSidebar" class="p-2 -ml-2 text-gray-400 hover:text-gray-700 rounded-md hover:bg-gray-100">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                </button>
                <div class="ml-4 font-medium text-slate-400 text-sm truncate">/ {{ currentPage.title }}</div>
            </header>

            <div v-if="loading" class="flex-1 flex items-center justify-center">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>

            <div v-else class="flex-1 overflow-hidden flex">
                <main class="flex-1 overflow-y-auto p-8 lg:p-12 scroll-smooth" ref="mainScroll">
                    <div class="max-w-3xl mx-auto">
                        <router-view v-slot="{ Component }">
                            <transition name="fade" mode="out-in">
                                <component :is="Component" :data="currentPage" />
                            </transition>
                        </router-view>
                    </div>
                </main>
                <aside class="hidden xl:block w-64 border-l border-gray-100 bg-white flex-shrink-0 overflow-y-auto p-8">
                    <div class="sticky top-0">
                        <h5 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-4">On this page</h5>
                        <nav class="space-y-2 relative border-l border-gray-100 ml-1">
                            <a v-for="link in currentPage.toc" :key="link.id" @click.prevent="scrollToHeader(link.id)" href="#"
                               class="block text-sm text-slate-500 hover:text-blue-600 transition-colors truncate cursor-pointer pl-4 -ml-px border-l-2 border-transparent hover:border-blue-500"
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
        const { createApp, ref, computed, watch } = Vue;
        const { createRouter, createWebHashHistory, useRoute } = VueRouter;

        // --- SIDEBAR ITEM (Templates fixed to use single quotes) ---
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
                    '<button @click="toggle" class="w-full flex items-center justify-between px-2 py-1.5 text-sm font-semibold text-slate-700 hover:bg-gray-100 rounded-md transition-colors">' +
                        '<div class="flex items-center"><svg class="w-4 h-4 mr-2 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"></path></svg><span>{{ item.title }}</span></div>' +
                        '<span class="text-gray-400 text-[10px] transform transition-transform duration-200" :class="isOpen ? \'rotate-90\' : \'\'">▶</span>' +
                    '</button>' +
                    '<div v-if="isOpen" class="pl-2 mt-1 ml-2 border-l border-gray-200 space-y-0.5"><sidebar-item v-for="child in item.children" :key="child.title" :item="child"></sidebar-item></div>' +
                '</div>' +
                '<router-link v-else :to="item.slug" class="block px-3 py-1.5 rounded-md text-sm font-medium transition-colors duration-200 flex items-center" :class="$route.path === item.slug ? \'bg-white text-blue-600 shadow-sm border border-gray-100\' : \'text-slate-600 hover:bg-gray-100 hover:text-slate-900\'">{{ item.title }}</router-link>' +
            '</div>'
        };

        // --- PAGE VIEW (Templates fixed to use single quotes) ---
        const PageView = {
            props: ['data'],
            template: '<div>' +
                '<div class="mb-6 border-b border-gray-100 pb-6">' +
                    '<div v-if="data.category" class="mb-3">' +
                        '<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700">{{ data.category }}</span>' +
                    '</div>' +
                    '<div class="flex items-center space-x-4 text-sm text-slate-400">' +
                        '<span v-if="data.published">Published: {{ data.published }}</span>' +
                        '<span v-if="data.published && data.updated">•</span>' +
                        '<span v-if="data.updated">Updated: {{ data.updated }}</span>' +
                    '</div>' +
                '</div>' +
                '<article class="prose prose-slate prose-lg max-w-none prose-headings:font-semibold prose-a:text-blue-600 prose-a:no-underline hover:prose-a:underline" v-html="data.content"></article>' +
            '</div>'
        };

        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
                const sidebarOpen = ref(window.innerWidth > 1024);
                const route = useRoute();
                const mainScroll = ref(null);

                fetch('db.json').then(res => res.json()).then(data => {
                    window.siteData = data;
                    menu.value = data.menu;
                    loading.value = false;
                });

                const currentPage = computed(() => {
                    if (loading.value || !window.siteData) return { toc: [] };
                    return window.siteData.pages[route.path] || { title: '404', content: "<h1 class='text-red-500'>404 Not Found</h1>", toc: [] };
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

                return { loading, menu, currentPage, sidebarOpen, toggleSidebar, mainScroll, scrollToHeader };
            }
        });

        app.component('sidebar-item', SidebarItem);
        app.use(createRouter({
            history: createWebHashHistory(),
            routes: [ { path: '/:pathMatch(.*)*', component: PageView } ]
        }));
        app.mount('#app');
    </script>
</body>
</html>`
	os.WriteFile(path, []byte(html), 0644)
}