package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"share-home/internal/db"
)

type ListFilesHandler struct{ db *db.DB }

func NewListFilesHandler(d *db.DB) *ListFilesHandler {
	return &ListFilesHandler{db: d}
}

func (h *ListFilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	files, err := h.db.ListFiles(limit)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	type resp struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Size      int64   `json:"size"`
		MIME      string  `json:"mime"`
		URL       string  `json:"url"`
		Downloads int     `json:"downloads"`
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	out := make([]resp, len(files))
	for i, f := range files {
		r := resp{ID: f.ID, Name: f.Name, Size: f.Size, MIME: f.MIME, URL: "/api/download/" + f.ID, Downloads: f.Downloads}
		if f.ExpiresAt != nil {
			exp := f.ExpiresAt.Format("2006-01-02T15:04:05Z")
			r.ExpiresAt = &exp
		}
		out[i] = r
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
