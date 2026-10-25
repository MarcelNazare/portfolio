package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// ─── CSRF PROTECTION ────────────────────────────────────────────────

func (a *App) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Ensure a CSRF cookie exists
		cookie, err := r.Cookie("csrf_token")
		var token string
		if err != nil || cookie.Value == "" {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			token = hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name:     "csrf_token",
				Value:    token,
				Path:     "/",
				HttpOnly: false, // Must be readable by JS to attach to HTMX headers
				SameSite: http.SameSiteStrictMode,
				MaxAge:   86400 * 7,
			})
		} else {
			token = cookie.Value
		}

		// Inject the token into the context for templates
		// (We'll just pass it via a wrapper to the context or use a custom Header setter)
		r.Header.Set("X-CSRF-Token-Context", token)

		// 2. Validate on state-changing requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			// Skip CSRF for login since there's no session yet, 
			// but we still validate the token to prevent login CSRF
			headerToken := r.Header.Get("X-CSRF-Token")
			formToken := r.FormValue("csrf_token")
			
			if headerToken == "" && formToken == "" {
				http.Error(w, "CSRF token missing", http.StatusForbidden)
				return
			}
			if headerToken != token && formToken != token {
				http.Error(w, "CSRF token invalid", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ─── RATE LIMITING (Login) ──────────────────────────────────────────

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{attempts: make(map[string][]time.Time)}
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// Keep only attempts from the last 15 minutes
	var valid []time.Time
	for _, t := range rl.attempts[ip] {
		if now.Sub(t) < 15*time.Minute {
			valid = append(valid, t)
		}
	}
	
	rl.attempts[ip] = valid

	if len(valid) >= 5 {
		return false // Max 5 attempts per 15 mins
	}

	rl.attempts[ip] = append(rl.attempts[ip], now)
	return true
}

func (a *App) loginRateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			ip := r.RemoteAddr
			if !a.RateLimiter.Allow(ip) {
				http.Error(w, "Too many login attempts. Please try again in 15 minutes.", http.StatusTooManyRequests)
				return
			}
		}
		next(w, r)
	}
}