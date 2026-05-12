package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"share-home/internal/auth"
	"share-home/internal/config"
	"share-home/internal/db"
	"share-home/internal/handlers"
	"share-home/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewSMBStore(
		cfg.SMBHost, cfg.SMBShare, cfg.SMBUsername, cfg.SMBPassword,
		cfg.SMBBaseDir, cfg.SMBEncryptKey,
	)
	if err != nil {
		log.Fatalf("SMB connect: %v", err)
	}
	defer store.Close()
	log.Println("Connected to SMB share")

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	// Periodic cleanup of expired files
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			keys, err := d.PurgeExpired()
			if err != nil {
				log.Printf("purge expired: %v", err)
				continue
			}
			for _, key := range keys {
				store.Delete(key)
			}
		}
	}()

	api := http.NewServeMux()
	api.Handle("POST /api/upload", handlers.NewUploadHandler(store, d))
	api.Handle("GET /api/download/{id}", handlers.NewDownloadHandler(store, d))
	api.Handle("DELETE /api/files/{id}", handlers.NewDeleteFileHandler(store, d))
	api.Handle("POST /api/clipboard", handlers.NewClipboardHandler(store, d))
	api.Handle("GET /api/clipboard/{id}", handlers.NewClipboardReadHandler(store, d))
	api.Handle("GET /api/clipboard", handlers.NewListClipboardHandler(d))
	api.Handle("DELETE /api/clipboard/{id}", handlers.NewDeleteClipboardHandler(store, d))
	api.Handle("POST /api/url", handlers.NewURLHandler(d))
	api.Handle("GET /api/urls", handlers.NewListURLsHandler(d))
	api.Handle("DELETE /api/urls/{code}", handlers.NewDeleteURLHandler(d))
	api.Handle("GET /api/files", handlers.NewListFilesHandler(d))
	api.Handle("GET /api/space", handlers.NewSpaceHandler(store))
	api.Handle("POST /api/download/zip", handlers.NewZipDownloadHandler(store, d))
	api.Handle("POST /api/upload_url", handlers.NewUploadURLHandler(store, d))
	api.Handle("POST /api/share", handlers.NewShareHandler(store, d))

	// SSE broker for live updates
	broker := handlers.NewSSEBroker()
	handlers.DefaultBroker = broker
	api.Handle("GET /api/events", broker)

	redirect := handlers.NewRedirectHandler(d)
	static := handlers.NewStaticHandler("/app/web")
	configHandler := &handlers.ConfigHandler{AuthRequired: cfg.AuthToken != ""}

	mux := http.NewServeMux()

	// Health check (no auth needed)
	mux.Handle("GET /healthz", handlers.NewHealthHandler())

	// Dynamic JS config for frontend
	mux.Handle("/config.js", configHandler)

	mux.Handle("/api/", auth.Middleware(cfg.AuthToken)(api))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Inject auth token status into index.html for frontend
			if cfg.AuthToken != "" {
				w.Header().Set("X-Auth-Required", "true")
			}
			static.ServeHTTP(w, r)
			return
		}
		if !strings.Contains(r.URL.Path, ".") && !strings.HasPrefix(r.URL.Path, "/api/") {
			redirect.ServeHTTP(w, r)
			return
		}
		static.ServeHTTP(w, r)
	}))

		srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: corsMiddleware(cfg.AllowedOrigins)(securityHeaders(loggingMiddleware(mux))),
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	origins := make(map[string]bool)
	for _, o := range allowedOrigins {
		origins[o] = true
	}

	return func(next http.Handler) http.Handler {
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
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
