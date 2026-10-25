package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ─── AUTH ───────────────────────────────────────────────────────────

func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("admin_session")
		if err != nil || !a.Store.ValidateSession(r.Context(), cookie.Value) {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/admin/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *App) adminLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil && a.Store.ValidateSession(r.Context(), cookie.Value) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	a.render(w, r, "admin_login.html", map[string]any{})
}

func (a *App) adminLoginSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	password := r.PostFormValue("password")
	hash := os.Getenv("ADMIN_PASSWORD_HASH")

	if hash == "" {
		a.render(w, r, "admin_login.html", map[string]any{"Error": "Server error: ADMIN_PASSWORD_HASH not configured."})
		return
	}

	// Verify Bcrypt Hash
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		a.render(w, r, "admin_login.html", map[string]any{"Error": "Incorrect password."})
		return
	}

	// Generate secure session token
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	expires := time.Now().Add(24 * time.Hour)

	// Save to DB
	if err := a.Store.CreateSession(r.Context(), token, expires); err != nil {
		log.Println("session save error:", err)
		a.render(w, r, "admin_login.html", map[string]any{"Error": "Database error. Please try again."})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *App) adminLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil {
		_ = a.Store.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// ─── DASHBOARD ──────────────────────────────────────────────────────

func (a *App) adminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pc, _ := a.Store.ProjectCount(ctx)
	bc, _ := a.Store.PostCount(ctx)
	tc, _ := a.Store.TestimonialCount(ctx)

	projects, _ := a.Store.All(ctx, "")
	if len(projects) > 5 {
		projects = projects[:5]
	}
	posts, _ := a.Store.Posts(ctx, 5, 0)

	a.render(w, r, "admin_dashboard.html", map[string]any{
		"Title":            "Admin — Dashboard",
		"Active":           "/admin",
		"ProjectCount":     pc,
		"PostCount":        bc,
		"TestimonialCount": tc,
		"RecentProjects":   projects,
		"RecentPosts":      posts,
	})
}

// ─── PROJECTS ───────────────────────────────────────────────────────

func (a *App) adminProjectsList(w http.ResponseWriter, r *http.Request) {
	list, _ := a.Store.All(r.Context(), "")
	a.render(w, r, "admin_projects_list.html", map[string]any{
		"Title":    "Admin — Projects",
		"Active":   "/admin/projects",
		"Projects": list,
	})
}

func (a *App) adminProjectNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_project_edit.html", map[string]any{
		"Title":   "Admin — New Project",
		"Active":  "/admin/projects",
		"Project": &Project{Kind: "project"},
		"IsNew":   true,
	})
}

func (a *App) adminProjectEdit(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "admin_project_edit.html", map[string]any{
		"Title":   "Admin — Edit " + p.Title,
		"Active":  "/admin/projects",
		"Project": p,
		"IsNew":   false,
	})
}

func (a *App) adminProjectSave(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	isNew := r.PathValue("id") == ""

	p := &Project{
		Title:        strings.TrimSpace(r.PostFormValue("title")),
		Slug:         slugify(r.PostFormValue("slug")),
		Tagline:      r.PostFormValue("tagline"),
		Problem:      r.PostFormValue("problem"),
		Approach:     r.PostFormValue("approach"),
		Impact:       r.PostFormValue("impact"),
		ImpactMetric: r.PostFormValue("impact_metric"),
		BodyMarkdown: r.PostFormValue("body_markdown"),
		RepoURL:      r.PostFormValue("repo_url"),
		DemoURL:      r.PostFormValue("demo_url"),
		ImageURL:     r.PostFormValue("image_url"),
		DataURL:      r.PostFormValue("data_url"),
		Featured:     r.PostFormValue("featured") == "on",
		Kind:         r.PostFormValue("kind"),
	}
	if p.Kind == "" {
		p.Kind = "project"
	}
	if raw := r.PostFormValue("tools"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				p.Tools = append(p.Tools, t)
			}
		}
	}

	if p.Title == "" || p.Slug == "" {
		a.render(w, r, "admin_flash", map[string]any{"Error": "Title and slug are required."})
		return
	}

	if !isNew {
		p.ID = r.PathValue("id")
	}
	var err error
	if isNew {
		err = a.Store.CreateProject(r.Context(), *p)
	} else {
		err = a.Store.UpdateProject(r.Context(), *p)
	}
	if err != nil {
		log.Println("save project:", err)
		a.render(w, r, "admin_flash", map[string]any{"Error": "Save failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Redirect", "/admin/projects")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) adminProjectDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteProject(r.Context(), r.PathValue("id")); err != nil {
		log.Println("delete project:", err)
		http.Error(w, "delete failed", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── POSTS ──────────────────────────────────────────────────────────

func (a *App) adminPostsList(w http.ResponseWriter, r *http.Request) {
	list, _ := a.Store.Posts(r.Context(), 100, 0)
	a.render(w, r, "admin_posts_list.html", map[string]any{
		"Title":  "Admin — Posts",
		"Active": "/admin/posts",
		"Posts":  list,
	})
}

func (a *App) adminPostNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_post_edit.html", map[string]any{
		"Title":  "Admin — New Post",
		"Active": "/admin/posts",
		"Post":   &Post{PublishedAt: time.Now()},
		"IsNew":  true,
	})
}

