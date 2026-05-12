package model

import "time"

type FileMeta struct {
	ID        string
	Name      string
	Size      int64
	MIME      string
	RGWKey    string
	CreatedAt time.Time
	ExpiresAt *time.Time
	Downloads int
}

type URLEntry struct {
	Code     string
	LongURL  string
	CreatedAt time.Time
}

type ClipboardEntry struct {
	ID        string
	Type      string
	Size      int64
	MIME      string
	RGWKey    string
	CreatedAt time.Time
}
