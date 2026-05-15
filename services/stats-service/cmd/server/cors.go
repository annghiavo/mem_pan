package main

import (
	"net/http"
	"os"
	"strings"
)

// withCORS wraps the given handler with permissive-by-default CORS handling.
//
// Origins are read from CORS_ALLOWED_ORIGINS (comma-separated). When unset or
// set to "*", any Origin is reflected back. Setting an explicit list restricts
// access to those origins. Credentials are always allowed so the Authorization
// header and cookies work from browsers.
func withCORS(next http.Handler) http.Handler {
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	allowAll := len(allowed) == 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (allowAll || originAllowed(origin, allowed)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			} else {
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
			}
			w.Header().Set("Access-Control-Max-Age", "3600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" || raw == "*" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
