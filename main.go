package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

const (
	InputDir  = "./content"
	OutputDir = "./public"
)

// The JSON Structure Vue will read
type SiteData struct {
	Pages map[string]PageData `json:"pages"` // Map of "slug" -> Content
	Menu  []MenuItem          `json:"menu"`  // Navigation links
}

type PageData struct {
	Title   string     `json:"title"`
	Content string     `json:"content"` // Pre-rendered HTML
	TOC     []TOCEntry `json:"toc"`
}

type MenuItem struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type TOCEntry struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	Level int    `json:"level"`
}

func main() {
	fmt.Println("--- BUILDING VUE SPA ---")

	// 1. Setup
	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	// 2. Parser Setup
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, highlighting.NewHighlighting(highlighting.WithStyle("dracula"))),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)

	// 3. Scan Files
	files, _ := os.ReadDir(InputDir)
	site := SiteData{
		Pages: make(map[string]PageData),
		Menu:  []MenuItem{},
	}

	fmt.Printf("Processing %d files...\n", len(files))

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".md" { continue }

		// Read & Parse
		srcPath := filepath.Join(InputDir, file.Name())
		source, _ := os.ReadFile(srcPath)
		
		// Generate Slug (URL friendly name)
		slug := strings.TrimSuffix(file.Name(), ".md")
		// "index" becomes the root path "/"
		if slug == "index" { slug = "/" } 

		// Generate Title
		title := strings.Title(strings.ReplaceAll(strings.TrimSuffix(file.Name(), ".md"), "-", " "))
		if slug == "/" { title = "Home" }

		// Extract TOC
		context := parser.NewContext()
		reader := text.NewReader(source)
		doc := md.Parser().Parse(reader, parser.WithContext(context))
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

		// Render HTML
		var buf bytes.Buffer
		md.Renderer().Render(&buf, source, doc)

		// Add to Data Structures
		site.Pages[slug] = PageData{
			Title:   title,
			Content: buf.String(), // We send HTML string to Vue
			TOC:     toc,
		}
		
		// Add to Menu (Sorted by filename usually, simplified here)
		site.Menu = append(site.Menu, MenuItem{Title: title, Slug: slug})
	}

	// 4. Write Database (db.json)
	jsonBytes, _ := json.Marshal(site)
	os.WriteFile(filepath.Join(OutputDir, "db.json"), jsonBytes, 0644)
	fmt.Println(" >> Created public/db.json")

	// 5. Write App Shell (index.html)
	writeAppShell(filepath.Join(OutputDir, "index.html"))
	fmt.Println(" >> Created public/index.html")
	fmt.Println("--- DONE ---")
}

func writeAppShell(path string) {
	// This HTML contains the Vue App logic
	const html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My Docs</title>
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>tailwind.config = { theme: { extend: { colors: { dracula: '#282a36' } } } }</script>
    
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    
    <style>
        pre { border-radius: 0.5rem; } 
        /* Transitions for SPA feel */
        .fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
        .fade-enter-from, .fade-leave-to { opacity: 0; }
    </style>
</head>
<body class="bg-gray-50 text-gray-900 h-screen overflow-hidden">
    <div id="app" class="h-full flex flex-col">
        
        <nav class="bg-white border-b border-gray-200 shadow-sm shrink-0 z-50">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div class="flex justify-between h-16">
                    <div class="flex items-center">
                        <span class="font-bold text-xl tracking-tight text-gray-900 mr-8">My Docs</span>
                        <div class="hidden md:flex space-x-4">
                            <router-link v-for="item in menu" :key="item.slug" :to="item.slug" 
                                class="px-3 py-2 rounded-md text-sm font-medium transition-colors"
                                :class="$route.path === item.slug ? 'bg-blue-50 text-blue-700' : 'text-gray-600 hover:bg-gray-100'">
                                {{ item.title }}
                            </router-link>
                        </div>
                    </div>
                </div>
            </div>
        </nav>

        <div v-if="loading" class="flex-1 flex items-center justify-center">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
        </div>

        <div v-else class="flex-1 flex overflow-hidden">
            <div class="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 flex overflow-hidden">
                
                <main class="flex-1 overflow-y-auto pr-6" ref="mainScroll">
                    <router-view v-slot="{ Component }">
                        <transition name="fade" mode="out-in">
                            <component :is="Component" />
                        </transition>
                    </router-view>
                </main>

                <aside class="hidden lg:block w-64 shrink-0 overflow-y-auto border-l border-gray-200 pl-4">
                    <div class="sticky top-0">
                        <p class="mb-4 text-sm font-bold tracking-wider text-gray-900 uppercase">On this page</p>
                        <nav class="flex flex-col space-y-2">
                            <a v-for="link in currentTOC" :key="link.id" :href="'#' + link.id"
                               class="text-sm text-gray-600 hover:text-blue-600 transition-colors block truncate"
                               :class="{ 'pl-4': link.level === 3 }">
                               {{ link.title }}
                            </a>
                        </nav>
                    </div>
                </aside>

            </div>
        </div>
    </div>

    <script>
        const { createApp, ref, computed, watch, nextTick } = Vue;
        const { createRouter, createWebHashHistory, useRoute } = VueRouter;

        // --- PAGE COMPONENT ---
        const PageView = {
            template: '<article class="prose prose-lg prose-slate max-w-none prose-pre:bg-[#282a36] prose-pre:p-0" v-html="content"></article>',
            props: ['data'],
            setup() {
                const route = useRoute();
                // Find content for current slug, or show 404
                const content = computed(() => {
                    const page = window.siteData?.pages[route.path];
                    return page ? page.content : "<h1>404 Not Found</h1>";
                });
                return { content };
            }
        };

        // --- APP SETUP ---
        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
                const route = useRoute();
                const mainScroll = ref(null);

                // Fetch the JSON database created by Go
                fetch('db.json')
                    .then(res => res.json())
                    .then(data => {
                        window.siteData = data; // Store globally for component access
                        menu.value = data.menu;
                        loading.value = false;
                    });

                // Get TOC for current page
                const currentTOC = computed(() => {
                    if (loading.value || !window.siteData) return [];
                    const page = window.siteData.pages[route.path];
                    return page ? page.toc : [];
                });

                // Scroll to top on navigation
                watch(() => route.path, () => {
                    if(mainScroll.value) mainScroll.value.scrollTop = 0;
                });

                return { loading, menu, currentTOC, mainScroll };
            }
        });

        // --- ROUTER SETUP ---
        // We use WebHashHistory (e.g. /#/about) because it works on ALL hosting (FTP/GitHub/etc) 
        // without needing server configuration.
        const router = createRouter({
            history: createWebHashHistory(),
            routes: [
                { path: '/:pathMatch(.*)*', component: PageView } // Catch-all route
            ]
        });

        app.use(router);
        app.mount('#app');
    </script>
</body>
</html>`
	os.WriteFile(path, []byte(html), 0644)
}