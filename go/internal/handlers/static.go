package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type StaticHandler struct {
	dir string
}

func NewStaticHandler(dir string) *StaticHandler {
	return &StaticHandler{dir: dir}
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path and check for traversal
	cleanPath := filepath.Clean(r.URL.Path)
	joined := filepath.Join(h.dir, cleanPath)

	// Ensure resolved path stays within the web directory
	if !strings.HasPrefix(filepath.Clean(joined), filepath.Clean(h.dir)+string(filepath.Separator)) &&
		filepath.Clean(joined) != filepath.Clean(h.dir) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Try the actual file
	if info, err := filepath.Abs(joined); err == nil {
		if stat, statErr := os.Stat(info); statErr == nil {
			// Set ETag from file modification time and size
			etag := fmt.Sprintf(`"%x"`, stat.ModTime().UnixNano()^stat.Size())
			w.Header().Set("ETag", etag)

			// Cache headers: immutable for JS/CSS/images, short for HTML
			ext := filepath.Ext(cleanPath)
			switch ext {
			case ".json":
				w.Header().Set("Content-Type", "application/manifest+json")
				w.Header().Set("Cache-Control", "no-cache")
			case ".html", ".htm":
				w.Header().Set("Cache-Control", "no-cache")
			case ".js", ".css":
				w.Header().Set("Cache-Control", "no-cache")
			case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp":
				w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
			}

			http.ServeFile(w, r, info)
			return
		}
	}

	// SPA fallback — serve index.html
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(h.dir, "index.html"))
}

// ConfigHandler serves a dynamic JS file with runtime config for the frontend.
type ConfigHandler struct {
	AuthRequired bool
}

func (h *ConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-store")
	required := "false"
	if h.AuthRequired {
		required = "true"
	}
	fmt.Fprintf(w, "var __CONFIG__={authRequired:%s};\n", required)
}
