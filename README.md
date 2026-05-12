# Share Home

A self-hosted file sharing and clipboard service for your home network. Upload files, save clipboard content, and shorten URLs — accessible from any device on your LAN via a beautiful web interface, CLI, or REST API.

![Platform](https://img.shields.io/badge/platform-docker%20%7C%20linux%20%7C%20macos-blue)
![Go](https://img.shields.io/badge/go-1.24-00ADD8)
![License](https://img.shields.io/badge/license-MIT-lightgrey)

## Features

- **File Upload** — Drag & drop or browse, with configurable expiry (1h / 1d / 1w)
- **Clipboard** — Paste text or images from your device clipboard, access from any browser
- **URL Shortener** — Generate short links with QR codes
- **Bulk Operations** — Select multiple files for batch delete or ZIP download
- **Search** — Filter files, clipboard entries, and links by name
- **QR Codes** — Scan to open links on mobile
- **Live Updates** — SSE-powered real-time history across all tabs
- **CLI Client** — Terminal interface for power users (`share-home upload`, `paste`, `url`, `list`, etc.)
- **Dark Mode** — System-aware with manual toggle
- **PWA** — Install on mobile, offline-capable service worker
- **LAN Auth Bypass** — No token needed on local network; external access requires auth token
- **Encrypted Storage** — Files encrypted with AES-256-GCM on SMB shares
- **Storage Info** — Live disk usage display with used/free capacity
- **Secure** — CSP headers, path traversal protection, CORS, nosniff

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  Docker Container                                    │
│                                                      │
│  ┌──────────┐    ┌──────────────┐    ┌────────────┐ │
│  │  Go API   │───▶│  SQLite DB   │    │  SMB Share  │ │
│  │  Server   │    │  (metadata)  │    │  (files)    │ │
│  │ :8080     │    └──────────────┘    └────────────┘ │
│  │           │                                         │
│  │  ┌─────┐  │    ┌──────────────┐                    │
│  │  │Web │  │───▶│  SSE Broker  │ (live updates)     │
│  │  └─────┘  │    └──────────────┘                    │
│  └──────────┘                                         │
└─────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
    Browser / CLI                     iOS Share Target
    (PWA installable)                  (Shortcuts)
```

### Components

| Directory | Description |
|-----------|-------------|
| `go/` | Go server — API handlers, config, SMB storage, SQLite DB |
| `go/cmd/server/` | Main server entrypoint |
| `go/internal/handlers/` | HTTP handlers (upload, download, clipboard, URL, share, SSE) |
| `go/internal/storage/` | SMB2 storage with AES-256-GCM encryption |
| `go/internal/db/` | SQLite database layer |
| `go/internal/auth/` | Bearer token auth + LAN IP bypass |
| `go/internal/config/` | Environment-based configuration |
| `go/internal/model/` | Shared data models |
| `web/` | Frontend — `index.html`, `app.js`, `style.css`, PWA assets |
| `cli/` | CLI client — standalone Go binary with stdlib only |
| `scripts/` | Helper scripts (SMB mount/umount) |

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/upload` | Upload file (multipart/form-data) |
| `GET` | `/api/download/{id}` | Download file |
| `DELETE` | `/api/files/{id}` | Delete file |
| `POST` | `/api/clipboard` | Save text/image to clipboard |
| `GET` | `/api/clipboard/{id}` | Get clipboard entry |
| `GET` | `/api/clipboard` | List clipboard entries |
| `DELETE` | `/api/clipboard/{id}` | Delete clipboard entry |
| `POST` | `/api/url` | Shorten URL |
| `GET` | `/api/urls` | List shortened URLs |
| `DELETE` | `/api/urls/{code}` | Delete URL |
| `GET` | `/api/files` | List uploaded files |
| `POST` | `/api/download/zip` | Download multiple files as ZIP |
| `POST` | `/api/share` | Web Share Target endpoint |
| `GET` | `/api/events` | SSE stream for live updates |
| `GET` | `/api/space` | Storage capacity (total/used/free) |
| `GET` | `/config.js` | Runtime JS config |

## Getting Started

### Prerequisites

- Docker & Docker Compose
- An SMB share (or compatible file server) for encrypted file storage

### Quick Start

1. **Clone and configure:**

   ```bash
   cp .env.example .env
   # Edit .env with your SMB server details
   ```

2. **Run:**

   ```bash
   make run
   ```

3. **Open** `http://localhost:8080` in your browser

### Configuration

All configuration via environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `SMB_HOST` | SMB server address | *(required)* |
| `SMB_SHARE` | SMB share name | *(required)* |
| `SMB_USERNAME` | SMB username | *(required)* |
| `SMB_PASSWORD` | SMB password | *(required)* |
| `SMB_ENCRYPT_KEY` | AES-256-GCM encryption key | *(required, 32 bytes)* |
| `AUTH_TOKEN` | Bearer token for external access | *(optional, empty = LAN-only)* |
| `LISTEN_ADDR` | Listen address | `:8080` |

## CLI Client

Build the CLI binary:

```bash
make cli
# Outputs to bin/share-home
```

### Usage

```bash
# Set server URL (or use SHARE_HOME_URL env var)
./bin/share-home config --url http://localhost:8080

# Upload a file
./bin/share-home upload photo.jpg --expires 1d

# Save text to clipboard
./bin/share-home paste "hello world"

# Pipe from stdin
cat notes.txt | ./bin/share-home paste

# Shorten a URL
./bin/share-home url https://example.com/very/long/path

# List all items
./bin/share-home list

# List by type
./bin/share-home list files
./bin/share-home list clips
./bin/share-home list urls

# Download a file
./bin/share-home get <id>

# Output clipboard content to terminal
./bin/share-home cat <id>

# Copy link to system clipboard
./bin/share-home copy <id>

# Open in browser
./bin/share-home open <id>

# Delete any item
./bin/share-home delete <id>

# JSON output for scripting
./bin/share-home list --json

# Check server connectivity
./bin/share-home server
```

### CLI Config

Config is stored in `~/.config/share-home/config.json` or via environment variables:

```bash
export SHARE_HOME_URL=http://localhost:8080
export SHARE_HOME_TOKEN=your-token  # if auth is enabled
```

## Development

```bash
# Run in foreground (see logs)
make run-fg

# Build only
make build

# View logs
make logs

# Clean up (removes volumes)
make clean

# Build CLI
make cli
```

### Running Tests

```bash
cd go && go test ./...
```

> Without local Go: `docker run --rm -v .:/work -w /work/go golang:1.24-alpine sh -c 'apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go test ./internal/handlers/... -v -count=1'`

## Security

Share Home is designed for **temporary, non-sensitive file sharing within a trusted home network**. Security is intentionally minimal — don't store passwords, personal data, or sensitive documents. Think of it as a digital handoff, not a vault.

### What's Protected
- All file storage uses **AES-256-GCM** encryption on disk
- **Path traversal** protection on all static file serving
- **CORS** restricted to requesting origin with `Vary: Origin`
- **Security headers**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, strict referrer policy
- **LAN auth bypass** — requests from private IP ranges skip token verification
- **External auth** — non-LAN requests require valid `Authorization: Bearer` token
- `.env` is gitignored; credentials never committed to repository

### What's Not
- No user accounts or access control beyond the single bearer token
- No HTTPS — designed for LAN use only; put a reverse proxy (Caddy, Nginx) in front if exposing externally
- No rate limiting or brute-force protection
- No encryption in transit — files travel unencrypted over HTTP within your LAN

## License

MIT
