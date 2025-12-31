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
	fmt.Println("--- BUILDING VUE SPA (FIXED TOC) ---")

	if _, err := os.Stat(InputDir); os.IsNotExist(err) {
		fmt.Println("Error: 'content' folder missing.")
		return
	}
	os.RemoveAll(OutputDir)
	os.Mkdir(OutputDir, 0755)

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, highlighting.NewHighlighting(highlighting.WithStyle("dracula"))),
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
    <title>My Docs</title>
    <script src="https://cdn.tailwindcss.com?plugins=typography"></script>
    <script>tailwind.config = { theme: { extend: { colors: { dracula: '#282a36' } } } }</script>
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/vue-router@4/dist/vue-router.global.prod.js"></script>
    <style>
        pre { border-radius: 0.5rem; } 
        .fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
        .fade-enter-from, .fade-leave-to { opacity: 0; }
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: #f1f1f1; }
        ::-webkit-scrollbar-thumb { background: #ccc; borderRadius: 4px; }
        html { scroll-behavior: smooth; }
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
                            <a v-for="link in currentTOC" :key="link.id" 
                               @click.prevent="scrollToHeader(link.id)"
                               href="#"
                               class="text-sm text-gray-600 hover:text-blue-600 transition-colors block truncate cursor-pointer"
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

        const PageView = {
            template: '<article class="prose prose-lg prose-slate max-w-none prose-pre:bg-[#282a36] prose-pre:p-0" v-html="content"></article>',
            setup() {
                const route = useRoute();
                const content = computed(() => {
                    const page = window.siteData?.pages[route.path];
                    return page ? page.content : "<h1>404 Not Found</h1>";
                });
                return { content };
            }
        };

        const app = createApp({
            setup() {
                const loading = ref(true);
                const menu = ref([]);
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

                watch(() => route.path, () => {
                    if(mainScroll.value) mainScroll.value.scrollTop = 0;
                });

                // --- NEW FUNCTION TO HANDLE SCROLLING ---
                const scrollToHeader = (id) => {
                    // Find the header element inside the article
                    const element = document.getElementById(id);
                    if (element) {
                        element.scrollIntoView({ behavior: 'smooth', block: 'start' });
                    }
                };

                return { loading, menu, currentTOC, mainScroll, scrollToHeader };
            }
        });

        const router = createRouter({
            history: createWebHashHistory(),
            routes: [
                { path: '/:pathMatch(.*)*', component: PageView }
            ]
        });

        app.use(router);
        app.mount('#app');
    </script>
</body>
</html>`
	os.WriteFile(path, []byte(html), 0644)
}