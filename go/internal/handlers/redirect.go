package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"share-home/internal/db"
)

type RedirectHandler struct {
	db *db.DB
}

func NewRedirectHandler(db *db.DB) *RedirectHandler {
	return &RedirectHandler{db: db}
}

func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/")
	longURL, err := h.db.GetURL(code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Validate the target URL is http/https
	parsed, err := url.Parse(longURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		http.Error(w, "invalid redirect target", http.StatusBadGateway)
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}
