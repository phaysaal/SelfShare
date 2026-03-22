package main

import (
	"crypto/tls"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/faisal/selfshare/api"
	"github.com/faisal/selfshare/config"
	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
	"golang.org/x/crypto/acme/autocert"
)

//go:embed all:web/dist
var webDistFS embed.FS

func main() {
	configPath := flag.String("config", "", "path to config file")
	storagePath := flag.String("storage", "", "override storage directory path")
	listenAddr := flag.String("listen", "", "override listen address (e.g. :8080)")
	enableTLS := flag.Bool("tls", false, "enable HTTPS with Let's Encrypt")
	tlsDomain := flag.String("domain", "", "domain name for Let's Encrypt")
	flag.Parse()

	// Determine config file path
	cfgPath := *configPath
	if cfgPath == "" {
		home, _ := os.UserHomeDir()
		cfgPath = filepath.Join(home, ".selfshare", "config.json")
	}

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Apply CLI overrides
	if *storagePath != "" {
		cfg.StoragePath = *storagePath
	}
	if *listenAddr != "" {
		cfg.ListenAddr = *listenAddr
	}
	if *enableTLS {
		cfg.TLSEnabled = true
	}
	if *tlsDomain != "" {
		cfg.TLSDomain = *tlsDomain
	}

	// Ensure storage directories exist
	for _, dir := range []string{cfg.DataDir(), cfg.ThumbDir(), cfg.TempUploadDir()} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Open database
	db, err := store.Open(cfg.DBPath())
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize file storage (mirrors app directory structure on disk)
	files, err := storage.NewFileStore(cfg.DataDir())
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}

	// Set up embedded SPA
	if sub, err := fs.Sub(webDistFS, "web/dist"); err == nil {
		api.DistFS = sub
	}

	// Create router
	router, _ := api.NewRouter(&api.RouterDeps{
		DB:         db,
		Files:      files,
		Cfg:        cfg,
		ConfigPath: cfgPath,
	})

	log.Printf("SelfShare server starting on %s", cfg.ListenAddr)
	log.Printf("Storage: %s", cfg.StoragePath)

	if !cfg.IsSetup() {
		log.Printf("First run detected — visit the server to set up your admin account")
	}

	if cfg.TLSEnabled && cfg.TLSDomain != "" {
		startTLS(cfg, router)
	} else {
		log.Printf("Open http://localhost%s in your browser", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, router); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}

func startTLS(cfg *config.Config, handler http.Handler) {
	certDir := filepath.Join(cfg.StoragePath, "certs")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		log.Fatalf("Failed to create cert directory: %v", err)
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cfg.TLSDomain),
	}

	// HTTP server on port 80 for ACME challenges + redirect to HTTPS
	go func() {
		log.Printf("HTTP redirect server on :80")
		if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil {
			log.Printf("HTTP redirect server error: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:    ":443",
		Handler: handler,
		TLSConfig: &tls.Config{
			GetCertificate: m.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		},
	}

	log.Printf("HTTPS server starting on :443 for %s", cfg.TLSDomain)
	if err := srv.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("HTTPS server failed: %v", err)
	}
}
