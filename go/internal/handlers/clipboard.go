package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"share-home/internal/db"
	"strings"

	"github.com/google/uuid"
)

type ClipboardHandler struct {
	store Store
	db    *db.DB
}

type ClipboardResp struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func NewClipboardHandler(store Store, db *db.DB) *ClipboardHandler {
	return &ClipboardHandler{store: store, db: db}
}

func (h *ClipboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := uuid.New().String()
	storeKey := fmt.Sprintf("clipboard/%s/%s", id[:2], id)

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		var mimeType string
		var data []byte
		switch req.Type {
		case "text":
			mimeType = "text/plain"
			data = []byte(req.Content)
		case "image":
			mimeType = "image/png"
			content := strings.TrimSpace(req.Content)
			if idx := strings.Index(content, ";base64,"); idx != -1 {
				content = content[idx+8:]
			}
			decoded, err := base64.StdEncoding.DecodeString(content)
			if err != nil {
				http.Error(w, "invalid base64", http.StatusBadRequest)
				return
			}
			data = decoded
		default:
			http.Error(w, "type must be text or image", http.StatusBadRequest)
			return
		}

		if err := h.store.Put(storeKey, bytes.NewReader(data)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := h.db.CreateClipboard(id, req.Type, mimeType, storeKey, int64(len(data))); err != nil {
			h.store.Delete(storeKey)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ClipboardResp{ID: id, Type: req.Type, URL: fmt.Sprintf("/api/clipboard/%s", id)})
			Broadcast()
		return
	}

	// Multipart (image upload)
	r.ParseMultipartForm(50 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file or JSON required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ext := filepath.Ext(header.Filename)
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	typ := "image"
	if strings.HasPrefix(mimeType, "text/") {
		typ = "text"
	}

	if err := h.store.Put(storeKey, bytes.NewReader(data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.db.CreateClipboard(id, typ, mimeType, storeKey, int64(len(data))); err != nil {
		h.store.Delete(storeKey)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ClipboardResp{ID: id, Type: typ, URL: fmt.Sprintf("/api/clipboard/%s", id)})
}
