package hub

import (
	"embed"
	"io/fs"
	"net/http"
	"net/netip"
	"path"
	"strings"
)

//go:embed web/*
var webFS embed.FS

func netipParse(s string) (netip.Prefix, error) { return netip.ParsePrefix(s) }

// Handler returns the dashboard + REST API HTTP handler for s.
func Handler(s *Server) http.Handler { return (&api{srv: s}).mux() }

// static serves the embedded dashboard with strict security headers.
func (a *api) static(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		r.URL.Path = "/index.html"
	}
	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean == "" || clean == "." {
		clean = "index.html"
	}
	b, err := fs.ReadFile(webFS, "web/"+clean)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(clean, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(clean, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(clean, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(clean, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(clean, ".json"), strings.HasSuffix(clean, ".webmanifest"):
		w.Header().Set("Content-Type", "application/json")
	case strings.HasSuffix(clean, ".png"):
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}
