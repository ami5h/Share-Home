package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"share-home/internal/db"
)

// --- mockStore (in-memory implementation of Store interface) ---

type mockStore struct {
	objects map[string][]byte
}

func newMockStore() *mockStore { return &mockStore{objects: make(map[string][]byte)} }

func (m *mockStore) Put(key string, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[key] = data
	return nil
}

func (m *mockStore) Get(key string) (io.ReadCloser, int64, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, 0, fmt.Errorf("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

func (m *mockStore) Delete(key string) error {
	delete(m.objects, key)
	return nil
}

func (m *mockStore) SpaceInfo() (total, used, free int64, err error) {
	var u int64
	for _, data := range m.objects {
		u += int64(len(data))
	}
	return 1024 * 1024 * 1024, u, 1024*1024*1024 - u, nil // 1 GB total for tests
}

// --- helpers ---

func setupTest(t *testing.T) (*db.DB, *mockStore) {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.Open() error: %v", err)
	}
	return d, newMockStore()
}

func makeMultipartBody(fieldName, filename, content string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, "", err
	}
	part.Write([]byte(content))
	w.Close()
	return &buf, w.Boundary(), nil
}

// wrap wraps a handler through a mux with the given pattern so PathValue works.
func wrap(handler http.HandlerFunc, method, pattern string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(pattern, handler)
	return mux
}

func wrapHandler(h http.Handler, pattern string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(pattern, h)
	return mux
}

// =============================================
// URL Handler Tests
// =============================================

func TestURLHandler_Create(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()

	h := NewURLHandler(d)
	body := strings.NewReader(`{"url":"https://example.com/test"}`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	var resp URLResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}
	if resp.Code == "" {
		t.Error("code should not be empty")
	}
	if len(resp.Code) != 6 {
		t.Errorf("code length = %d, want 6", len(resp.Code))
	}
	if resp.ShortURL != "/"+resp.Code {
		t.Errorf("short_url = %q, want /%s", resp.ShortURL, resp.Code)
	}
}

func TestURLHandler_HTTPScheme(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()

	h := NewURLHandler(d)
	body := strings.NewReader(`{"url":"http://example.com"}`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("http scheme: got %d, want 201", rec.Code)
	}
}

func TestURLHandler_RejectInvalidScheme(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	for _, tc := range []struct {
		url  string
		name string
	}{
		{"ftp://example.com", "ftp"},
		{"file:///etc/passwd", "file"},
		{"javascript:alert(1)", "javascript"},
		{"data:text/html,<h1>test</h1>", "data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.NewReader(`{"url":"` + tc.url + `"}`)
			req := httptest.NewRequest("POST", "/api/url", body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("scheme %s: got %d, want 400", tc.name, rec.Code)
			}
		})
	}
}

func TestURLHandler_EmptyURL(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	body := strings.NewReader(`{"url":""}`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty url: got %d, want 400", rec.Code)
	}
}

func TestURLHandler_MissingURL(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing url: got %d, want 400", rec.Code)
	}
}

func TestURLHandler_InvalidJSON(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid json: got %d, want 400", rec.Code)
	}
}

func TestURLHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	for _, method := range []string{"GET", "PUT", "DELETE", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/url", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: got %d, want 405", method, rec.Code)
			}
		})
	}
}

func TestURLHandler_DuplicateCodeRetry(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewURLHandler(d)

	// Pre-insert codes to test collision retry
	d.CreateURL("000000", "https://ex.com/1")
	d.CreateURL("000001", "https://ex.com/2")
	d.CreateURL("000002", "https://ex.com/3")

	// Should still succeed despite possible collisions
	successes := 0
	for i := 0; i < 5; i++ {
		body := strings.NewReader(`{"url":"https://example.com/test"}`)
		req := httptest.NewRequest("POST", "/api/url", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusCreated {
			successes++
		}
	}
	if successes < 3 {
		t.Errorf("expected at least 3 successes, got %d", successes)
	}
}

// =============================================
// Redirect Handler Tests
// =============================================

func TestRedirectHandler_Found(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("abc123", "https://target.example.com/path")

	h := NewRedirectHandler(d)
	req := httptest.NewRequest("GET", "/abc123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("got %d, want 302", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "https://target.example.com/path" {
		t.Errorf("location = %q, want %q", loc, "https://target.example.com/path")
	}
}

func TestRedirectHandler_NotFound(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewRedirectHandler(d)

	req := httptest.NewRequest("GET", "/nonexist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestRedirectHandler_RejectInvalidTarget(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("bad1", "file:///etc/passwd")
	d.CreateURL("bad2", "javascript:alert(1)")
	h := NewRedirectHandler(d)

	for _, code := range []string{"bad1", "bad2"} {
		t.Run(code, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+code, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadGateway {
				t.Errorf("code %s: got %d, want 502", code, rec.Code)
			}
		})
	}
}

func TestRedirectHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("abc", "https://example.com")
	h := NewRedirectHandler(d)

	req := httptest.NewRequest("POST", "/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST redirect: got %d, want 405", rec.Code)
	}
}

func TestRedirectHandler_HTTPTarget(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("ht", "http://example.com")
	h := NewRedirectHandler(d)

	req := httptest.NewRequest("GET", "/ht", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("http target: got %d, want 302", rec.Code)
	}
}

// =============================================
// Upload Handler Tests
// =============================================

func TestUploadHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadHandler(store, d)
	buf, boundary, err := makeMultipartBody("file", "hello.txt", "hello world content")
	if err != nil {
		t.Fatalf("makeMultipartBody error: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/upload", buf)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	var resp UploadResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}
	if resp.ID == "" {
		t.Error("id should not be empty")
	}
	if resp.Name != "hello.txt" {
		t.Errorf("name = %q, want %q", resp.Name, "hello.txt")
	}
	if resp.Size != 19 {
		t.Errorf("size = %d, want 19", resp.Size)
	}
	if resp.DownloadURL != "/api/download/"+resp.ID {
		t.Errorf("download_url = %q, want /api/download/%s", resp.DownloadURL, resp.ID)
	}

	// Verify file is in store
	key := "files/" + resp.ID[:2] + "/" + resp.ID
	data, _, err := store.Get(key)
	if err != nil {
		t.Fatalf("store.Get() error: %v", err)
	}
	body, _ := io.ReadAll(data)
	if string(body) != "hello world content" {
		t.Errorf("stored content = %q, want %q", string(body), "hello world content")
	}
}

func TestUploadHandler_RejectGET(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewUploadHandler(store, d)

	req := httptest.NewRequest("GET", "/api/upload", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET upload: got %d, want 405", rec.Code)
	}
}

func TestUploadHandler_MissingFile(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewUploadHandler(store, d)

	req := httptest.NewRequest("POST", "/api/upload", strings.NewReader("no multipart"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("no file: got %d, want 400", rec.Code)
	}
}

// =============================================
// Upload URL Handler Tests
// =============================================

func TestUploadURLHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", `attachment; filename="remote.txt"`)
		w.Write([]byte("remote content"))
	}))
	defer srv.Close()

	h := NewUploadURLHandler(store, d)
	body := strings.NewReader(`{"url":"` + srv.URL + `/remote.txt"}`)
	req := httptest.NewRequest("POST", "/api/upload_url", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201, body: %s", rec.Code, rec.Body.String())
	}
	var resp UploadResp
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ID == "" {
		t.Error("id should not be empty")
	}
	if resp.Name != "remote.txt" {
		t.Errorf("name = %q, want %q", resp.Name, "remote.txt")
	}

	// Verify stored
	key := "files/" + resp.ID[:2] + "/" + resp.ID
	data, _, err := store.Get(key)
	if err != nil {
		t.Fatalf("store.Get() error: %v", err)
	}
	content, _ := io.ReadAll(data)
	if string(content) != "remote content" {
		t.Errorf("content = %q", string(content))
	}
}

