package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"share-home/internal/db"

	"github.com/google/uuid"
)

type UploadURLHandler struct {
	store Store
	db    *db.DB
}

func NewUploadURLHandler(store Store, d *db.DB) *UploadURLHandler {
	return &UploadURLHandler{store: store, db: d}
}

func (h *UploadURLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	// Validate URL
	parsed, err := url.Parse(req.URL)
	if err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		http.Error(w, "only http/https supported", http.StatusBadRequest)
		return
	}

	// Fetch the remote URL with a 10GB limit and timeout
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Get(req.URL)
	if err != nil {
		http.Error(w, "failed to fetch", http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, fmt.Sprintf("remote returned %d", resp.StatusCode), http.StatusBadRequest)
		return
	}

	// Limit body size to 10GB
	body := io.LimitReader(resp.Body, 10<<30)

	id := uuid.New().String()
	storeKey := fmt.Sprintf("files/%s/%s", id[:2], id)

	if err := h.store.Put(storeKey, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine filename from URL path or Content-Disposition
	name := filenameFromURL(parsed.Path)
	ct := resp.Header.Get("Content-Type")

	size := resp.ContentLength
	if size < 0 {
		size = 0
	}

	if err := h.db.CreateFile(id, name, ct, storeKey, size); err != nil {
		h.store.Delete(storeKey)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	Broadcast()
	json.NewEncoder(w).Encode(UploadResp{
		ID:          id,
		Name:        name,
		Size:        size,
		MIME:        ct,
		DownloadURL: fmt.Sprintf("/api/download/%s", id),
	})
}

func filenameFromURL(path string) string {
	base := filepath.Base(path)
	if base == "." || base == "/" {
		return "download"
	}
	base = strings.ReplaceAll(base, "+", " ")
	return base
}