func (a *App) adminPostEdit(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.PostByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "admin_post_edit.html", map[string]any{
		"Title":  "Admin — Edit " + p.Title,
		"Active": "/admin/posts",
		"Post":   p,
		"IsNew":  false,
	})
}

func (a *App) adminPostSave(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	isNew := r.PathValue("id") == ""

	p := &Post{
		Slug:         slugify(r.PostFormValue("slug")),
		Title:        strings.TrimSpace(r.PostFormValue("title")),
		Excerpt:      r.PostFormValue("excerpt"),
		BodyMarkdown: r.PostFormValue("body_markdown"),
		ImageURL:     r.PostFormValue("image_url"),
	}

	if dt := r.PostFormValue("published_at"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil {
			p.PublishedAt = t
		} else {
			p.PublishedAt = time.Now()
		}
	} else {
		p.PublishedAt = time.Now()
	}

	if p.Title == "" || p.Slug == "" || p.Excerpt == "" {
		a.render(w, r, "admin_flash", map[string]any{"Error": "Title, slug, and excerpt are required."})
		return
	}

	if !isNew {
		p.ID = r.PathValue("id")
	}
	var err error
	if isNew {
		err = a.Store.CreatePost(r.Context(), *p)
	} else {
		err = a.Store.UpdatePost(r.Context(), *p)
	}
	if err != nil {
		log.Println("save post:", err)
		a.render(w, r, "admin_flash", map[string]any{"Error": "Save failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Redirect", "/admin/posts")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) adminPostDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeletePost(r.Context(), r.PathValue("id")); err != nil {
		log.Println("delete post:", err)
		http.Error(w, "delete failed", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── TESTIMONIALS ───────────────────────────────────────────────────

func (a *App) adminTestimonialsList(w http.ResponseWriter, r *http.Request) {
	list, _ := a.Store.AllTestimonials(r.Context())
	a.render(w, r, "admin_testimonials_list.html", map[string]any{
		"Title":        "Admin — Testimonials",
		"Active":       "/admin/testimonials",
		"Testimonials": list,
	})
}

func (a *App) adminTestimonialNew(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, "admin_testimonial_edit.html", map[string]any{
		"Title":       "Admin — New Testimonial",
		"Active":      "/admin/testimonials",
		"Testimonial": &Testimonial{},
		"IsNew":       true,
	})
}

func (a *App) adminTestimonialEdit(w http.ResponseWriter, r *http.Request) {
	t, err := a.Store.TestimonialByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.render(w, r, "admin_testimonial_edit.html", map[string]any{
		"Title":       "Admin — Edit Testimonial",
		"Active":      "/admin/testimonials",
		"Testimonial": t,
		"IsNew":       false,
	})
}

func (a *App) adminTestimonialSave(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	isNew := r.PathValue("id") == ""

	t := &Testimonial{
		Author: strings.TrimSpace(r.PostFormValue("author")),
		Role:   r.PostFormValue("role"),
		Quote:  r.PostFormValue("quote"),
		Source: r.PostFormValue("source"),
	}

	if t.Author == "" || t.Quote == "" {
		a.render(w, r, "admin_flash", map[string]any{"Error": "Author and Quote are required."})
		return
	}

	if !isNew {
		t.ID = r.PathValue("id")
	}

	var err error
	if isNew {
		err = a.Store.CreateTestimonial(r.Context(), *t)
	} else {
		err = a.Store.UpdateTestimonial(r.Context(), *t)
	}

	if err != nil {
		log.Println("save testimonial:", err)
		a.render(w, r, "admin_flash", map[string]any{"Error": "Save failed: " + err.Error()})
		return
	}

	w.Header().Set("HX-Redirect", "/admin/testimonials")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) adminTestimonialDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteTestimonial(r.Context(), r.PathValue("id")); err != nil {
		log.Println("delete testimonial:", err)
		http.Error(w, "delete failed", 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── UPLOADS ────────────────────────────────────────────────────────

func (a *App) adminUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "bad multipart", 400)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "no file", 400)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
	default:
		http.Error(w, "unsupported file type", 400)
		return
	}
	name := randHex(12) + ext

	if err := os.MkdirAll("uploads", 0o755); err != nil {
		http.Error(w, "storage error", 500)
		return
	}
	out, err := os.Create(filepath.Join("uploads", name))
	if err != nil {
		http.Error(w, "write error", 500)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "copy error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"data":{"filePath":%q}}`, "/uploads/"+name)
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}