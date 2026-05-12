package db_test

import (
	"testing"
	"time"

	"share-home/internal/db"
)

func TestOpenAndCreateTables(t *testing.T) {
	path := t.TempDir() + "/test.db"
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer d.Close()
}

// --- File CRUD ---

func TestFile_CreateAndGet(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	if err := d.CreateFile("f1", "test.txt", "text/plain", "rgw-key-1", 42); err != nil {
		t.Fatalf("CreateFile() error: %v", err)
	}

	meta, err := d.GetFile("f1")
	if err != nil {
		t.Fatalf("GetFile() error: %v", err)
	}
	if meta.ID != "f1" {
		t.Errorf("ID = %q, want %q", meta.ID, "f1")
	}
	if meta.Name != "test.txt" {
		t.Errorf("Name = %q, want %q", meta.Name, "test.txt")
	}
	if meta.Size != 42 {
		t.Errorf("Size = %d, want %d", meta.Size, 42)
	}
	if meta.MIME != "text/plain" {
		t.Errorf("MIME = %q, want %q", meta.MIME, "text/plain")
	}
	if meta.RGWKey != "rgw-key-1" {
		t.Errorf("RGWKey = %q, want %q", meta.RGWKey, "rgw-key-1")
	}
	if meta.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestFile_GetNotFound(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	_, err := d.GetFile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFile_CreateDuplicate(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "a.txt", "text/plain", "key1", 10)
	err := d.CreateFile("f1", "b.txt", "text/plain", "key2", 20)
	if err == nil {
		t.Error("expected error for duplicate file ID")
	}
}

func TestFile_Delete(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "test.txt", "text/plain", "key1", 100)
	if err := d.DeleteFile("f1"); err != nil {
		t.Fatalf("DeleteFile() error: %v", err)
	}
	_, err := d.GetFile("f1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestFile_DeleteNotFound(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	if err := d.DeleteFile("nonexistent"); err != nil {
		t.Fatalf("DeleteFile() error: %v", err)
	}
}

func TestFile_DownloadCounter(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "test.txt", "text/plain", "key1", 10)

	meta, _ := d.GetFile("f1")
	if meta.Downloads != 0 {
		t.Errorf("initial downloads = %d, want 0", meta.Downloads)
	}

	d.IncrementDownload("f1")
	meta, _ = d.GetFile("f1")
	if meta.Downloads != 1 {
		t.Errorf("after 1 increment, downloads = %d, want 1", meta.Downloads)
	}

	d.IncrementDownload("f1")
	d.IncrementDownload("f1")
	meta, _ = d.GetFile("f1")
	if meta.Downloads != 3 {
		t.Errorf("after 3 increments, downloads = %d, want 3", meta.Downloads)
	}
}

func TestFile_DownloadCounterInList(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "a.txt", "text/plain", "k1", 10)
	time.Sleep(1100 * time.Millisecond)
	d.CreateFile("f2", "b.txt", "text/plain", "k2", 20)
	d.IncrementDownload("f2")
	d.IncrementDownload("f2")

	files, _ := d.ListFiles(50)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// f2 is first (DESC), should have 2 downloads
	if files[0].ID != "f2" {
		t.Fatalf("first file = %q, want %q", files[0].ID, "f2")
	}
	if files[0].Downloads != 2 {
		t.Errorf("f2 downloads = %d, want 2", files[0].Downloads)
	}
	// f1 should have 0
	if files[1].Downloads != 0 {
		t.Errorf("f1 downloads = %d, want 0", files[1].Downloads)
	}
}

func TestIncrementDownload_NotFound(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	// Should not error even if file doesn't exist (no rows affected)
	if err := d.IncrementDownload("nonexistent"); err != nil {
		t.Fatalf("IncrementDownload() error: %v", err)
	}
}

func TestFile_WithExpiry(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	future := time.Now().Add(1 * time.Hour)
	if err := d.CreateFileWithExpiry("f1", "a.txt", "text/plain", "k1", 10, &future); err != nil {
		t.Fatalf("CreateFileWithExpiry() error: %v", err)
	}

	meta, err := d.GetFile("f1")
	if err != nil {
		t.Fatalf("GetFile() error: %v", err)
	}
	if meta.ExpiresAt == nil {
		t.Fatal("expires_at should not be nil")
	}
}

