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

type SiteData struct {
	Pages map[string]PageData `json:"pages"`
	Menu  []MenuItem          `json:"menu"`
}

type PageData struct {
	Title   string     `json:"title"`
	Content string     `json:"content"`
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
	fmt.Println("--- BUILDING SMOOTH FONT SPA ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	// Light Theme for Code (GitHub Style)
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, highlighting.NewHighlighting(highlighting.WithStyle("github"))),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithHardWraps(), html.WithUnsafe()),
	)

	files, _ := os.ReadDir(InputDir)
	site := SiteData{
		Pages: make(map[string]PageData),
		Menu:  []MenuItem{},
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".md" { continue }

		filename := strings.TrimSuffix(file.Name(), ".md")
		var slug string
		if filename == "index" {
			slug = "/"
		} else {
			slug = "/" + filename
		}

		srcPath := filepath.Join(InputDir, file.Name())
		source, _ := os.ReadFile(srcPath)
		
		title := strings.Title(strings.ReplaceAll(filename, "-", " "))
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

		var buf bytes.Buffer
		md.Renderer().Render(&buf, source, doc)

		site.Pages[slug] = PageData{
			Title:   title,
			Content: buf.String(),
			TOC:     toc,
		}
		
		site.Menu = append(site.Menu, MenuItem{Title: title, Slug: slug})
	}

	jsonBytes, _ := json.Marshal(site)
	os.WriteFile(filepath.Join(OutputDir, "db.json"), jsonBytes, 0644)
	
	writeAppShell(filepath.Join(OutputDir, "index.html"))
	fmt.Println("--- DONE ---")
}

func writeAppShell(path string) {
	const html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Docs</title>
    
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>
        tailwind.config = { 
            theme: { 
                extend: { 
                    fontFamily: { sans: ['Inter', 'sans-serif'] },
                    colors: { dracula: '#282a36' } 
                } 
            } 
        }
    </script>
    
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    
    <style>
        /* Light Theme Code Block Overrides */
        .prose pre { 
            background-color: #f6f8fa !important; 
            color: #24292e !important; 
            border: 1px solid #e1e4e8;
            border-radius: 6px;
            font-size: 0.9em;
        }
        .prose code { color: #d73a49; font-weight: 500; }
        .prose pre code { color: inherit; font-weight: normal; }
        
        /* Transitions */
        .fade-enter-active, .fade-leave-active { transition: opacity 0.15s ease; }
        .fade-enter-from, .fade-leave-to { opacity: 0; }
        
        ::-webkit-scrollbar { width: 6px; }
        ::-webkit-scrollbar-thumb { background: #cbd5e1; border-radius: 3px; }
        html { scroll-behavior: smooth; }
    </style>
</head>
<body class="bg-white text-slate-800 h-screen overflow-hidden flex antialiased">
    <div id="app" class="w-full h-full flex relative">

        <aside 
            class="bg-gray-50 border-r border-gray-200 w-64 flex-shrink-0 flex flex-col transition-all duration-300 absolute md:relative z-20 h-full"
            :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full md:w-0 md:overflow-hidden md:border-none'"
        >
            <div class="p-5 border-b border-gray-200 flex justify-between items-center bg-white">
                <span class="font-bold text-lg tracking-tight text-slate-900">Documentation</span>
                <button @click="toggleSidebar" class="md:hidden text-gray-500 hover:text-gray-900">✕</button>
            </div>
            
            <nav class="flex-1 overflow-y-auto p-3 space-y-1">
                <router-link v-for="item in menu" :key="item.slug" :to="item.slug" 
                    @click="closeMobileSidebar"
                    class="block px-3 py-2 rounded-md text-sm font-medium transition-colors duration-200"
                    :class="$route.path === item.slug ? 'bg-white text-blue-600 shadow-sm border border-gray-100 ring-1 ring-black/5' : 'text-slate-600 hover:bg-gray-100 hover:text-slate-900'">
                    {{ item.title }}
                </router-link>
            </nav>
        </aside>

        <div class="flex-1 flex flex-col h-full overflow-hidden w-full relative bg-white">
            
            <header class="h-14 border-b border-gray-100 flex items-center px-4 flex-shrink-0 bg-white/80 backdrop-blur-sm z-10">
                <button @click="toggleSidebar" class="p-2 -ml-2 text-gray-400 hover:text-gray-700 rounded-md hover:bg-gray-100 transition-colors">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path></svg>
                </button>
                <div class="ml-4 font-medium text-slate-400 text-sm select-none">/ {{ currentTitle }}</div>
            </header>

            <div v-if="loading" class="flex-1 flex items-center justify-center">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>

            <div v-else class="flex-1 overflow-hidden flex">
                <main class="flex-1 overflow-y-auto p-8 lg:p-12 scroll-smooth" ref="mainScroll">
                    <div class="max-w-3xl mx-auto">
                        <router-view v-slot="{ Component }">
                            <transition name="fade" mode="out-in">
                                <component :is="Component" />
                            </transition>
                        </router-view>
                    </div>
                </main>

                <aside class="hidden xl:block w-64 border-l border-gray-100 bg-white flex-shrink-0 overflow-y-auto p-8">
                    <div class="sticky top-0">
                        <h5 class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-4">On this page</h5>
                        <nav class="space-y-2 relative border-l border-gray-100 ml-1">
                            <a v-for="link in currentTOC" :key="link.id" 
                               @click.prevent="scrollToHeader(link.id)"
                               href="#"
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

        const PageView = {
            template: '<article class="prose prose-slate prose-lg max-w-none prose-headings:font-semibold prose-a:text-blue-600 prose-a:no-underline hover:prose-a:underline" v-html="content"></article>',
            setup() {
                const route = useRoute();
                const content = computed(() => {
                    const page = window.siteData?.pages[route.path];
                    return page ? page.content : "<h1 class='text-red-500'>404 Not Found</h1>";
                });
                return { content };
            }
        };

        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
                const sidebarOpen = ref(window.innerWidth > 1024);
                const route = useRoute();
                const mainScroll = ref(null);

                fetch('db.json')
                    .then(res => res.json())
                    .then(data => {
                        window.siteData = data;
                        menu.value = data.menu;
                        loading.value = false;
                    });

                const currentTOC = computed(() => {
                    if (loading.value || !window.siteData) return [];
                    const page = window.siteData.pages[route.path];
                    return page ? page.toc : [];
                });

                const currentTitle = computed(() => {
                    if (loading.value || !window.siteData) return '';
                    const page = window.siteData.pages[route.path];
                    return page ? page.title : '';
                });

                watch(() => route.path, () => {
                    if(mainScroll.value) mainScroll.value.scrollTop = 0;
                });

                const toggleSidebar = () => sidebarOpen.value = !sidebarOpen.value;
                const closeMobileSidebar = () => {
                    if(window.innerWidth < 1024) sidebarOpen.value = false;
                }

                const scrollToHeader = (id) => {
                    const element = document.getElementById(id);
                    if (element) {
                        element.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }
                };

                return { loading, menu, currentTOC, currentTitle, sidebarOpen, toggleSidebar, closeMobileSidebar, mainScroll, scrollToHeader };
            }
        });

        const router = createRouter({
            history: createWebHashHistory(),
            routes: [ { path: '/:pathMatch(.*)*', component: PageView } ]
        });

        app.use(router);
        app.mount('#app');
    </script>
</body>
</html>`
	os.WriteFile(path, []byte(html), 0644)
}