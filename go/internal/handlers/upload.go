package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"share-home/internal/db"
	"time"

	"github.com/google/uuid"
)

type UploadHandler struct {
	store Store
	db    *db.DB
}

type UploadResp struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MIME       string `json:"mime"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	DownloadURL string `json:"download_url"`
}

func NewUploadHandler(store Store, d *db.DB) *UploadHandler {
	return &UploadHandler{store: store, db: d}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 30) // 10GB max
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	id := uuid.New().String()
	storeKey := fmt.Sprintf("files/%s/%s", id[:2], id)

	if err := h.store.Put(storeKey, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var expiresAt *time.Time
	if ea := r.FormValue("expires_at"); ea != "" {
		switch ea {
		case "1h", "1d", "1w":
			dur := 0 * time.Second
			switch ea {
			case "1h":
				dur = 1 * time.Hour
			case "1d":
				dur = 24 * time.Hour
			case "1w":
				dur = 7 * 24 * time.Hour
			}
			t := time.Now().Add(dur)
			expiresAt = &t
		}
	}

	if expiresAt != nil {
		err = h.db.CreateFileWithExpiry(id, header.Filename, header.Header.Get("Content-Type"), storeKey, header.Size, expiresAt)
	} else {
		err = h.db.CreateFile(id, header.Filename, header.Header.Get("Content-Type"), storeKey, header.Size)
	}
	if err != nil {
		h.store.Delete(storeKey)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	expStr := ""
	if expiresAt != nil {
		expStr = expiresAt.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	Broadcast()
	json.NewEncoder(w).Encode(UploadResp{
		ID:          id,
		Name:        header.Filename,
		Size:        header.Size,
		MIME:        header.Header.Get("Content-Type"),
		ExpiresAt:   expStr,
		DownloadURL: fmt.Sprintf("/api/download/%s", id),
	})
}
