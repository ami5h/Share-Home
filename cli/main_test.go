package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- fmtSize ---

func TestFmtSize_Bytes(t *testing.T) {
	if got := fmtSize(0); got != "0B" {
		t.Errorf("fmtSize(0) = %q, want 0B", got)
	}
	if got := fmtSize(512); got != "512B" {
		t.Errorf("fmtSize(512) = %q, want 512B", got)
	}
}

func TestFmtSize_KB(t *testing.T) {
	if got := fmtSize(1024); got != "1.0KB" {
		t.Errorf("fmtSize(1024) = %q, want 1.0KB", got)
	}
	if got := fmtSize(1536); got != "1.5KB" {
		t.Errorf("fmtSize(1536) = %q, want 1.5KB", got)
	}
}

func TestFmtSize_MB(t *testing.T) {
	if got := fmtSize(1048576); got != "1.0MB" {
		t.Errorf("fmtSize(1048576) = %q, want 1.0MB", got)
	}
}

func TestFmtSize_GB(t *testing.T) {
	if got := fmtSize(1073741824); got != "1.0GB" {
		t.Errorf("fmtSize(1073741824) = %q, want 1.0GB", got)
	}
}

func TestFmtSize_Negative(t *testing.T) {
	if got := fmtSize(-1); got != "" {
		t.Errorf("fmtSize(-1) = %q, want empty", got)
	}
}

// --- extractFilename ---

func TestExtractFilename_Quoted(t *testing.T) {
	fn := extractFilename(`attachment; filename="report.pdf"`, "fallback")
	if fn != "report.pdf" {
		t.Errorf("got %q, want report.pdf", fn)
	}
}

func TestExtractFilename_Unquoted(t *testing.T) {
	fn := extractFilename(`attachment; filename=hello.txt`, "fallback")
	if fn != "hello.txt" {
		t.Errorf("got %q, want hello.txt", fn)
	}
}

func TestExtractFilename_Missing(t *testing.T) {
	fn := extractFilename(`attachment`, "fallback")
	if fn != "fallback" {
		t.Errorf("got %q, want fallback", fn)
	}
}

func TestExtractFilename_Empty(t *testing.T) {
	fn := extractFilename("", "fallback")
	if fn != "fallback" {
		t.Errorf("got %q, want fallback", fn)
	}
}

// --- maskToken ---

func TestMaskToken_Empty(t *testing.T) {
	if got := maskToken(""); got != "" {
		t.Errorf("maskToken(\"\") = %q, want empty", got)
	}
}

func TestMaskToken_Short(t *testing.T) {
	if got := maskToken("abc"); got != "****" {
		t.Errorf("maskToken(\"abc\") = %q, want ****", got)
	}
}

func TestMaskToken_Normal(t *testing.T) {
	got := maskToken("mytoken123")
	if got != "myto****" {
		t.Errorf("maskToken(\"mytoken123\") = %q, want myto****", got)
	}
}

// --- parseBoolFlag ---

func TestParseBoolFlag_NoFlag(t *testing.T) {
	args := []string{"file.txt", "--expires", "1h"}
	parsed, found := parseBoolFlag(args, "--json")
	if found {
		t.Error("should not have found --json")
	}
	if len(parsed) != 3 {
		t.Errorf("parsed = %v, want 3 items", parsed)
	}
}

func TestParseBoolFlag_HasFlag(t *testing.T) {
	args := []string{"file.txt", "--json"}
	parsed, found := parseBoolFlag(args, "--json")
	if !found {
		t.Error("should have found --json")
	}
	if len(parsed) != 1 || parsed[0] != "file.txt" {
		t.Errorf("parsed = %v, want [file.txt]", parsed)
	}
}

func TestParseBoolFlag_Middle(t *testing.T) {
	args := []string{"--json", "file.txt"}
	parsed, found := parseBoolFlag(args, "--json")
	if !found {
		t.Error("should have found --json")
	}
	if len(parsed) != 1 || parsed[0] != "file.txt" {
		t.Errorf("parsed = %v, want [file.txt]", parsed)
	}
}

// --- loadConfig / saveConfig ---