func TestFile_NoExpiry(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "a.txt", "text/plain", "k1", 10)
	meta, err := d.GetFile("f1")
	if err != nil {
		t.Fatalf("GetFile() error: %v", err)
	}
	if meta.ExpiresAt != nil {
		t.Errorf("expires_at = %v, want nil", meta.ExpiresAt)
	}
}

func TestExpiredFiles_Purge(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	// File that expired in the past
	past := time.Now().Add(-1 * time.Hour)
	d.CreateFileWithExpiry("expired", "old.txt", "text/plain", "key-old", 10, &past)

	// File that expires in the future
	future := time.Now().Add(1 * time.Hour)
	d.CreateFileWithExpiry("future", "new.txt", "text/plain", "key-new", 10, &future)

	// File with no expiry
	d.CreateFile("none", "plain.txt", "text/plain", "key-none", 10)

	keys, err := d.PurgeExpired()
	if err != nil {
		t.Fatalf("PurgeExpired() error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("purged %d files, want 1", len(keys))
	}
	if keys[0] != "key-old" {
		t.Errorf("purged key = %q, want %q", keys[0], "key-old")
	}

	// Verify expired is gone
	_, err = d.GetFile("expired")
	if err == nil {
		t.Error("expired file should be gone")
	}

	// Verify future still exists
	_, err = d.GetFile("future")
	if err != nil {
		t.Error("future file should still exist")
	}

	// Verify none still exists
	_, err = d.GetFile("none")
	if err != nil {
		t.Error("none expiry file should still exist")
	}
}

// --- URL CRUD ---

func TestURL_CreateAndGet(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	if err := d.CreateURL("abc123", "https://example.com/long"); err != nil {
		t.Fatalf("CreateURL() error: %v", err)
	}

	url, err := d.GetURL("abc123")
	if err != nil {
		t.Fatalf("GetURL() error: %v", err)
	}
	if url != "https://example.com/long" {
		t.Errorf("url = %q, want %q", url, "https://example.com/long")
	}
}

func TestURL_GetNotFound(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	_, err := d.GetURL("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent URL")
	}
}

func TestURL_CreateDuplicate(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateURL("abc", "https://a.com")
	err := d.CreateURL("abc", "https://b.com")
	if err == nil {
		t.Error("expected error for duplicate URL code")
	}
}

