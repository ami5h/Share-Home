package handlers

import (
	"fmt"
	"io"
	"net/http"
	"share-home/internal/db"
)

type ClipboardReadHandler struct {
	store Store
	db    *db.DB
}

func NewClipboardReadHandler(store Store, db *db.DB) *ClipboardReadHandler {
	return &ClipboardReadHandler{store: store, db: db}
}

func (h *ClipboardReadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entry, err := h.db.GetClipboard(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	body, size, err := h.store.Get(entry.RGWKey)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	if size >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	mimeType := entry.MIME
	if isUnsafeMIME(mimeType) {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	io.Copy(w, body)
}
