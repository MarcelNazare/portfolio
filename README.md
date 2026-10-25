# Portfolio — Go + HTMX + Neon

Personal portfolio for Marcel Nazare. Full CMS: projects, case studies, blog posts.

## Stack

- **Go 1.22** — single-binary HTTP server
- **HTMX 2.0** — server-driven interactivity (no JS framework)
- **Neon Postgres** — content store
- **Tailwind CSS** — compiled once, no build step at runtime
- **goldmark + Chroma** — Markdown + syntax highlighting

## Setup

### 1. Create a Neon database

1. Sign up at https://neon.tech
2. Create a project, copy the **pooled** connection string
3. In the SQL editor, paste `migrations/001_init.sql` and run it






# First time
make setup

# Local testing with SQLite (no Neon needed)
make dev-sqlite

# Production / real DB
export DATABASE_URL='postgresql://...'
export ADMIN_PASSWORD='your-password'
make dev

# Rebuild CSS only (e.g. after editing templates)
make css

# Deploy
make deploy


### 2. Configure environment

```bash
cp .env.example .env
# Edit .env with your DATABASE_URL and a strong ADMIN_PASSWORD
