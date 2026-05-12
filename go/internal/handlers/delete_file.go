package handlers

import (
	"net/http"
	"share-home/internal/db"
)

type DeleteFileHandler struct {
	store Store
	db    *db.DB
}

func NewDeleteFileHandler(store Store, d *db.DB) *DeleteFileHandler {
	return &DeleteFileHandler{store: store, db: d}
}

func (h *DeleteFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	meta, err := h.db.GetFile(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.store.Delete(meta.RGWKey)
	h.db.DeleteFile(id)
	Broadcast()
	w.WriteHeader(http.StatusNoContent)
}
