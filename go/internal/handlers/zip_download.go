package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"share-home/internal/db"
)

type ZipDownloadHandler struct {
	store Store
	db    *db.DB
}

func NewZipDownloadHandler(store Store, d *db.DB) *ZipDownloadHandler {
	return &ZipDownloadHandler{store: store, db: d}
}

func (h *ZipDownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		http.Error(w, "ids required", http.StatusBadRequest)
		return
	}

	if len(req.IDs) > 100 {
		http.Error(w, "too many files", http.StatusBadRequest)
		return
	}

	// Build zip in memory then stream
	// For many/large files we'd stream directly, but zip.Writer doesn't support streaming
	// So we buffer the zip to a bytes.Buffer and stream that
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Track which files we've added (deduplicate + count)
	added := make(map[string]int)
	for _, id := range req.IDs {
		meta, err := h.db.GetFile(id)
		if err != nil {
			continue
		}

		// Deduplicate: handle name collisions
		name := meta.Name
		if count, ok := added[name]; ok {
			name = fmt.Sprintf("%s_%d%s", meta.ID[:8], count, ext(name))
			added[meta.Name] = count + 1
		} else {
			added[name] = 1
		}

		f, err := zw.Create(name)
		if err != nil {
			continue
		}

		body, _, err := h.store.Get(meta.RGWKey)
		if err != nil {
			continue
		}

		_, err = io.Copy(f, body)
		body.Close()
		if err != nil {
			continue
		}

		// Increment download count
		h.db.IncrementDownload(id)
	}

	zw.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="share-home-%d.zip"`, time.Now().Unix()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())
}

func ext(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}
