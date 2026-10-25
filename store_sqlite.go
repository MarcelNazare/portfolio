//go:build sqlite

package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func NewStore(ctx context.Context, url string) (*Store, error) {
	path := strings.TrimPrefix(url, "sqlite:")
	path = strings.TrimPrefix(path, "file:")
	if path == "" {
		path = "portfolio.db"
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() { s.DB.Close() }

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS projects (
    id            TEXT PRIMARY KEY,
    slug          TEXT UNIQUE NOT NULL,
    title         TEXT NOT NULL,
    tagline       TEXT NOT NULL DEFAULT '',
    problem       TEXT NOT NULL DEFAULT '',
    approach      TEXT NOT NULL DEFAULT '',
    impact        TEXT NOT NULL DEFAULT '',
    impact_metric TEXT,
    tools         TEXT NOT NULL DEFAULT '[]',
    repo_url      TEXT,
    demo_url      TEXT,
    image_url     TEXT,
    data_url      TEXT,
    resources     TEXT NOT NULL DEFAULT '[]',
    body_markdown TEXT NOT NULL DEFAULT '',
    featured      INTEGER NOT NULL DEFAULT 0,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    kind          TEXT NOT NULL DEFAULT 'project',
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS testimonials (
    id         TEXT PRIMARY KEY,
    author     TEXT NOT NULL,
    role       TEXT,
    quote      TEXT NOT NULL,
    source     TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS site_stats (
    label TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
    id            TEXT PRIMARY KEY,
    slug          TEXT UNIQUE NOT NULL,
    title         TEXT NOT NULL,
    excerpt       TEXT NOT NULL DEFAULT '',
    body_markdown TEXT NOT NULL DEFAULT '',
    image_url     TEXT,
    published_at  TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS posts_published_idx ON posts (published_at DESC);
CREATE INDEX IF NOT EXISTS projects_kind_idx    ON projects (kind);

INSERT OR IGNORE INTO site_stats (label, value) VALUES
    ('reporting_lag',       '14 days → 4 hrs'),
    ('parcels_reallocated', '1,200'),
    ('clinics_live',        '4');
`

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ─── SESSIONS ───────────────────────────────────────────────────────

func (s *Store) CreateSession(ctx context.Context, token string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token, expires_at) VALUES (?, ?)`,
		token, expires.Format(time.RFC3339))
	return err
}

func (s *Store) ValidateSession(ctx context.Context, token string) bool {
	var expiresStr string
	err := s.DB.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&expiresStr)
	if err != nil {
		return false
	}
	expires, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil || time.Now().After(expires) {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
		return false
	}
	return true
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// ─── PROJECTS ───────────────────────────────────────────────────────

const projectCols = `id, slug, title, tagline, problem, approach, impact,
  COALESCE(impact_metric,''), tools,
  COALESCE(repo_url,''), COALESCE(demo_url,''), COALESCE(image_url,''),
  featured, sort_order, created_at,
  COALESCE(body_markdown,''), COALESCE(resources,'[]'),
  COALESCE(data_url,''), COALESCE(kind,'project')`

func scanProject(p *Project, scan func(...any) error) error {
	var toolsJSON, resourcesJSON, createdAt string
	err := scan(
		&p.ID, &p.Slug, &p.Title, &p.Tagline, &p.Problem, &p.Approach, &p.Impact,
		&p.ImpactMetric, &toolsJSON, &p.RepoURL, &p.DemoURL, &p.ImageURL,
		&p.Featured, &p.SortOrder, &createdAt,
		&p.BodyMarkdown, &resourcesJSON, &p.DataURL, &p.Kind,
	)
	if err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(toolsJSON), &p.Tools)
	_ = json.Unmarshal([]byte(resourcesJSON), &p.Resources)
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		p.CreatedAt = t
	}
	return nil
}

func (s *Store) Featured(ctx context.Context, limit int) ([]Project, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT `+projectCols+`
		FROM projects WHERE featured = 1
		ORDER BY sort_order, created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := scanProject(&p, rows.Scan); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) All(ctx context.Context, tag string) ([]Project, error) {
	var (
		q    string
		args []any
	)
	if tag != "" {
		q = `SELECT ` + projectCols + ` FROM projects
		     WHERE EXISTS (SELECT 1 FROM json_each(tools) WHERE value = ?)
		     ORDER BY sort_order, created_at DESC`
		args = []any{tag}
	} else {
		q = `SELECT ` + projectCols + ` FROM projects ORDER BY sort_order, created_at DESC`
	}

	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := scanProject(&p, rows.Scan); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) BySlug(ctx context.Context, slug string) (*Project, error) {
	var p Project
	err := scanProject(&p, s.DB.QueryRowContext(ctx,
		`SELECT `+projectCols+` FROM projects WHERE slug = ?`, slug).Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ByID(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := scanProject(&p, s.DB.QueryRowContext(ctx,
		`SELECT `+projectCols+` FROM projects WHERE id = ?`, id).Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) AllTags(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT DISTINCT value FROM projects, json_each(projects.tools) ORDER BY value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ProjectCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM projects`).Scan(&n)
	return n, err
}

func (s *Store) CreateProject(ctx context.Context, p Project) error {
	tools, _ := json.Marshal(p.Tools)
	res, _ := json.Marshal(p.Resources)
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO projects
			(id, slug, title, tagline, problem, approach, impact, impact_metric,
			 tools, repo_url, demo_url, image_url, featured,
			 body_markdown, data_url, kind, resources, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		newID(), p.Slug, p.Title, p.Tagline, p.Problem, p.Approach, p.Impact,
		p.ImpactMetric, string(tools), p.RepoURL, p.DemoURL, p.ImageURL, p.Featured,
		p.BodyMarkdown, p.DataURL, p.Kind, string(res),
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) UpdateProject(ctx context.Context, p Project) error {
	tools, _ := json.Marshal(p.Tools)
	res, _ := json.Marshal(p.Resources)
	_, err := s.DB.ExecContext(ctx, `
		UPDATE projects SET
			slug=?, title=?, tagline=?, problem=?, approach=?, impact=?,
			impact_metric=?, tools=?, repo_url=?, demo_url=?, image_url=?,
			featured=?, body_markdown=?, data_url=?, kind=?, resources=?
		WHERE id=?`,
		p.Slug, p.Title, p.Tagline, p.Problem, p.Approach, p.Impact,
		p.ImpactMetric, string(tools), p.RepoURL, p.DemoURL, p.ImageURL,
		p.Featured, p.BodyMarkdown, p.DataURL, p.Kind, string(res), p.ID)
	return err
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	return err
}

// ─── TESTIMONIALS & STATS ───────────────────────────────────────────

func (s *Store) Testimonials(ctx context.Context) ([]Testimonial, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials ORDER BY created_at DESC LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Testimonial
	for rows.Next() {
		var t Testimonial
		var createdAt string
		if err := rows.Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &createdAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			t.CreatedAt = parsed
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) AllTestimonials(ctx context.Context) ([]Testimonial, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Testimonial
	for rows.Next() {
		var t Testimonial
		var createdAt string
		if err := rows.Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &createdAt); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			t.CreatedAt = parsed
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TestimonialByID(ctx context.Context, id string) (*Testimonial, error) {
	var t Testimonial
	var createdAt string
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials WHERE id = ?`, id).
		Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &createdAt)
	if err != nil {
		return nil, err
	}
	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		t.CreatedAt = parsed
	}
	return &t, nil
}

func (s *Store) CreateTestimonial(ctx context.Context, t Testimonial) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO testimonials (id, author, role, quote, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), t.Author, t.Role, t.Quote, t.Source, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) UpdateTestimonial(ctx context.Context, t Testimonial) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE testimonials SET author=?, role=?, quote=?, source=? WHERE id=?`,
		t.Author, t.Role, t.Quote, t.Source, t.ID)
	return err
}

func (s *Store) DeleteTestimonial(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM testimonials WHERE id = ?`, id)
	return err
}

