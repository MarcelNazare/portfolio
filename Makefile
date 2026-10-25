# ─── Platform detection ─────────────────────────────────────────────
ifeq ($(OS),Windows_NT)
    TW      := ./tailwindcss.exe
    EXE     := .exe
    TW_URL  := https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-windows-x64.exe
else
    TW      := ./tailwindcss
    EXE     :=
    TW_URL  := https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
endif

.PHONY: help setup install css dev dev-sqlite hot build clean reset deploy

help:
	@echo "make setup       One-time: install deps, download Tailwind, create dirs"
	@echo "make css         Rebuild static/style.css"
	@echo "make dev         Run with Postgres (reads .env)"
	@echo "make dev-sqlite  Run with SQLite (reads .env)"
	@echo "make hot         Run with hot-reload (Air, reads .env)"
	@echo "make build       Compile binary"
	@echo "make clean       Remove binary + local SQLite files"
	@echo "make reset       Clean + remove uploads/"
	@echo "make deploy      fly deploy"

install:
	go mod tidy
ifeq ($(OS),Windows_NT)
	@if [ ! -f tailwindcss.exe ]; then \
		echo "Downloading Tailwind CLI for Windows..."; \
		powershell -Command "Invoke-WebRequest -Uri '$(TW_URL)' -OutFile 'tailwindcss.exe'"; \
	else \
		echo "Tailwind CLI already present."; \
	fi
else
	@if [ ! -f tailwindcss ]; then \
		echo "Downloading Tailwind CLI for Linux..."; \
		curl -sLO $(TW_URL) && \
		chmod +x tailwindcss-linux-x64 && \
		mv tailwindcss-linux-x64 tailwindcss; \
	else \
		echo "Tailwind CLI already present."; \
	fi
endif

setup: install
	mkdir -p uploads static/blog
	@echo ""
	@echo "Setup complete. Next:"
	@echo "  1. Create a .env file with DATABASE_URL and ADMIN_PASSWORD"
	@echo "  2. Run 'make hot' or 'make dev'"

css:
	$(TW) -i input.css -o static/style.css --minify

dev: css
	go run .

dev-sqlite: css
	go run -tags sqlite .

hot: css
	go run github.com/air-verse/air@latest

build: css
	go build -ldflags="-s -w" -o portfolio$(EXE) .

clean:
	rm -f portfolio portfolio.exe
	rm -f portfolio.db portfolio.db-wal portfolio.db-shm

reset: clean
	rm -rf uploads

deploy:
	fly deploy