func TestSaveAndLoadConfig(t *testing.T) {
	orig := configDir
	defer func() { configDir = orig }()

	tmp := t.TempDir()
	configDir = func() string { return tmp }

	cfg := Config{URL: "http://localhost:8080", Token: "secret"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig error: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if loaded.URL != "http://localhost:8080" {
		t.Errorf("URL = %q, want http://localhost:8080", loaded.URL)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmp := t.TempDir()
	configDir = func() string { return tmp }

	// Write config file directly
	os.WriteFile(configPath(), []byte(`{"url":"http://test.local:9090","token":"filetoken"}`), 0644)

	cfg := loadConfig()
	if cfg.URL != "http://test.local:9090" {
		t.Errorf("URL = %q, want http://test.local:9090", cfg.URL)
	}
	if cfg.Token != "filetoken" {
		t.Errorf("Token = %q, want filetoken", cfg.Token)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	configDir = func() string { return tmp }

	os.WriteFile(configPath(), []byte(`{"url":"http://file.local","token":"filetoken"}`), 0644)

	os.Setenv("SHARE_HOME_URL", "http://env.local")
	os.Setenv("SHARE_HOME_TOKEN", "envtoken")
	defer func() {
		os.Unsetenv("SHARE_HOME_URL")
		os.Unsetenv("SHARE_HOME_TOKEN")
	}()

	cfg := loadConfig()
	if cfg.URL != "http://env.local" {
		t.Errorf("URL = %q, want http://env.local (env should override file)", cfg.URL)
	}
	if cfg.Token != "envtoken" {
		t.Errorf("Token = %q, want envtoken (env should override file)", cfg.Token)
	}
}

// --- do (URL building with token) ---

func TestDo_URLWithToken(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL, Token: "mytoken"}
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	resp.Body.Close()

	if gotURL != "/api/files?token=mytoken" {
		t.Errorf("URL = %q, want /api/files?token=mytoken", gotURL)
	}
}

func TestDo_URLWithoutToken(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL}
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	resp.Body.Close()

	if gotPath != "/api/files" {
		t.Errorf("Path = %q, want /api/files", gotPath)
	}
}

func TestDo_AuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL, Token: "secret"}
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q, want Bearer secret", gotAuth)
	}
}

func TestDo_ContentTypeHeader(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL}
	resp, err := do(cfg, "POST", "/api/clipboard", []byte(`{}`), "application/json")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	resp.Body.Close()

	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
}

// --- doJSON ---

func TestDoJSON_SendsBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(201)
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL}
	resp, err := doJSON(cfg, "POST", "/api/url", map[string]string{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("doJSON error: %v", err)
	}
	resp.Body.Close()

	if gotBody != `{"url":"https://example.com"}` {
		t.Errorf("body = %q, want {\"url\":\"https://example.com\"}", gotBody)
	}
}

// --- Integration: list with empty response ---

func TestIntegration_ListFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL}
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var files []fileEntry
	json.NewDecoder(resp.Body).Decode(&files)
	if len(files) != 0 {
		t.Errorf("expected empty array, got %d files", len(files))
	}
}

func TestIntegration_ListFilesWithData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":"abc123","name":"test.txt","size":100,"mime":"text/plain","url":"/api/download/abc123","downloads":5}]`))
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL}
	resp, err := do(cfg, "GET", "/api/files", nil, "")
	if err != nil {
		t.Fatalf("do error: %v", err)
	}
	defer resp.Body.Close()

	var files []fileEntry
	json.NewDecoder(resp.Body).Decode(&files)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "test.txt" {
		t.Errorf("name = %q, want test.txt", files[0].Name)
	}
	if files[0].DL != 5 {
		t.Errorf("downloads = %d, want 5", files[0].DL)
	}
}

// --- fileIcon ---

func TestFileIcon_Image(t *testing.T) {
	noColor = true
	defer func() { noColor = false }()

	if got := fileIcon("image/png"); got != "🖼" {
		t.Errorf("fileIcon(\"image/png\") = %q, want 🖼", got)
	}
}

func TestFileIcon_Video(t *testing.T) {
	noColor = true
	defer func() { noColor = false }()

	if got := fileIcon("video/mp4"); got != "🎬" {
		t.Errorf("fileIcon(\"video/mp4\") = %q, want 🎬", got)
	}
}

func TestFileIcon_Text(t *testing.T) {
	noColor = true
	defer func() { noColor = false }()

	if got := fileIcon("text/plain"); got != "📄" {
		t.Errorf("fileIcon(\"text/plain\") = %q, want 📄", got)
	}
}

func TestFileIcon_Default(t *testing.T) {
	noColor = true
	defer func() { noColor = false }()

	if got := fileIcon("application/zip"); got != "📎" {
		t.Errorf("fileIcon(\"application/zip\") = %q, want 📎", got)
	}
}

// --- configPath ---

func TestConfigPath_Default(t *testing.T) {
	tmp := t.TempDir()
	orig := configDir
	defer func() { configDir = orig }()
	configDir = func() string { return tmp }

	p := configPath()
	if !filepath.IsAbs(p) {
		t.Errorf("configPath = %q, want absolute", p)
	}
	if !strings.HasSuffix(p, "config.json") {
		t.Errorf("configPath = %q, should end with config.json", p)
	}
}