func TestURL_Delete(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateURL("abc", "https://example.com")
	if err := d.DeleteURL("abc"); err != nil {
		t.Fatalf("DeleteURL() error: %v", err)
	}
	_, err := d.GetURL("abc")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --- Clipboard CRUD ---

func TestClipboard_CreateAndGet(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	if err := d.CreateClipboard("c1", "text", "text/plain", "rgw-key-c1", 100); err != nil {
		t.Fatalf("CreateClipboard() error: %v", err)
	}

	entry, err := d.GetClipboard("c1")
	if err != nil {
		t.Fatalf("GetClipboard() error: %v", err)
	}
	if entry.ID != "c1" {
		t.Errorf("ID = %q, want %q", entry.ID, "c1")
	}
	if entry.Type != "text" {
		t.Errorf("Type = %q, want %q", entry.Type, "text")
	}
	if entry.MIME != "text/plain" {
		t.Errorf("MIME = %q, want %q", entry.MIME, "text/plain")
	}
	if entry.Size != 100 {
		t.Errorf("Size = %d, want %d", entry.Size, 100)
	}
	if entry.RGWKey != "rgw-key-c1" {
		t.Errorf("RGWKey = %q, want %q", entry.RGWKey, "rgw-key-c1")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestClipboard_Image(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateClipboard("c2", "image", "image/png", "rgw-key-c2", 5000)
	entry, err := d.GetClipboard("c2")
	if err != nil {
		t.Fatalf("GetClipboard() error: %v", err)
	}
	if entry.Type != "image" {
		t.Errorf("Type = %q, want %q", entry.Type, "image")
	}
	if entry.MIME != "image/png" {
		t.Errorf("MIME = %q, want %q", entry.MIME, "image/png")
	}
}

func TestClipboard_GetNotFound(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	_, err := d.GetClipboard("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent clipboard entry")
	}
}

func TestClipboard_CreateDuplicate(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateClipboard("c1", "text", "text/plain", "k1", 10)
	err := d.CreateClipboard("c1", "text", "text/plain", "k2", 20)
	if err == nil {
		t.Error("expected error for duplicate clipboard ID")
	}
}

func TestClipboard_InvalidType(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	err := d.CreateClipboard("c1", "video", "video/mp4", "k1", 100)
	if err == nil {
		t.Error("expected error for invalid clipboard type")
	}
}

func TestClipboard_Delete(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateClipboard("c1", "text", "text/plain", "k1", 100)
	if err := d.DeleteClipboard("c1"); err != nil {
		t.Fatalf("DeleteClipboard() error: %v", err)
	}
	_, err := d.GetClipboard("c1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --- List operations ---

func TestListFiles_Empty(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	files, err := d.ListFiles(50)
	if err != nil {
		t.Fatalf("ListFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListFiles_Multiple(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "a.txt", "text/plain", "k1", 10)
	time.Sleep(1100 * time.Millisecond)
	d.CreateFile("f2", "b.txt", "text/plain", "k2", 20)
	time.Sleep(1100 * time.Millisecond)
	d.CreateFile("f3", "c.txt", "text/plain", "k3", 30)

	files, err := d.ListFiles(50)
	if err != nil {
		t.Fatalf("ListFiles() error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	// DESC by created_at
	if files[0].ID != "f3" {
		t.Errorf("first file ID = %q, want %q", files[0].ID, "f3")
	}
	if files[2].ID != "f1" {
		t.Errorf("last file ID = %q, want %q", files[2].ID, "f1")
	}
}

func TestListFiles_Limit(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateFile("f1", "a.txt", "text/plain", "k1", 10)
	time.Sleep(10 * time.Millisecond)
	d.CreateFile("f2", "b.txt", "text/plain", "k2", 20)
	time.Sleep(10 * time.Millisecond)
	d.CreateFile("f3", "c.txt", "text/plain", "k3", 30)

	files, err := d.ListFiles(2)
	if err != nil {
		t.Fatalf("ListFiles() error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestListURLs_Empty(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	urls, err := d.ListURLs(50)
	if err != nil {
		t.Fatalf("ListURLs() error: %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs, got %d", len(urls))
	}
}

func TestListURLs_Multiple(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateURL("aaa", "https://a.com")
	time.Sleep(1100 * time.Millisecond)
	d.CreateURL("bbb", "https://b.com")

	urls, err := d.ListURLs(50)
	if err != nil {
		t.Fatalf("ListURLs() error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	if urls[0].Code != "bbb" {
		t.Errorf("first URL code = %q, want %q", urls[0].Code, "bbb")
	}
}

func TestListURLs_Limit(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateURL("aaa", "https://a.com")
	time.Sleep(10 * time.Millisecond)
	d.CreateURL("bbb", "https://b.com")
	time.Sleep(10 * time.Millisecond)
	d.CreateURL("ccc", "https://c.com")

	urls, err := d.ListURLs(1)
	if err != nil {
		t.Fatalf("ListURLs() error: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
}

func TestListClipboard_Empty(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	items, err := d.ListClipboard(50)
	if err != nil {
		t.Fatalf("ListClipboard() error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestListClipboard_Multiple(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateClipboard("c1", "text", "text/plain", "k1", 10)
	time.Sleep(1100 * time.Millisecond)
	d.CreateClipboard("c2", "image", "image/png", "k2", 20)

	items, err := d.ListClipboard(50)
	if err != nil {
		t.Fatalf("ListClipboard() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "c2" {
		t.Errorf("first item ID = %q, want %q", items[0].ID, "c2")
	}
}

func TestListClipboard_Limit(t *testing.T) {
	d := setupDB(t)
	defer d.Close()

	d.CreateClipboard("c1", "text", "text/plain", "k1", 10)
	time.Sleep(10 * time.Millisecond)
	d.CreateClipboard("c2", "text", "text/plain", "k2", 20)

	items, err := d.ListClipboard(1)
	if err != nil {
		t.Fatalf("ListClipboard() error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func setupDB(t *testing.T) *db.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return d
}
