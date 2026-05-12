package handlers

import (
	"net/http"
	"share-home/internal/db"
)

type DeleteClipboardHandler struct {
	store Store
	db    *db.DB
}

func NewDeleteClipboardHandler(store Store, d *db.DB) *DeleteClipboardHandler {
	return &DeleteClipboardHandler{store: store, db: d}
}

func (h *DeleteClipboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	entry, err := h.db.GetClipboard(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.store.Delete(entry.RGWKey)
	h.db.DeleteClipboard(id)
	Broadcast()
	w.WriteHeader(http.StatusNoContent)
}
