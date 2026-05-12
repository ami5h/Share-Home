package db

import (
	"database/sql"
	"time"

	"share-home/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, size INTEGER NOT NULL,
			mime TEXT NOT NULL, rgw_key TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')), expires_at TEXT,
			download_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS urls (
			code TEXT PRIMARY KEY, long_url TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS clipboard (
			id TEXT PRIMARY KEY, type TEXT NOT NULL CHECK (type IN ('text','image')),
			size INTEGER NOT NULL, mime TEXT NOT NULL, rgw_key TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return nil, err
	}
	// Migrate: add download_count column to existing files tables
		conn.Exec("ALTER TABLE files ADD COLUMN download_count INTEGER NOT NULL DEFAULT 0")

	return &DB{conn: conn}, nil
}

func (d *DB) CreateFile(id, name, mime, rgwKey string, size int64) error {
	_, err := d.conn.Exec(
		"INSERT INTO files (id, name, size, mime, rgw_key) VALUES (?, ?, ?, ?, ?)",
		id, name, size, mime, rgwKey,
	)
	return err
}

func (d *DB) CreateFileWithExpiry(id, name, mime, rgwKey string, size int64, expiresAt *time.Time) error {
	var ea sql.NullString
	if expiresAt != nil {
		ea = sql.NullString{String: expiresAt.Format("2006-01-02 15:04:05"), Valid: true}
	}
	_, err := d.conn.Exec(
		"INSERT INTO files (id, name, size, mime, rgw_key, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, size, mime, rgwKey, ea,
	)
	return err
}

func (d *DB) ExpiredFiles() ([]string, error) {
	rows, err := d.conn.Query(
		"SELECT id FROM files WHERE expires_at IS NOT NULL AND expires_at < datetime('now')",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// PurgeExpired deletes expired files from the DB and returns their rgw_keys for store cleanup.
func (d *DB) PurgeExpired() ([]string, error) {
	rows, err := d.conn.Query(
		"SELECT rgw_key FROM files WHERE expires_at IS NOT NULL AND expires_at < datetime('now')",
	)
	if err != nil {
		return nil, err
	}
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	rows.Close()

	if len(keys) > 0 {
		_, err = d.conn.Exec("DELETE FROM files WHERE expires_at IS NOT NULL AND expires_at < datetime('now')")
	}
	return keys, err
}

func (d *DB) GetFile(id string) (*model.FileMeta, error) {
	var f model.FileMeta
	var ca string
	var ea sql.NullString
	err := d.conn.QueryRow(
		"SELECT id, name, size, mime, rgw_key, created_at, expires_at, download_count FROM files WHERE id = ?", id,
	).Scan(&f.ID, &f.Name, &f.Size, &f.MIME, &f.RGWKey, &ca, &ea, &f.Downloads)
	if err != nil {
		return nil, err
	}
	f.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
	if ea.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", ea.String)
		f.ExpiresAt = &t
	}
	return &f, nil
}

func (d *DB) CreateURL(code, longURL string) error {
	_, err := d.conn.Exec(
		"INSERT INTO urls (code, long_url) VALUES (?, ?)", code, longURL,
	)
	return err
}

func (d *DB) GetURL(code string) (string, error) {
	var url string
	err := d.conn.QueryRow("SELECT long_url FROM urls WHERE code = ?", code).Scan(&url)
	return url, err
}

func (d *DB) CreateClipboard(id, typ, mime, rgwKey string, size int64) error {
	_, err := d.conn.Exec(
		"INSERT INTO clipboard (id, type, size, mime, rgw_key) VALUES (?, ?, ?, ?, ?)",
		id, typ, size, mime, rgwKey,
	)
	return err
}

func (d *DB) GetClipboard(id string) (*model.ClipboardEntry, error) {
	var c model.ClipboardEntry
	var ca string
	err := d.conn.QueryRow(
		"SELECT id, type, size, mime, rgw_key, created_at FROM clipboard WHERE id = ?", id,
	).Scan(&c.ID, &c.Type, &c.Size, &c.MIME, &c.RGWKey, &ca)
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
	return &c, nil
}

func (d *DB) ListFiles(limit int) ([]model.FileMeta, error) {
	rows, err := d.conn.Query("SELECT id, name, size, mime, rgw_key, created_at, download_count FROM files ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []model.FileMeta
	for rows.Next() {
		var f model.FileMeta
		var ca string
		if err := rows.Scan(&f.ID, &f.Name, &f.Size, &f.MIME, &f.RGWKey, &ca, &f.Downloads); err != nil {
			return nil, err
		}
		f.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
		files = append(files, f)
	}
	return files, nil
}

func (d *DB) DeleteFile(id string) error {
	_, err := d.conn.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

func (d *DB) IncrementDownload(id string) error {
	_, err := d.conn.Exec("UPDATE files SET download_count = download_count + 1 WHERE id = ?", id)
	return err
}

func (d *DB) ListURLs(limit int) ([]model.URLEntry, error) {
	rows, err := d.conn.Query("SELECT code, long_url, created_at FROM urls ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var urls []model.URLEntry
	for rows.Next() {
		var u model.URLEntry
		var ca string
		if err := rows.Scan(&u.Code, &u.LongURL, &ca); err != nil {
			return nil, err
		}
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
		urls = append(urls, u)
	}
	return urls, nil
}

func (d *DB) DeleteURL(code string) error {
	_, err := d.conn.Exec("DELETE FROM urls WHERE code = ?", code)
	return err
}

func (d *DB) ListClipboard(limit int) ([]model.ClipboardEntry, error) {
	rows, err := d.conn.Query("SELECT id, type, size, mime, rgw_key, created_at FROM clipboard ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.ClipboardEntry
	for rows.Next() {
		var c model.ClipboardEntry
		var ca string
		if err := rows.Scan(&c.ID, &c.Type, &c.Size, &c.MIME, &c.RGWKey, &ca); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
		items = append(items, c)
	}
	return items, nil
}

func (d *DB) DeleteClipboard(id string) error {
	_, err := d.conn.Exec("DELETE FROM clipboard WHERE id = ?", id)
	return err
}

func (d *DB) SetExpiry(id string, expiresAt string) error {
	_, err := d.conn.Exec("UPDATE files SET expires_at = ? WHERE id = ?", expiresAt, id)
	return err
}

func (d *DB) Close() error {
	return d.conn.Close()
}
