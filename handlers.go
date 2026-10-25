package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"strings"

	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github-dark"),
			highlighting.WithGuessLanguage(true),
		),
	),
)

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, _ := a.Store.Featured(ctx, 3)
	testimonials, _ := a.Store.Testimonials(ctx)
	stats, _ := a.Store.Stats(ctx)

	a.render(w, r, "index.html", map[string]any{
		"Title":        "Marcel Nazare — Data Analyst",
		"Active":       "/",
		"Projects":     projects,
		"Testimonials": testimonials,
		"Stats":        stats,
	})
}

func (a *App) projectsList(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")

	list, err := a.Store.All(r.Context(), tag)
	if err != nil {
		log.Println("projects:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		a.render(w, r, "project_list", map[string]any{"Projects": list})
		return
	}

	tags, _ := a.Store.AllTags(r.Context())
	count, _ := a.Store.ProjectCount(r.Context())

	a.render(w, r, "projects.html", map[string]any{
		"Title":      "Projects — Marcel Nazare",
		"Active":     "/projects",
		"Projects":   list,
		"Tags":       tags,
		"ActiveTag":  tag,
		"ShowFilters": count >= 3,
	})
}

func (a *App) projectDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p, err := a.Store.BySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(p.BodyMarkdown), &buf); err != nil {
		log.Printf("markdown render %q: %v", slug, err)
	}
	p.BodyHTML = template.HTML(buf.String())

	a.render(w, r, "project.html", map[string]any{
		"Title":   p.Title + " — Marcel Nazare",
		"Active":  "/projects",
		"Project": p,
	})
}

func (a *App) about(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "about.html", map[string]any{
		"Title":  "About — Marcel Nazare",
		"Active": "/about",
	})
}

func (a *App) philosophy(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "philosophy.html", map[string]any{
		"Title":  "Philosophy — Marcel Nazare",
		"Active": "/philosophy",
	})
}

func (a *App) techstack(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "techstack.html", map[string]any{
		"Title":  "Tech Stack — Marcel Nazare",
		"Active": "/techstack",
	})
}

func (a *App) blogList(w http.ResponseWriter, r *http.Request) {
	posts, err := a.Store.Posts(r.Context(), 50, 0)
	if err != nil {
		log.Println("posts:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	a.render(w, r, "blog.html", map[string]any{
		"Title":  "Blog — Marcel Nazare",
		"Active": "/blog",
		"Posts":  posts,
	})
}

func (a *App) blogPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p, err := a.Store.PostBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(p.BodyMarkdown), &buf); err != nil {
		log.Printf("markdown render %q: %v", slug, err)
	}
	p.BodyHTML = template.HTML(buf.String())

	a.render(w, r, "blog_post.html", map[string]any{
		"Title":  p.Title + " — Marcel Nazare",
		"Active": "/blog",
		"Post":   p,
	})
}

func toolIcon(name string) template.HTML {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "python":
		return template.HTML(`<img src="https://upload.wikimedia.org/wikipedia/commons/c/c3/Python-logo-notext.svg" alt="Python" width="24" height="24">`)
	case "power bi", "powerbi":
		return template.HTML(`<img src="https://upload.wikimedia.org/wikipedia/commons/c/cf/New_Power_BI_Logo.svg" alt="Power BI" width="24" height="24">`)
	case "postgresql", "postgres":
		return template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#336791" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-label="PostgreSQL"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>`)
	case "tableau":
		return template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 54.4 54.2" aria-label="Tableau"><path fill="#eb9129" d="M25.3 35.8h3.8v10.5h-3.8zM21.9 44h10.5v-3.8H21.9zM25.3 7.8h3.8v10.5h-3.8zM21.9 16h10.5v-3.8H21.9zM12.4 25.1h3.8v10.5h-3.8zM9 33.3h10.5v-3.8H9zM38.1 25.1h3.8v10.5h-3.8zM34.7 33.3h10.5v-3.8H34.7z"/><path fill="#1c5ba3" d="M24.4 21.4h5.6v11.1h-5.6zM19.3 29.8h15.8v-5.6H19.3z"/><path fill="#4e79a7" d="M11.9 13.1h2.9v7.7h-2.9zM8.1 18.4H18.7v-2.9H8.1zM39.6 13.1h2.9v7.7h-2.9zM35.7 18.4h10.6v-2.9H35.7zM11.9 33.3h2.9v7.7h-2.9zM8.1 38.6H18.7v-2.9H8.1zM39.6 33.3h2.9v7.7h-2.9zM35.7 38.6h10.6v-2.9H35.7z"/><path fill="#e15759" d="M26.4 0h1.7v7.5h-1.7zM22.3 4.6h9.9V2.9h-9.9zM26.4 46.7h1.7v7.5h-1.7zM22.3 51.3h9.9v-1.7h-9.9zM46.9 23.3h1.7v7.5h-1.7zM5.8 23.3h1.7v7.5H5.8z"/></svg>`)
	}
	return template.HTML("")
}