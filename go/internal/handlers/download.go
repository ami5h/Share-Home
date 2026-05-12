package handlers

import (
	"fmt"
	"io"
	"net/http"
	"share-home/internal/db"
	"strings"
)

type DownloadHandler struct {
	store Store
	db    *db.DB
}

func NewDownloadHandler(store Store, db *db.DB) *DownloadHandler {
	return &DownloadHandler{store: store, db: db}
}

func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	meta, err := h.db.GetFile(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	body, size, err := h.store.Get(meta.RGWKey)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	h.db.IncrementDownload(id)

	if size >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	safeName := sanitizeFilename(meta.Name)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))

	// Use user MIME for display, but block unsafe types
	mimeType := meta.MIME
	if isUnsafeMIME(mimeType) {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)

	w.Header().Set("Cache-Control", "public, max-age=31536000")
	io.Copy(w, body)
}

// sanitizeFilename removes dangerous characters from filenames used in headers.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, `\`, "")
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	return name
}

// isUnsafeMIME checks for MIME types that could be rendered/executed by browsers.
func isUnsafeMIME(m string) bool {
	lower := strings.ToLower(m)
	unsafe := []string{
		"text/html", "application/xhtml", "application/xml",
		"image/svg", "application/javascript", "text/javascript",
	}
	for _, u := range unsafe {
		if strings.Contains(lower, u) {
			return true
		}
	}
	return false
}
