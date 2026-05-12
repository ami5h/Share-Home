package config_test

import (
	"os"
	"strings"
	"testing"

	"share-home/internal/config"
)

func TestLoad_MissingEncryptKey(t *testing.T) {
	os.Unsetenv("SMB_ENCRYPT_KEY")
	os.Setenv("SMB_PASSWORD", "pass")
	defer os.Unsetenv("SMB_PASSWORD")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMB_ENCRYPT_KEY is missing")
	}
	if !strings.Contains(err.Error(), "SMB_ENCRYPT_KEY") {
		t.Errorf("error should mention SMB_ENCRYPT_KEY, got: %v", err)
	}
}

func TestLoad_EmptyEncryptKey(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "")
	os.Setenv("SMB_PASSWORD", "pass")
	defer os.Unsetenv("SMB_PASSWORD")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMB_ENCRYPT_KEY is empty")
	}
}

func TestLoad_MissingUsername(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "test-key")
	os.Unsetenv("SMB_USERNAME")
	defer os.Unsetenv("SMB_ENCRYPT_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when SMB_USERNAME is missing")
	}
	if !strings.Contains(err.Error(), "SMB_USERNAME") {
		t.Errorf("error should mention SMB_USERNAME, got: %v", err)
	}
}

func TestLoad_WithEncryptKey(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "test-key")
	os.Setenv("SMB_HOST", "192.168.1.100")
	os.Setenv("SMB_SHARE", "myshare")
	os.Setenv("SMB_USERNAME", "testuser")
	os.Setenv("SMB_PASSWORD", "testpass")
	os.Setenv("AUTH_TOKEN", "my-auth-token")
	defer func() {
		os.Unsetenv("SMB_ENCRYPT_KEY")
		os.Unsetenv("SMB_HOST")
		os.Unsetenv("SMB_SHARE")
		os.Unsetenv("SMB_USERNAME")
		os.Unsetenv("SMB_PASSWORD")
		os.Unsetenv("AUTH_TOKEN")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.SMBHost != "192.168.1.100" {
		t.Errorf("SMBHost = %q, want %q", cfg.SMBHost, "192.168.1.100")
	}
	if cfg.SMBShare != "myshare" {
		t.Errorf("SMBShare = %q, want %q", cfg.SMBShare, "myshare")
	}
	if cfg.SMBUsername != "testuser" {
		t.Errorf("SMBUsername = %q, want %q", cfg.SMBUsername, "testuser")
	}
	if cfg.SMBEncryptKey != "test-key" {
		t.Errorf("SMBEncryptKey = %q, want %q", cfg.SMBEncryptKey, "test-key")
	}
	if cfg.AuthToken != "my-auth-token" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "my-auth-token")
	}
}

func TestLoad_ListenAddr(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "key")
	os.Setenv("SMB_USERNAME", "testuser")
	os.Setenv("LISTEN_ADDR", ":9999")
	defer func() {
		os.Unsetenv("SMB_ENCRYPT_KEY")
		os.Unsetenv("SMB_USERNAME")
		os.Unsetenv("LISTEN_ADDR")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9999")
	}
}

func TestLoad_DBPath(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "key")
	os.Setenv("SMB_USERNAME", "testuser")
	os.Setenv("DB_PATH", "/custom/path/db.sqlite")
	defer func() {
		os.Unsetenv("SMB_ENCRYPT_KEY")
		os.Unsetenv("SMB_USERNAME")
		os.Unsetenv("DB_PATH")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DBPath != "/custom/path/db.sqlite" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/custom/path/db.sqlite")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("SMB_ENCRYPT_KEY", "key")
	os.Setenv("SMB_USERNAME", "testuser")
	os.Unsetenv("SMB_HOST")
	os.Unsetenv("SMB_SHARE")
	os.Unsetenv("SMB_PASSWORD")
	os.Unsetenv("SMB_BASE_DIR")
	os.Unsetenv("AUTH_TOKEN")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("LISTEN_ADDR")
	defer os.Unsetenv("SMB_ENCRYPT_KEY")
	defer os.Unsetenv("SMB_USERNAME")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.SMBHost != "" {
		t.Errorf("SMBHost = %q, want empty", cfg.SMBHost)
	}
	if cfg.SMBShare != "" {
		t.Errorf("SMBShare = %q, want empty", cfg.SMBShare)
	}
	if cfg.SMBUsername != "testuser" {
		t.Errorf("SMBUsername = %q, want %q", cfg.SMBUsername, "testuser")
	}
	if cfg.SMBBaseDir != "share-home" {
		t.Errorf("SMBBaseDir = %q, want %q", cfg.SMBBaseDir, "share-home")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.DBPath != "/data/share-home.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/data/share-home.db")
	}
	if cfg.AuthToken != "" {
		t.Errorf("AuthToken = %q, want empty", cfg.AuthToken)
	}
}
