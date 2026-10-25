CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─── PROJECTS ───────────────────────────────────────────
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    tagline TEXT NOT NULL DEFAULT '',
    problem TEXT NOT NULL DEFAULT '',
    approach TEXT NOT NULL DEFAULT '',
    impact TEXT NOT NULL DEFAULT '',
    impact_metric TEXT,
    tools TEXT[] NOT NULL DEFAULT '{}',
    repo_url TEXT, demo_url TEXT, image_url TEXT,
    data_url TEXT,
    resources JSONB NOT NULL DEFAULT '[]'::jsonb,
    body_markdown TEXT NOT NULL DEFAULT '',
    featured BOOLEAN NOT NULL DEFAULT false,
    sort_order INT NOT NULL DEFAULT 0,
    kind TEXT NOT NULL DEFAULT 'project',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE projects ADD COLUMN IF NOT EXISTS impact_metric TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS body_markdown TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS resources JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS data_url TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'project';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'projects_kind_check'
    ) THEN
        ALTER TABLE projects ADD CONSTRAINT projects_kind_check
            CHECK (kind IN ('project','case_study'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS projects_kind_idx ON projects (kind);

-- ─── TESTIMONIALS ───────────────────────────────────────
CREATE TABLE IF NOT EXISTS testimonials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author TEXT NOT NULL,
    role TEXT,
    quote TEXT NOT NULL,
    source TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── STATS ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS site_stats (
    label TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- ─── POSTS ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    excerpt TEXT NOT NULL DEFAULT '',
    body_markdown TEXT NOT NULL DEFAULT '',
    image_url TEXT,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS posts_published_idx ON posts (published_at DESC);

-- ─── SEED: STATS ────────────────────────────────────────
INSERT INTO site_stats (label, value) VALUES
    ('reporting_lag', '14 days → 4 hrs'),
    ('parcels_reallocated', '1,200'),
    ('clinics_live', '4')
ON CONFLICT (label) DO NOTHING;

-- ─── SEED: TESTIMONIALS ─────────────────────────────────
INSERT INTO testimonials (author, role, quote, source)
SELECT * FROM (VALUES
    ('Amara O.', 'Program Director, Regional NGO',
     'Marcel translated a messy field-data problem into a dashboard our field team actually uses. His communication was excellent throughout.',
     'Upwork · 5.0★'),
    ('David K.', 'Data Lead, Development Program',
     'The gap analysis was the single most useful piece of analysis we received last year. It changed how we allocate routes.',
     'Client reference')
) AS v(author, role, quote, source)
WHERE NOT EXISTS (SELECT 1 FROM testimonials LIMIT 1);

-- ─── SEED: SUPERSTORE PROJECT ───────────────────────────
INSERT INTO projects
    (slug, title, tagline, problem, approach, impact, impact_metric,
     tools, repo_url, image_url, featured, sort_order, kind, created_at)
VALUES (
    'superstore-sales-project',
    'Superstore Sales Project',
    'End-to-end analysis of US retail sales — finding the profitability gaps regional reports missed.',
    'US retail sales data showed revenue growth but flat profitability across regions. Management could not explain why certain regions with strong sales were underperforming on margin.',
    'Built an end-to-end pipeline: Python for cleaning and feature engineering, PostgreSQL for the analytical warehouse, then Power BI and Tableau dashboards for two audiences — executives (weekly snapshot) and regional managers (drilldown).',
    'Uncovered key profitability gaps across regions — discount-led volume masked negative-margin product categories. Delivered actionable pricing and bundling recommendations adopted by sales strategy.',
    '4 regional gaps found',
    ARRAY['Python','PostgreSQL','Power BI','Tableau'],
    'https://github.com/MarcelNazare/Superstore-Sales-Project',
    '/static/superstore-dashboard.webp',
    true, 1, 'case_study',
    '2024-06-19 00:00:00+00'::timestamptz
)
ON CONFLICT (slug) DO NOTHING;

-- ─── SEED: BLOG POSTS (metadata only) ───────────────────
INSERT INTO posts (slug, title, excerpt, image_url, published_at) VALUES
    ('tanzania-tourism-expenditure-classifier',
     'Machine Learning in Practice: My Journey Building the Tanzania Tourism Expenditure Classifier',
     'From Survey Responses to Spending Bands. The Tanzania Tourism Expenditure Classifier is a hackathon project that predicts how much a visitor will spend in Tanzania. Using 24,675 national survey responses, CatBoost, LightGBM and a reproducible notebook pipeline, it turns raw data into calibrated probabilities across six expenditure bands, scored on log loss.',
     '/static/blog/tanzania-tourism.webp',
     '2024-09-21 00:00:00+00'),

    ('automating-data-viz-exports-python-playwright',
     'Automating Data Viz Exports with Python & Playwright',
     'Learn how to automate data visualization exports using Python and Playwright. Convert interactive HTML charts into high-quality JPG images after JavaScript rendering finishes.',
     '/static/blog/html-to-jpg.webp',
     '2024-04-29 00:00:00+00'),

    ('data-engineering',
     'Data Engineering: The Invisible Backbone of Innovation',
     'The Blueprint Behind the Intelligence. Behind every successful AI model and business dashboard is a robust infrastructure that most people never see.',
     '/static/blog/data-engineering.webp',
     '2024-04-17 00:00:00+00'),

    ('data-engineering-in-practice',
     'Data Engineering in Practice: My Journey Building the Superstore Analytics Ecosystem',
     'From Raw Data to Regional Strategy. An end-to-end data engineering and analytics ecosystem designed to uncover the "why" behind retail performance.',
     '/static/blog/data-eng-in-practice.webp',
     '2024-04-16 00:00:00+00'),

    ('kpi-generator',
     'Bridging the Gap Between Raw Data and Strategy: Why I Built the KPI Generator',
     'Stop staring at spreadsheets and start driving strategy. The KPI Generator is a Python-based automation tool that leverages Google Gemini AI to instantly transform raw CSV data into actionable business metrics.',
     '/static/blog/kpi-generator.webp',
     '2024-04-08 00:00:00+00')
ON CONFLICT (slug) DO NOTHING;


CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL
);