package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"share-home/internal/db"
)

type ListClipboardHandler struct{ db *db.DB }

func NewListClipboardHandler(d *db.DB) *ListClipboardHandler {
	return &ListClipboardHandler{db: d}
}

func (h *ListClipboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, err := h.db.ListClipboard(limit)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type resp struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Size int64  `json:"size"`
		MIME string `json:"mime"`
		URL  string `json:"url"`
	}
	out := make([]resp, len(items))
	for i, c := range items {
		out[i] = resp{ID: c.ID, Type: c.Type, Size: c.Size, MIME: c.MIME, URL: "/api/clipboard/" + c.ID}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
