package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"share-home/internal/db"
)

type URLHandler struct {
	db *db.DB
}

type URLReq struct {
	URL string `json:"url"`
}

type URLResp struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const maxCodeAttempts = 100

func generateCode() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	code := make([]byte, 6)
	for i := range code {
		code[i] = base62Chars[b[i]%62]
	}
	return string(code)
}

func NewURLHandler(db *db.DB) *URLHandler {
	return &URLHandler{db: db}
}

func (h *URLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req URLReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}

	// Validate URL scheme
	parsed, err := url.Parse(req.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		http.Error(w, "invalid URL: must be http or https", http.StatusBadRequest)
		return
	}

	code := generateCode()
	for i := 0; i < maxCodeAttempts; i++ {
		if _, err := h.db.GetURL(code); err != nil {
			break // code doesn't exist, use it
		}
		code = generateCode()
	}

	if err := h.db.CreateURL(code, req.URL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(URLResp{
		Code:     code,
		ShortURL: fmt.Sprintf("/%s", code),
	})
}
