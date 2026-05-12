package handlers

import (
	"net/http"
	"share-home/internal/db"
)

type DeleteURLHandler struct{ db *db.DB }

func NewDeleteURLHandler(d *db.DB) *DeleteURLHandler {
	return &DeleteURLHandler{db: d}
}

func (h *DeleteURLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code := r.PathValue("code")
	h.db.DeleteURL(code)
	Broadcast()
	w.WriteHeader(http.StatusNoContent)
}
