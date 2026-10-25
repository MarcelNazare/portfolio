//go:build !sqlite

package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func NewStore(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

const projectCols = `id, slug, title, tagline, problem, approach, impact,
  COALESCE(impact_metric,''), tools,
  COALESCE(repo_url,''), COALESCE(demo_url,''), COALESCE(image_url,''),
  featured, sort_order, created_at,
  COALESCE(body_markdown,''), COALESCE(resources,'[]'::jsonb), COALESCE(data_url,''),
  COALESCE(kind,'project')`

func scanProject(p *Project, s interface{ Scan(...any) error }) error {
	var resourcesJSON []byte
	err := s.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Tagline, &p.Problem, &p.Approach, &p.Impact,
		&p.ImpactMetric, &p.Tools, &p.RepoURL, &p.DemoURL, &p.ImageURL,
		&p.Featured, &p.SortOrder, &p.CreatedAt,
		&p.BodyMarkdown, &resourcesJSON, &p.DataURL, &p.Kind,
	)
	if err != nil {
		return err
	}
	if len(resourcesJSON) > 0 {
		_ = json.Unmarshal(resourcesJSON, &p.Resources)
	}
	return nil
}

func (s *Store) Featured(ctx context.Context, limit int) ([]Project, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+projectCols+`
		FROM projects WHERE featured = true
		ORDER BY sort_order, created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := scanProject(&p, rows); err != nil {
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
		q = `SELECT ` + projectCols + ` FROM projects WHERE $1 = ANY(tools)
		     ORDER BY sort_order, created_at DESC`
		args = []any{tag}
	} else {
		q = `SELECT ` + projectCols + ` FROM projects
		     ORDER BY sort_order, created_at DESC`
	}

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := scanProject(&p, rows); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) BySlug(ctx context.Context, slug string) (*Project, error) {
	var p Project
	err := scanProject(&p, s.Pool.QueryRow(ctx,
		`SELECT `+projectCols+` FROM projects WHERE slug = $1`, slug))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ByID(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := scanProject(&p, s.Pool.QueryRow(ctx,
		`SELECT `+projectCols+` FROM projects WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) Testimonials(ctx context.Context) ([]Testimonial, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials ORDER BY created_at DESC LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Testimonial
	for rows.Next() {
		var t Testimonial
		if err := rows.Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) AllTestimonials(ctx context.Context) ([]Testimonial, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Testimonial
	for rows.Next() {
		var t Testimonial
		if err := rows.Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TestimonialByID(ctx context.Context, id string) (*Testimonial, error) {
	var t Testimonial
	err := s.Pool.QueryRow(ctx,
		`SELECT id, author, COALESCE(role,''), quote, COALESCE(source,''), created_at
		 FROM testimonials WHERE id = $1`, id).
		Scan(&t.ID, &t.Author, &t.Role, &t.Quote, &t.Source, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateTestimonial(ctx context.Context, t Testimonial) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO testimonials (author, role, quote, source) VALUES ($1, $2, $3, $4)`,
		t.Author, t.Role, t.Quote, t.Source)
	return err
}

func (s *Store) UpdateTestimonial(ctx context.Context, t Testimonial) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE testimonials SET author=$2, role=$3, quote=$4, source=$5 WHERE id=$1`,
		t.ID, t.Author, t.Role, t.Quote, t.Source)
	return err
}

func (s *Store) DeleteTestimonial(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM testimonials WHERE id = $1`, id)
	return err
}

func (s *Store) TestimonialCount(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM testimonials`).Scan(&n)
	return n, err
}

func (s *Store) AllTags(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT DISTINCT unnest(tools) AS t FROM projects ORDER BY t`)
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

func (s *Store) Stats(ctx context.Context) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT label, value FROM site_stats`)
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

func (s *Store) ProjectCount(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&n)
	return n, err
}

func (s *Store) CreateProject(ctx context.Context, p Project) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO projects
			(slug, title, tagline, problem, approach, impact, impact_metric,
			 tools, repo_url, demo_url, image_url, featured,
			 body_markdown, data_url, kind)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		p.Slug, p.Title, p.Tagline, p.Problem, p.Approach, p.Impact, p.ImpactMetric,
		p.Tools, p.RepoURL, p.DemoURL, p.ImageURL, p.Featured,
		p.BodyMarkdown, p.DataURL, p.Kind)
	return err
}

func (s *Store) UpdateProject(ctx context.Context, p Project) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE projects SET
			slug=$2, title=$3, tagline=$4, problem=$5, approach=$6, impact=$7,
			impact_metric=$8, tools=$9, repo_url=$10, demo_url=$11, image_url=$12,
			featured=$13, body_markdown=$14, data_url=$15, kind=$16
		WHERE id=$1`,
		p.ID, p.Slug, p.Title, p.Tagline, p.Problem, p.Approach, p.Impact,
		p.ImpactMetric, p.Tools, p.RepoURL, p.DemoURL, p.ImageURL,
		p.Featured, p.BodyMarkdown, p.DataURL, p.Kind)
	return err
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

const postCols = `id, slug, title, excerpt,
  COALESCE(image_url,''), published_at, body_markdown`

func scanPost(p *Post, s interface{ Scan(...any) error }) error {
	return s.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt,
		&p.ImageURL, &p.PublishedAt, &p.BodyMarkdown)
}

func (s *Store) Posts(ctx context.Context, limit, offset int) ([]Post, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+postCols+`
		FROM posts
		ORDER BY published_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Post
	for rows.Next() {
		var p Post
		if err := scanPost(&p, rows); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PostBySlug(ctx context.Context, slug string) (*Post, error) {
	var p Post
	err := scanPost(&p, s.Pool.QueryRow(ctx,
		`SELECT `+postCols+` FROM posts WHERE slug = $1`, slug))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) PostByID(ctx context.Context, id string) (*Post, error) {
	var p Post
	err := scanPost(&p, s.Pool.QueryRow(ctx,
		`SELECT `+postCols+` FROM posts WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) PostCount(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM posts`).Scan(&n)
	return n, err
}

func (s *Store) CreatePost(ctx context.Context, p Post) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO posts (slug, title, excerpt, body_markdown, image_url, published_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		p.Slug, p.Title, p.Excerpt, p.BodyMarkdown, p.ImageURL, p.PublishedAt)
	return err
}

func (s *Store) UpdatePost(ctx context.Context, p Post) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE posts SET
			slug=$2, title=$3, excerpt=$4, body_markdown=$5,
			image_url=$6, published_at=$7
		WHERE id=$1`,
		p.ID, p.Slug, p.Title, p.Excerpt, p.BodyMarkdown, p.ImageURL, p.PublishedAt)
	return err
}

func (s *Store) DeletePost(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, token string, expires time.Time) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO sessions (token, expires_at) VALUES ($1, $2)`, token, expires)
	return err
}

func (s *Store) ValidateSession(ctx context.Context, token string) bool {
	var expires time.Time
	err := s.Pool.QueryRow(ctx, `SELECT expires_at FROM sessions WHERE token = $1`, token).Scan(&expires)
	if err != nil {
		return false
	}
	if time.Now().After(expires) {
		_, _ = s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
		return false
	}
	return true
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}