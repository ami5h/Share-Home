package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	SMBHost       string
	SMBShare      string
	SMBUsername   string
	SMBPassword   string
	SMBBaseDir    string
	SMBEncryptKey string
	AuthToken     string
	AllowedOrigins []string
	DBPath        string
	ListenAddr    string
}

func Load() (*Config, error) {
	encryptKey := os.Getenv("SMB_ENCRYPT_KEY")
	if encryptKey == "" {
		return nil, fmt.Errorf("SMB_ENCRYPT_KEY is required")
	}

	username := getEnv("SMB_USERNAME", "")
	if username == "" {
		return nil, fmt.Errorf("SMB_USERNAME is required")
	}

	return &Config{
		SMBHost:       getEnv("SMB_HOST", ""),
		SMBShare:      getEnv("SMB_SHARE", ""),
		SMBUsername:   username,
		SMBPassword:   getEnv("SMB_PASSWORD", ""),
		SMBBaseDir:    getEnv("SMB_BASE_DIR", "share-home"),
		SMBEncryptKey: encryptKey,
		AuthToken:     os.Getenv("AUTH_TOKEN"),
		AllowedOrigins: parseListEnv("CORS_ALLOWED_ORIGINS", "http://localhost:8080,https://localhost:8080"),
		DBPath:        getEnv("DB_PATH", "/data/share-home.db"),
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseListEnv(key, defaultCSV string) []string {
	raw := getEnv(key, defaultCSV)
	var list []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			list = append(list, s)
		}
	}
	return list
}
