package main

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

//go:embed templates static
var FS embed.FS

type App struct {
	Store       *Store
	Tmpl        *template.Template
	RateLimiter *RateLimiter
}

func main() {
	_ = godotenv.Load()
	hash := os.Getenv("ADMIN_PASSWORD_HASH")
	if len(hash) > 10 {
    	log.Printf("✅ LOADED HASH: %s... (Length: %d)", hash[:10], len(hash))
	} else {
    	log.Println("❌ WARNING: ADMIN_PASSWORD_HASH is missing or invalid")
	}
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	store, err := NewStore(ctx, dbURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	funcs := template.FuncMap{
		"toolIcon": toolIcon,
		"join":     strings.Join,
	}

	tmpl := template.Must(template.New("").Funcs(funcs).ParseFS(FS,
		"templates/*.html",
		"templates/partials/*.html",
		"templates/admin/*.html",
	))

	app := &App{
		Store:       store,
		Tmpl:        tmpl,
		RateLimiter: NewRateLimiter(),
	}

	mux := http.NewServeMux()

	// Public site
	mux.HandleFunc("GET /{$}", app.home)
	mux.HandleFunc("GET /projects", app.projectsList)
	mux.HandleFunc("GET /projects/{slug}", app.projectDetail)
	mux.HandleFunc("GET /about", app.about)
	mux.HandleFunc("GET /philosophy", app.philosophy)
	mux.HandleFunc("GET /techstack", app.techstack)
	mux.HandleFunc("GET /blog", app.blogList)
	mux.HandleFunc("GET /blog/{slug}", app.blogPost)

	mux.Handle("GET /static/", http.FileServer(http.FS(FS)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Admin Auth
	mux.HandleFunc("GET /admin/login", app.adminLoginPage)
	mux.HandleFunc("POST /admin/login", app.loginRateLimitMiddleware(app.adminLoginSubmit))
	mux.HandleFunc("POST /admin/logout", app.adminLogout)

	// Protected Admin Routes
	mux.HandleFunc("GET /admin", app.requireAuth(app.adminDashboard))

	mux.HandleFunc("GET /admin/projects", app.requireAuth(app.adminProjectsList))
	mux.HandleFunc("GET /admin/projects/new", app.requireAuth(app.adminProjectNew))
	mux.HandleFunc("POST /admin/projects", app.requireAuth(app.adminProjectSave))
	mux.HandleFunc("GET /admin/projects/{id}/edit", app.requireAuth(app.adminProjectEdit))
	mux.HandleFunc("POST /admin/projects/{id}", app.requireAuth(app.adminProjectSave))
	mux.HandleFunc("DELETE /admin/projects/{id}", app.requireAuth(app.adminProjectDelete))

	mux.HandleFunc("GET /admin/posts", app.requireAuth(app.adminPostsList))
	mux.HandleFunc("GET /admin/posts/new", app.requireAuth(app.adminPostNew))
	mux.HandleFunc("POST /admin/posts", app.requireAuth(app.adminPostSave))
	mux.HandleFunc("GET /admin/posts/{id}/edit", app.requireAuth(app.adminPostEdit))
	mux.HandleFunc("POST /admin/posts/{id}", app.requireAuth(app.adminPostSave))
	mux.HandleFunc("DELETE /admin/posts/{id}", app.requireAuth(app.adminPostDelete))

	mux.HandleFunc("GET /admin/testimonials", app.requireAuth(app.adminTestimonialsList))
	mux.HandleFunc("GET /admin/testimonials/new", app.requireAuth(app.adminTestimonialNew))
	mux.HandleFunc("POST /admin/testimonials", app.requireAuth(app.adminTestimonialSave))
	mux.HandleFunc("GET /admin/testimonials/{id}/edit", app.requireAuth(app.adminTestimonialEdit))
	mux.HandleFunc("POST /admin/testimonials/{id}", app.requireAuth(app.adminTestimonialSave))
	mux.HandleFunc("DELETE /admin/testimonials/{id}", app.requireAuth(app.adminTestimonialDelete))

	mux.HandleFunc("POST /admin/upload", app.requireAuth(app.adminUpload))

	// Apply global CSRF middleware
	handler := app.csrfMiddleware(stripTrailingSlash(mux))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func stripTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || p == "" || strings.HasPrefix(p, "/static") || strings.HasPrefix(p, "/uploads") {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasSuffix(p, "/") {
			u := &url.URL{Path: strings.TrimRight(p, "/"), RawQuery: r.URL.RawQuery}
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	if data == nil {
		data = map[string]any{}
	}
	if m, ok := data.(map[string]any); ok {
		if _, exists := m["CSRF"]; !exists {
			m["CSRF"] = r.Header.Get("X-CSRF-Token-Context")
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.Tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %q: %v", name, err)
	}
}