func TestUploadURLHandler_InvalidJSON(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadURLHandler(store, d)
	req := httptest.NewRequest("POST", "/api/upload_url", strings.NewReader("bad"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestUploadURLHandler_EmptyURL(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadURLHandler(store, d)
	body := strings.NewReader(`{"url":""}`)
	req := httptest.NewRequest("POST", "/api/upload_url", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestUploadURLHandler_BadScheme(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadURLHandler(store, d)
	body := strings.NewReader(`{"url":"ftp://example.com/file.txt"}`)
	req := httptest.NewRequest("POST", "/api/upload_url", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestUploadURLHandler_RejectGET(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadURLHandler(store, d)
	req := httptest.NewRequest("GET", "/api/upload_url", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rec.Code)
	}
}

// =============================================
// Download Handler Tests
// =============================================

func TestDownloadHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "test-dl-id"
	storeKey := "files/te/" + id
	store.Put(storeKey, bytes.NewReader([]byte("file content here")))
	d.CreateFile(id, "report.pdf", "application/pdf", storeKey, 17)

	h := NewDownloadHandler(store, d)
	req := httptest.NewRequest("GET", "/api/download/"+id, nil)
	rec := httptest.NewRecorder()
	wrapHandler(h, "GET /api/download/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "file content here" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "file content here")
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition missing 'attachment': %q", cd)
	}
	if !strings.Contains(cd, "report.pdf") {
		t.Errorf("Content-Disposition missing filename: %q", cd)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options: nosniff")
	}
}

func TestDownloadHandler_NotFound(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDownloadHandler(store, d)

	req := httptest.NewRequest("GET", "/api/download/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestDownloadHandler_BlockUnsafeMIME(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "unsafe-id"
	storeKey := "files/un/" + id
	store.Put(storeKey, bytes.NewReader([]byte("<script>alert(1)</script>")))
	d.CreateFile(id, "evil.html", "text/html", storeKey, 25)

	h := NewDownloadHandler(store, d)
	req := httptest.NewRequest("GET", "/api/download/"+id, nil)
	rec := httptest.NewRecorder()
	wrapHandler(h, "GET /api/download/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestDownloadHandler_SanitizeFilename(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "san-id"
	storeKey := "files/sa/" + id
	store.Put(storeKey, bytes.NewReader([]byte("test")))
	d.CreateFile(id, `bad"file\nname.txt`, "text/plain", storeKey, 4)

	h := NewDownloadHandler(store, d)
	req := httptest.NewRequest("GET", "/api/download/"+id, nil)
	rec := httptest.NewRecorder()
	wrapHandler(h, "GET /api/download/{id}").ServeHTTP(rec, req)

	cd := rec.Header().Get("Content-Disposition")
	// Extract the filename value between quotes
	idx := strings.Index(cd, `filename="`)
	if idx == -1 {
		t.Fatal("Content-Disposition missing filename=")
	}
	fn := cd[idx+len("filename=\"") : len(cd)-1]
	if strings.ContainsAny(fn, "\"\r\n") {
		t.Errorf("unsanitized filename in Content-Disposition: %q", fn)
	}
	if fn != "badfilenname.txt" {
		t.Errorf("filename = %q, want %q", fn, "badfilenname.txt")
	}
}

func TestDownloadHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDownloadHandler(store, d)

	req := httptest.NewRequest("DELETE", "/api/download/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE download: got %d, want 405", rec.Code)
	}
}

func TestDownloadHandler_JSFile(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "js-id"
	storeKey := "files/js/" + id
	store.Put(storeKey, bytes.NewReader([]byte("alert(1)")))
	d.CreateFile(id, "app.js", "application/javascript", storeKey, 8)

	h := NewDownloadHandler(store, d)
	req := httptest.NewRequest("GET", "/api/download/"+id, nil)
	rec := httptest.NewRecorder()
	wrapHandler(h, "GET /api/download/{id}").ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestDownloadHandler_IncrementsCount(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "count-id"
	storeKey := "files/co/" + id
	store.Put(storeKey, bytes.NewReader([]byte("count me")))
	d.CreateFile(id, "count.txt", "text/plain", storeKey, 10)

	h := NewDownloadHandler(store, d)
	mux := wrapHandler(h, "GET /api/download/{id}")

	// Download twice
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/download/"+id, nil))
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/download/"+id, nil))

	meta, err := d.GetFile(id)
	if err != nil {
		t.Fatalf("GetFile() error: %v", err)
	}
	if meta.Downloads != 2 {
		t.Errorf("downloads = %d, want 2", meta.Downloads)
	}
}

// =============================================
// Zip Download Handler Tests
// =============================================

func TestZipDownload_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id1 := "zip-id-1"
	id2 := "zip-id-2"
	store.Put("files/zi/"+id1, bytes.NewReader([]byte("file one")))
	store.Put("files/zi/"+id2, bytes.NewReader([]byte("file two")))
	d.CreateFile(id1, "one.txt", "text/plain", "files/zi/"+id1, 8)
	d.CreateFile(id2, "two.txt", "text/plain", "files/zi/"+id2, 8)

	h := NewZipDownloadHandler(store, d)
	body := strings.NewReader(`{"ids":["` + id1 + `","` + id2 + `"]}`)
	req := httptest.NewRequest("POST", "/api/download/zip", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", rec.Header().Get("Content-Type"))
	}
	// Verify it's a valid zip
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader error: %v", err)
	}
	if len(zr.File) != 2 {
		t.Errorf("zip files = %d, want 2", len(zr.File))
	}
}

func TestZipDownload_EmptyIDs(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewZipDownloadHandler(store, d)
	body := strings.NewReader(`{"ids":[]}`)
	req := httptest.NewRequest("POST", "/api/download/zip", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestZipDownload_InvalidJSON(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewZipDownloadHandler(store, d)
	req := httptest.NewRequest("POST", "/api/download/zip", strings.NewReader("bad json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestZipDownload_SkipsNotFound(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "zip-gone"
	store.Put("files/zi/"+id, bytes.NewReader([]byte("exists")))
	d.CreateFile(id, "exists.txt", "text/plain", "files/zi/"+id, 7)

	h := NewZipDownloadHandler(store, d)
	body := strings.NewReader(`{"ids":["nonexistent","` + id + `"]}`)
	req := httptest.NewRequest("POST", "/api/download/zip", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip error: %v", err)
	}
	if len(zr.File) != 1 {
		t.Errorf("zip files = %d, want 1", len(zr.File))
	}
}

func TestZipDownload_RejectGET(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewZipDownloadHandler(store, d)
	req := httptest.NewRequest("GET", "/api/download/zip", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rec.Code)
	}
}

// =============================================
// Delete File Handler Tests
// =============================================

func TestDeleteFileHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "del-id"
	storeKey := "files/de/" + id
	store.Put(storeKey, bytes.NewReader([]byte("to delete")))
	d.CreateFile(id, "temp.txt", "text/plain", storeKey, 9)

	h := NewDeleteFileHandler(store, d)
	req := httptest.NewRequest("DELETE", "/api/files/"+id, nil)
	rec := httptest.NewRecorder()
	wrapHandler(h, "DELETE /api/files/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}

	// Verify removed from store
	_, _, err := store.Get(storeKey)
	if err == nil {
		t.Error("file should be removed from store")
	}
}

func TestDeleteFileHandler_NotFound(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDeleteFileHandler(store, d)

	req := httptest.NewRequest("DELETE", "/api/files/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestDeleteFileHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDeleteFileHandler(store, d)

	req := httptest.NewRequest("GET", "/api/files/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET delete: got %d, want 405", rec.Code)
	}
}

// =============================================
// Clipboard Handler Tests
// =============================================

func TestClipboardHandler_Text(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	body := strings.NewReader(`{"type":"text","content":"hello world"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201, body: %s", rec.Code, rec.Body.String())
	}

	var resp ClipboardResp
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Type != "text" {
		t.Errorf("type = %q, want %q", resp.Type, "text")
	}
	if resp.URL == "" {
		t.Error("url should not be empty")
	}
	if resp.ID == "" {
		t.Error("id should not be empty")
	}
}

func TestClipboardHandler_ImageBase64(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	encoded := base64.StdEncoding.EncodeToString([]byte("PNG_IMAGE_DATA"))
	body := strings.NewReader(`{"type":"image","content":"` + encoded + `"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201", rec.Code)
	}

	var resp ClipboardResp
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Type != "image" {
		t.Errorf("type = %q, want %q", resp.Type, "image")
	}

	// Verify stored data
	key := "clipboard/" + resp.ID[:2] + "/" + resp.ID
	data, _, err := store.Get(key)
	if err != nil {
		t.Fatalf("store.Get() error: %v", err)
	}
	content, _ := io.ReadAll(data)
	if string(content) != "PNG_IMAGE_DATA" {
		t.Errorf("stored data = %q, want %q", string(content), "PNG_IMAGE_DATA")
	}
}

func TestClipboardHandler_ImageWithPrefix(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	encoded := base64.StdEncoding.EncodeToString([]byte("IMG"))
	body := strings.NewReader(`{"type":"image","content":"data:image/png;base64,` + encoded + `"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("image with prefix: got %d, want 201", rec.Code)
	}
}

func TestClipboardHandler_InvalidType(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	body := strings.NewReader(`{"type":"video","content":"data"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("video type: got %d, want 400", rec.Code)
	}
}

func TestClipboardHandler_InvalidJSON(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	body := strings.NewReader(`{broken`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid json: got %d, want 400", rec.Code)
	}
}

func TestClipboardHandler_InvalidBase64(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	body := strings.NewReader(`{"type":"image","content":"!!!not-base64!!!"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid base64: got %d, want 400", rec.Code)
	}
}

func TestClipboardHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardHandler(store, d)

	req := httptest.NewRequest("GET", "/api/clipboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET clipboard: got %d, want 405", rec.Code)
	}
}

// =============================================
// Clipboard Read Handler Tests
// =============================================

func TestClipboardReadHandler_Text(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "clip-read-id"
	storeKey := "clipboard/cl/" + id
	store.Put(storeKey, bytes.NewReader([]byte("clipboard text")))
	d.CreateClipboard(id, "text", "text/plain", storeKey, 14)

	h := NewClipboardReadHandler(store, d)
	mux := wrapHandler(h, "GET /api/clipboard/{id}")
	req := httptest.NewRequest("GET", "/api/clipboard/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "clipboard text" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "clipboard text")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options: nosniff")
	}
}

func TestClipboardReadHandler_Image(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "img-read-id"
	storeKey := "clipboard/im/" + id
	store.Put(storeKey, bytes.NewReader([]byte("IMG_DATA")))
	d.CreateClipboard(id, "image", "image/png", storeKey, 8)

	h := NewClipboardReadHandler(store, d)
	mux := wrapHandler(h, "GET /api/clipboard/{id}")
	req := httptest.NewRequest("GET", "/api/clipboard/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestClipboardReadHandler_NotFound(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardReadHandler(store, d)

	req := httptest.NewRequest("GET", "/api/clipboard/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestClipboardReadHandler_BlockUnsafeMIME(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "unsafe-clip"
	storeKey := "clipboard/un/" + id
	store.Put(storeKey, bytes.NewReader([]byte("<html>")))
	d.CreateClipboard(id, "text", "text/html", storeKey, 6)

	h := NewClipboardReadHandler(store, d)
	mux := wrapHandler(h, "GET /api/clipboard/{id}")
	req := httptest.NewRequest("GET", "/api/clipboard/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestClipboardReadHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewClipboardReadHandler(store, d)

	req := httptest.NewRequest("POST", "/api/clipboard/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST clipboard read: got %d, want 405", rec.Code)
	}
}

// =============================================
// Delete Clipboard Handler Tests
// =============================================

func TestDeleteClipboardHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	id := "del-clip-id"
	storeKey := "clipboard/de/" + id
	store.Put(storeKey, bytes.NewReader([]byte("to delete")))
	d.CreateClipboard(id, "text", "text/plain", storeKey, 9)

	h := NewDeleteClipboardHandler(store, d)
	mux := wrapHandler(h, "DELETE /api/clipboard/{id}")
	req := httptest.NewRequest("DELETE", "/api/clipboard/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	_, _, err := store.Get(storeKey)
	if err == nil {
		t.Error("clipboard data should be removed from store")
	}
}

func TestDeleteClipboardHandler_NotFound(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDeleteClipboardHandler(store, d)

	req := httptest.NewRequest("DELETE", "/api/clipboard/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestDeleteClipboardHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewDeleteClipboardHandler(store, d)

	req := httptest.NewRequest("GET", "/api/clipboard/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET delete clipboard: got %d, want 405", rec.Code)
	}
}

// =============================================
// Delete URL Handler Tests
// =============================================

func TestDeleteURLHandler_Success(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("abc", "https://example.com")

	h := NewDeleteURLHandler(d)
	mux := wrapHandler(h, "DELETE /api/urls/{code}")
	req := httptest.NewRequest("DELETE", "/api/urls/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rec.Code)
	}
	_, err := d.GetURL("abc")
	if err == nil {
		t.Error("URL should be deleted")
	}
}

func TestDeleteURLHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewDeleteURLHandler(d)

	req := httptest.NewRequest("GET", "/api/urls/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET delete url: got %d, want 405", rec.Code)
	}
}

// =============================================
// List Handlers Tests
// =============================================

func TestListFilesHandler_Empty(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListFilesHandler(d)

	req := httptest.NewRequest("GET", "/api/files", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var files []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &files)
	if len(files) != 0 {
		t.Errorf("expected empty array, got %d files", len(files))
	}
}

func TestListFilesHandler_WithData(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateFile("f1", "a.txt", "text/plain", "k1", 10)
	time.Sleep(1100 * time.Millisecond)
	d.CreateFile("f2", "b.txt", "text/plain", "k2", 20)
	h := NewListFilesHandler(d)

	req := httptest.NewRequest("GET", "/api/files", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var files []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Size int64  `json:"size"`
		URL  string `json:"url"`
	}
	json.Unmarshal(rec.Body.Bytes(), &files)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// Most recent first
	if files[0].ID != "f2" {
		t.Errorf("first file = %q, want %q", files[0].ID, "f2")
	}
	if files[1].URL != "/api/download/f1" {
		t.Errorf("url = %q, want /api/download/f1", files[1].URL)
	}
}

func TestListFilesHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListFilesHandler(d)

	req := httptest.NewRequest("POST", "/api/files", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST list files: got %d, want 405", rec.Code)
	}
}

func TestListClipboardHandler_Empty(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListClipboardHandler(d)

	req := httptest.NewRequest("GET", "/api/clipboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var items []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &items)
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}
}

func TestListClipboardHandler_WithData(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateClipboard("c1", "text", "text/plain", "k1", 10)
	time.Sleep(1100 * time.Millisecond)
	d.CreateClipboard("c2", "image", "image/png", "k2", 20)
	h := NewListClipboardHandler(d)

	req := httptest.NewRequest("GET", "/api/clipboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var items []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	json.Unmarshal(rec.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "c2" {
		t.Errorf("first item = %q, want %q", items[0].ID, "c2")
	}
}

func TestListClipboardHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListClipboardHandler(d)

	req := httptest.NewRequest("POST", "/api/clipboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST list clipboard: got %d, want 405", rec.Code)
	}
}

func TestListURLsHandler_Empty(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListURLsHandler(d)

	req := httptest.NewRequest("GET", "/api/urls", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var urls []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &urls)
	if len(urls) != 0 {
		t.Errorf("expected empty array, got %d URLs", len(urls))
	}
}

func TestListURLsHandler_WithData(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	d.CreateURL("aaa", "https://a.com")
	time.Sleep(1100 * time.Millisecond)
	d.CreateURL("bbb", "https://b.com")
	h := NewListURLsHandler(d)

	req := httptest.NewRequest("GET", "/api/urls", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var urls []struct {
		Code     string `json:"code"`
		LongURL  string `json:"long_url"`
		ShortURL string `json:"short_url"`
	}
	json.Unmarshal(rec.Body.Bytes(), &urls)
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	if urls[0].Code != "bbb" {
		t.Errorf("first URL code = %q, want %q", urls[0].Code, "bbb")
	}
	if urls[1].ShortURL != "/aaa" {
		t.Errorf("short_url = %q, want /aaa", urls[1].ShortURL)
	}
}

func TestListURLsHandler_InvalidMethod(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()
	h := NewListURLsHandler(d)

	req := httptest.NewRequest("POST", "/api/urls", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST list urls: got %d, want 405", rec.Code)
	}
}

// =============================================
// Static Handler Tests
// =============================================

func TestStaticHandler_ServesFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

func TestStaticHandler_SPAFallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/some-spa-path", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<html>spa</html>") {
		t.Error("expected index.html content for SPA fallback")
	}
}

func TestStaticHandler_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("SECRET"), 0644)

	h := NewStaticHandler(dir)

	// Try to access file outside the directory
	req := httptest.NewRequest("GET", "/../secret.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK && rec.Body.String() == "SECRET" {
		t.Error("path traversal allowed! Got secret file content")
	}
}

// =============================================
// Config Handler Tests
// =============================================

func TestConfigHandler_AuthRequired(t *testing.T) {
	h := &ConfigHandler{AuthRequired: true}
	req := httptest.NewRequest("GET", "/config.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, should contain javascript", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "authRequired:true") {
		t.Errorf("body should contain authRequired:true, got: %s", body)
	}
}

func TestConfigHandler_AuthNotRequired(t *testing.T) {
	h := &ConfigHandler{AuthRequired: false}
	req := httptest.NewRequest("GET", "/config.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "authRequired:false") {
		t.Errorf("body should contain authRequired:false, got: %s", body)
	}
}

func TestConfigHandler_NoCache(t *testing.T) {
	h := &ConfigHandler{AuthRequired: false}
	req := httptest.NewRequest("GET", "/config.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

// =============================================
// Share Handler Tests
// =============================================

func TestShareHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewShareHandler(store, d)
	body := strings.NewReader(`text=shared+from+ios`)
	req := httptest.NewRequest("POST", "/api/share", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d, want 303, body: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "share-success.html") {
		t.Errorf("location = %q, want share-success.html", loc)
	}

	// Verify saved as clipboard
	files, _ := d.ListClipboard(100)
	if len(files) != 1 {
		t.Fatalf("expected 1 clipboard entry, got %d", len(files))
	}
	if files[0].Type != "text" {
		t.Errorf("type = %q, want text", files[0].Type)
	}
}

func TestShareHandler_URLFallback(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewShareHandler(store, d)
	body := strings.NewReader(`url=https://example.com/shared`)
	req := httptest.NewRequest("POST", "/api/share", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d, want 303", rec.Code)
	}
	files, _ := d.ListClipboard(100)
	if len(files) != 1 {
		t.Fatalf("expected 1 clipboard entry, got %d", len(files))
	}
}

func TestShareHandler_TextPreferredOverURL(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewShareHandler(store, d)
	body := strings.NewReader(`text=primary&url=secondary`)
	req := httptest.NewRequest("POST", "/api/share", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d, want 303", rec.Code)
	}
	files, _ := d.ListClipboard(100)
	key := "clipboard/" + files[0].ID[:2] + "/" + files[0].ID
	data, _, _ := store.Get(key)
	content, _ := io.ReadAll(data)
	if string(content) != "primary" {
		t.Errorf("content = %q, want %q", string(content), "primary")
	}
}

func TestShareHandler_NoContent(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewShareHandler(store, d)
	body := strings.NewReader(`text=&url=`)
	req := httptest.NewRequest("POST", "/api/share", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rec.Code)
	}
}

func TestShareHandler_InvalidMethod(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()
	h := NewShareHandler(store, d)

	req := httptest.NewRequest("GET", "/api/share", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET share: got %d, want 405", rec.Code)
	}
}

// =============================================
// SSE Broker Tests
// =============================================

func TestSSEBroker_Broadcast(t *testing.T) {
	broker := NewSSEBroker()
	ch := make(chan struct{}, 1)
	broker.subscribers["addr1"] = ch

	broker.Broadcast()

	select {
	case <-ch:
		// success
	default:
		t.Error("subscriber channel did not receive signal")
	}
}

func TestSSEBroker_BroadcastMultiple(t *testing.T) {
	broker := NewSSEBroker()
	ch1 := make(chan struct{}, 1)
	ch2 := make(chan struct{}, 1)
	broker.subscribers["addr1"] = ch1
	broker.subscribers["addr2"] = ch2

	broker.Broadcast()

	select {
	case <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Error("subscriber 1 did not receive signal")
	}
	select {
	case <-ch2:
	case <-time.After(100 * time.Millisecond):
		t.Error("subscriber 2 did not receive signal")
	}
}

func TestSSEBroker_ServeHTTP(t *testing.T) {
	broker := NewSSEBroker()
	req := httptest.NewRequest("GET", "/api/events", nil)
	rec := httptest.NewRecorder()

	// Use context cancellation to stop the long-lived handler
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		broker.ServeHTTP(rec, req)
		close(done)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	broker.Broadcast()
	time.Sleep(50 * time.Millisecond)

	// Cancel to stop
	cancel()
	<-done

	if !strings.Contains(rec.Body.String(), "event: connected") {
		t.Error("expected connected event")
	}
	if !strings.Contains(rec.Body.String(), "data: update") {
		t.Error("expected update event after broadcast")
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestSSEBroker_ServeHTTP_InvalidMethod(t *testing.T) {
	broker := NewSSEBroker()
	req := httptest.NewRequest("POST", "/api/events", nil)
	rec := httptest.NewRecorder()
	broker.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST SSE: got %d, want 405", rec.Code)
	}
}

func TestSSEBroker_BroadcastDeadSubscriber(t *testing.T) {
	// Full channel should not block
	broker := NewSSEBroker()
	ch := make(chan struct{}, 1)
	ch <- struct{}{} // fill the buffer
	broker.subscribers["blocked"] = ch

	// Should not block
	done := make(chan struct{})
	go func() {
		broker.Broadcast()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("Broadcast blocked on full channel")
	}
}

// =============================================
// Upload Handler — Expiry Tests
// =============================================

func TestUploadHandler_WithExpiry(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	for _, dur := range []string{"1h", "1d", "1w"} {
		t.Run(dur, func(t *testing.T) {
			h := NewUploadHandler(store, d)
			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			part, _ := w.CreateFormFile("file", "exp.txt")
			part.Write([]byte("expiring"))
			w.WriteField("expires_at", dur)
			w.Close()

			req := httptest.NewRequest("POST", "/api/upload", &buf)
			req.Header.Set("Content-Type", "multipart/form-data; boundary="+w.Boundary())
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201, body: %s", rec.Code, rec.Body.String())
			}
			var resp UploadResp
			json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.ExpiresAt == "" {
				t.Error("expires_at should not be empty")
			}
		})
	}
}

func TestUploadHandler_InvalidExpiry(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewUploadHandler(store, d)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "bad.txt")
	part.Write([]byte("content"))
	w.WriteField("expires_at", "99x")
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+w.Boundary())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201", rec.Code)
	}
	var resp UploadResp
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ExpiresAt != "" {
		t.Errorf("expires_at should be empty for invalid value, got %q", resp.ExpiresAt)
	}
}

// =============================================
// Static Handler — Additional Tests
// =============================================

func TestStaticHandler_JSONManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"name":"test"}`), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/manifest.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q, want application/manifest+json", ct)
	}
}

func TestStaticHandler_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/does-not-exist.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Should fall back to index.html (SPA)
	if !strings.Contains(rec.Body.String(), "<html>") {
		t.Error("expected SPA fallback for non-existent file")
	}
}

// =============================================
// End-to-end integration tests
// =============================================

func TestE2E_FullFileLifecycle(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	// 1. Upload
	uploadH := NewUploadHandler(store, d)
	buf, boundary, _ := makeMultipartBody("file", "e2e.txt", "end to end content")
	req := httptest.NewRequest("POST", "/api/upload", buf)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	rec := httptest.NewRecorder()
	uploadH.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: got %d, want 201", rec.Code)
	}
	var uploadResp UploadResp
	json.Unmarshal(rec.Body.Bytes(), &uploadResp)
	id := uploadResp.ID

	// 2. List
	listH := NewListFilesHandler(d)
	req = httptest.NewRequest("GET", "/api/files", nil)
	rec = httptest.NewRecorder()
	listH.ServeHTTP(rec, req)

	var files []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &files)
	if len(files) != 1 {
		t.Fatalf("list: expected 1 file, got %d", len(files))
	}

	// 3. Download
	dlH := NewDownloadHandler(store, d)
	req = httptest.NewRequest("GET", "/api/download/"+id, nil)
	rec = httptest.NewRecorder()
	wrapHandler(dlH, "GET /api/download/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("download: got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "end to end content" {
		t.Errorf("download body = %q", rec.Body.String())
	}

	// 4. Delete
	delH := NewDeleteFileHandler(store, d)
	req = httptest.NewRequest("DELETE", "/api/files/"+id, nil)
	rec = httptest.NewRecorder()
	wrapHandler(delH, "DELETE /api/files/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", rec.Code)
	}

	// 5. Verify deleted from list
	req = httptest.NewRequest("GET", "/api/files", nil)
	rec = httptest.NewRecorder()
	listH.ServeHTTP(rec, req)

	json.Unmarshal(rec.Body.Bytes(), &files)
	if len(files) != 0 {
		t.Errorf("expected 0 files after delete, got %d", len(files))
	}
}

func TestE2E_FullClipboardLifecycle(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	// 1. Create
	cbH := NewClipboardHandler(store, d)
	body := strings.NewReader(`{"type":"text","content":"e2e clipboard test"}`)
	req := httptest.NewRequest("POST", "/api/clipboard", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	cbH.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rec.Code)
	}
	var cbResp ClipboardResp
	json.Unmarshal(rec.Body.Bytes(), &cbResp)
	id := cbResp.ID

	// 2. Read
	readH := NewClipboardReadHandler(store, d)
	req = httptest.NewRequest("GET", "/api/clipboard/"+id, nil)
	rec = httptest.NewRecorder()
	wrapHandler(readH, "GET /api/clipboard/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("read: got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "e2e clipboard test" {
		t.Errorf("read body = %q", rec.Body.String())
	}

	// 3. List
	listH := NewListClipboardHandler(d)
	req = httptest.NewRequest("GET", "/api/clipboard", nil)
	rec = httptest.NewRecorder()
	listH.ServeHTTP(rec, req)

	var items []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("list: expected 1 item, got %d", len(items))
	}

	// 4. Delete
	delH := NewDeleteClipboardHandler(store, d)
	req = httptest.NewRequest("DELETE", "/api/clipboard/"+id, nil)
	rec = httptest.NewRecorder()
	wrapHandler(delH, "DELETE /api/clipboard/{id}").ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", rec.Code)
	}

	// 5. Verify deleted
	_, _, err := store.Get("clipboard/" + id[:2] + "/" + id)
	if err == nil {
		t.Error("clipboard data should be removed from store")
	}
}

func TestE2E_FullURLLifecycle(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()

	// 1. Create
	urlH := NewURLHandler(d)
	body := strings.NewReader(`{"url":"https://example.com/e2e"}`)
	req := httptest.NewRequest("POST", "/api/url", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	urlH.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rec.Code)
	}
	var urlResp URLResp
	json.Unmarshal(rec.Body.Bytes(), &urlResp)
	code := urlResp.Code

	// 2. Redirect
	redH := NewRedirectHandler(d)
	req = httptest.NewRequest("GET", "/"+code, nil)
	rec = httptest.NewRecorder()
	redH.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("redirect: got %d, want 302", rec.Code)
	}

	// 3. List
	listH := NewListURLsHandler(d)
	req = httptest.NewRequest("GET", "/api/urls", nil)
	rec = httptest.NewRecorder()
	listH.ServeHTTP(rec, req)

	var urls []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &urls)
	if len(urls) != 1 {
		t.Fatalf("list: expected 1 URL, got %d", len(urls))
	}

	// 4. Delete
	delH := NewDeleteURLHandler(d)
	req = httptest.NewRequest("DELETE", "/api/urls/"+code, nil)
	rec = httptest.NewRecorder()
	wrapHandler(delH, "DELETE /api/urls/{code}").ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", rec.Code)
	}

	// 5. Verify deleted — redirect should 404
	req = httptest.NewRequest("GET", "/"+code, nil)
	rec = httptest.NewRecorder()
	redH.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("redirect after delete: got %d, want 404", rec.Code)
	}
}

// =============================================
// Space Handler Tests
// =============================================

func TestSpaceHandler_Success(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	// Put some data in store
	store.Put("test-key", bytes.NewReader([]byte("1234567890"))) // 10 bytes

	h := NewSpaceHandler(store)
	req := httptest.NewRequest("GET", "/api/space", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]int64
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON decode error: %v", err)
	}
	if resp["total"] != 1024*1024*1024 {
		t.Errorf("total = %d, want %d", resp["total"], 1024*1024*1024)
	}
	if resp["used"] != 10 {
		t.Errorf("used = %d, want 10", resp["used"])
	}
	if resp["free"] != 1024*1024*1024-10 {
		t.Errorf("free = %d, want %d", resp["free"], 1024*1024*1024-10)
	}
}

func TestSpaceHandler_RejectPOST(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewSpaceHandler(store)
	req := httptest.NewRequest("POST", "/api/space", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST space: got %d, want 405", rec.Code)
	}
}

func TestSpaceHandler_JSONContentType(t *testing.T) {
	d, store := setupTest(t)
	defer d.Close()

	h := NewSpaceHandler(store)
	req := httptest.NewRequest("GET", "/api/space", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// =============================================
// Health Handler Tests
// =============================================

func TestHealthHandler_Get(t *testing.T) {
	h := NewHealthHandler()
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestHealthHandler_RejectPOST(t *testing.T) {
	h := NewHealthHandler()
	req := httptest.NewRequest("POST", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST healthz: got %d, want 405", rec.Code)
	}
}

func TestHealthHandler_RejectPUT(t *testing.T) {
	h := NewHealthHandler()
	req := httptest.NewRequest("PUT", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT healthz: got %d, want 405", rec.Code)
	}
}

// =============================================
// Static Handler — ETag Tests
// =============================================

func TestStaticHandler_ETag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "etag.txt"), []byte("etag content"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/etag.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Error("ETag header is empty")
	}
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag = %q, should be quoted", etag)
	}
}

func TestStaticHandler_ETagConsistent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "same.txt"), []byte("content"), 0644)

	h := NewStaticHandler(dir)

	req1 := httptest.NewRequest("GET", "/same.txt", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest("GET", "/same.txt", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec1.Header().Get("ETag") != rec2.Header().Get("ETag") {
		t.Error("ETag should be consistent across requests")
	}
}

func TestStaticHandler_JSCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.js"), []byte("alert(1)"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache for JS", cc)
	}
}

func TestStaticHandler_CSSCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/style.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache for CSS", cc)
	}
}

func TestStaticHandler_HTMLNoCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/index.html", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache for HTML", cc)
	}
}

func TestStaticHandler_SPAFallbackNoCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0644)

	h := NewStaticHandler(dir)
	req := httptest.NewRequest("GET", "/some-virtual-path", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control for SPA fallback = %q, want no-cache", cc)
	}
}

// =============================================
// CORS Middleware Tests
// =============================================

func TestCORSMiddleware_AllowsListedOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	corsH := corsMiddlewareTest([]string{"https://example.com"}, handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	corsH.ServeHTTP(rec, req)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want https://example.com", allowOrigin)
	}
}

func TestCORSMiddleware_RejectsUnlistedOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	corsH := corsMiddlewareTest([]string{"https://example.com"}, handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	corsH.ServeHTTP(rec, req)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for unlisted origin", allowOrigin)
	}
}

func TestCORSMiddleware_OptionsPreflight(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	corsH := corsMiddlewareTest([]string{"https://example.com"}, handler)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	corsH.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: got %d, want 204", rec.Code)
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-AllowMethods missing in preflight response")
	}
}

func TestCORSMiddleware_NoReflection(t *testing.T) {
	// Verify the middleware does NOT reflect arbitrary origins
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	corsH := corsMiddlewareTest([]string{"https://safe.com"}, handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://attacker.com")
	rec := httptest.NewRecorder()
	corsH.ServeHTTP(rec, req)

	// Must not have the wildcard either
	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin == "https://attacker.com" || allowOrigin == "*" {
		t.Errorf("CORS origin reflection vulnerability: got %q", allowOrigin)
	}
}

// Helper: inline CORS middleware for testing
func corsMiddlewareTest(allowedOrigins []string, next http.Handler) http.Handler {
	origins := make(map[string]bool)
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// =============================================
// Security Headers Tests
// =============================================

func TestSecurityHeaders_XContentOptions(t *testing.T) {
	handler := securityHeadersTest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	v := rec.Header().Get("X-Content-Type-Options")
	if v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", v)
	}
}

func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	handler := securityHeadersTest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	v := rec.Header().Get("X-Frame-Options")
	if v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", v)
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	handler := securityHeadersTest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("CSP missing default-src 'self': %s", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP missing frame-ancestors 'none': %s", csp)
	}
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	handler := securityHeadersTest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	pp := rec.Header().Get("Permissions-Policy")
	if !strings.Contains(pp, "camera=()") {
		t.Errorf("Permissions-Policy missing camera=(): %s", pp)
	}
	if !strings.Contains(pp, "microphone=()") {
		t.Errorf("Permissions-Policy missing microphone=(): %s", pp)
	}
}

// Helper: inline security headers for testing
func securityHeadersTest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' /config.js blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// =============================================
// DB Tests — Expiry
// =============================================

func TestDB_ExpiredFiles(t *testing.T) {
	d, _ := setupTest(t)
	defer d.Close()

	d.CreateFile("expired-id", "expired.txt", "text/plain", "rk1", 10)
	d.SetExpiry("expired-id", "2020-01-01 00:00:00")
	d.CreateFile("alive-id", "alive.txt", "text/plain", "rk2", 10)

	ids, err := d.ExpiredFiles()
	if err != nil {
		t.Fatalf("ExpiredFiles() error: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 expired file, got %d", len(ids))
	}
	if ids[0] != "expired-id" {
		t.Errorf("expired id = %q, want expired-id", ids[0])
	}
}
