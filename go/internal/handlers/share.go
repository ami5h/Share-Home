package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"share-home/internal/db"

	"github.com/google/uuid"
)

type ShareHandler struct {
	store Store
	db    *db.DB
}

func NewShareHandler(store Store, db *db.DB) *ShareHandler {
	return &ShareHandler{store: store, db: db}
}

func (h *ShareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form-encoded body
	const maxBody = 1 << 20 // 1MB
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")
	url := r.FormValue("url")
	content := text
	if content == "" {
		content = url
	}
	if content == "" {
		http.Error(w, "text or url required", http.StatusBadRequest)
		return
	}

	// Save as clipboard entry
	id := uuid.New().String()
	storeKey := fmt.Sprintf("clipboard/%s/%s", id[:2], id)
	data := []byte(content)

	if err := h.store.Put(storeKey, bytes.NewReader(data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.db.CreateClipboard(id, "text", "text/plain", storeKey, int64(len(data))); err != nil {
		h.store.Delete(storeKey)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	Broadcast()

	// Redirect to success page
	http.Redirect(w, r, "/share-success.html", http.StatusSeeOther)
}
