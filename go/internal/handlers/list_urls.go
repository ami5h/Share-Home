package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"share-home/internal/db"
)

type ListURLsHandler struct{ db *db.DB }

func NewListURLsHandler(d *db.DB) *ListURLsHandler {
	return &ListURLsHandler{db: d}
}

func (h *ListURLsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	urls, err := h.db.ListURLs(limit)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type resp struct {
		Code     string `json:"code"`
		LongURL  string `json:"long_url"`
		ShortURL string `json:"short_url"`
	}
	out := make([]resp, len(urls))
	for i, u := range urls {
		out[i] = resp{Code: u.Code, LongURL: u.LongURL, ShortURL: "/" + u.Code}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