func (s *Store) TestimonialCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM testimonials`).Scan(&n)
	return n, err
}

func (s *Store) Stats(ctx context.Context) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT label, value FROM site_stats`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

// ─── POSTS ──────────────────────────────────────────────────────────

const postCols = `id, slug, title, excerpt,
  COALESCE(image_url,''), published_at, body_markdown`

func scanPost(p *Post, scan func(...any) error) error {
	var publishedAt string
	err := scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt,
		&p.ImageURL, &publishedAt, &p.BodyMarkdown)
	if err != nil {
		return err
	}
	if t, err := time.Parse(time.RFC3339, publishedAt); err == nil {
		p.PublishedAt = t
	}
	return nil
}

func (s *Store) Posts(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT `+postCols+`
		FROM posts
		ORDER BY published_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Post
	for rows.Next() {
		var p Post
		if err := scanPost(&p, rows.Scan); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PostBySlug(ctx context.Context, slug string) (*Post, error) {
	var p Post
	err := scanPost(&p, s.DB.QueryRowContext(ctx,
		`SELECT `+postCols+` FROM posts WHERE slug = ?`, slug).Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) PostByID(ctx context.Context, id string) (*Post, error) {
	var p Post
	err := scanPost(&p, s.DB.QueryRowContext(ctx,
		`SELECT `+postCols+` FROM posts WHERE id = ?`, id).Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) PostCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM posts`).Scan(&n)
	return n, err
}

func (s *Store) CreatePost(ctx context.Context, p Post) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO posts (id, slug, title, excerpt, body_markdown, image_url, published_at, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		newID(), p.Slug, p.Title, p.Excerpt, p.BodyMarkdown, p.ImageURL,
		p.PublishedAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) UpdatePost(ctx context.Context, p Post) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE posts SET
			slug=?, title=?, excerpt=?, body_markdown=?,
			image_url=?, published_at=?
		WHERE id=?`,
		p.Slug, p.Title, p.Excerpt, p.BodyMarkdown,
		p.ImageURL, p.PublishedAt.Format(time.RFC3339), p.ID)
	return err
}

func (s *Store) DeletePost(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, id)
	return err
}