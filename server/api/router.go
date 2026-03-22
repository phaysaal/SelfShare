package api

import (
	"net/http"
	"time"

	"github.com/faisal/selfshare/config"
	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
	"github.com/faisal/selfshare/tasks"
)

// RouterDeps holds all dependencies needed to build the router.
type RouterDeps struct {
	DB         *store.DB
	Files      *storage.FileStore
	Cfg        *config.Config
	ConfigPath string
}

// NewRouter creates the HTTP router with all API routes.
// Returns the handler and the ThumbWorker (so callers can enqueue jobs).
func NewRouter(deps *RouterDeps) (http.Handler, *tasks.ThumbWorker) {
	mux := http.NewServeMux()

	// Start thumbnail worker (2 goroutines)
	thumbWorker := tasks.NewThumbWorker(deps.DB, deps.Cfg.ThumbDir(), 2)

	files := &FileHandler{DB: deps.DB, Files: deps.Files, ThumbWorker: thumbWorker}
	getSecret := func() string { return deps.Cfg.JWTSecret }
	authH := &AuthHandler{DB: deps.DB, GetSecret: getSecret}
	setup := &SetupHandler{
		DB:         deps.DB,
		Cfg:        deps.Cfg,
		ConfigPath: deps.ConfigPath,
	}
	photos := &PhotoHandler{DB: deps.DB, Files: deps.Files, ThumbDir: deps.Cfg.ThumbDir()}

	// Auth routes
	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authH.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", authH.Logout)

	// Setup routes
	mux.HandleFunc("GET /setup", setup.HandleSetupPage)
	mux.HandleFunc("POST /api/v1/setup", setup.HandleSetupAPI)

	// Chunked upload routes
	uploads := &UploadHandler{DB: deps.DB, Files: deps.Files, TempDir: deps.Cfg.TempUploadDir(), ThumbWorker: thumbWorker}
	mux.HandleFunc("POST /api/v1/uploads", uploads.Initiate)
	mux.HandleFunc("GET /api/v1/uploads/{id}", uploads.GetStatus)
	mux.HandleFunc("PUT /api/v1/uploads/{id}/{chunk}", uploads.UploadChunk)
	mux.HandleFunc("POST /api/v1/uploads/{id}/complete", uploads.Complete)
	mux.HandleFunc("DELETE /api/v1/uploads/{id}", uploads.Cancel)

	// Start stale upload cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			uploads.CleanupStaleUploads()
		}
	}()

	// File routes
	mux.HandleFunc("GET /api/v1/files", files.ListRoot)
	mux.HandleFunc("GET /api/v1/files/{id}", files.GetFile)
	mux.HandleFunc("GET /api/v1/files/{id}/children", files.ListChildren)
	mux.HandleFunc("POST /api/v1/files", files.Upload)
	mux.HandleFunc("PUT /api/v1/files/{id}", files.UpdateFile)
	mux.HandleFunc("DELETE /api/v1/files/{id}", files.DeleteFile)
	mux.HandleFunc("GET /api/v1/files/{id}/download", files.Download)
	mux.HandleFunc("GET /api/v1/files/{id}/view", files.View)
	mux.HandleFunc("GET /api/v1/files/{id}/thumb", photos.Thumb)

	// Photo routes
	mux.HandleFunc("GET /api/v1/photos", photos.ListPhotos)
	mux.HandleFunc("GET /api/v1/photos/timeline", photos.Timeline)

	// Share API routes (authenticated)
	shares := &ShareHandler{DB: deps.DB}
	mux.HandleFunc("POST /api/v1/shares", shares.CreateShare)
	mux.HandleFunc("GET /api/v1/shares", shares.ListShares)
	mux.HandleFunc("DELETE /api/v1/shares/{id}", shares.RevokeShare)

	// Public share pages (no auth)
	pub := &PublicShareHandler{DB: deps.DB, Files: deps.Files}
	mux.HandleFunc("GET /s/{token}", pub.ViewShare)
	mux.HandleFunc("GET /s/{token}/download", pub.DownloadShare)
	mux.HandleFunc("GET /s/{token}/view", pub.ViewShareInline)
	mux.HandleFunc("POST /s/{token}/auth", pub.AuthShare)

	// Health/discovery
	mux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"service":   "selfshare",
			"server_id": deps.Cfg.ServerID,
			"setup":     boolStr(deps.Cfg.IsSetup()),
		})
	})

	// Web UI — serve embedded SPA
	spa := newSPAHandler()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if !deps.Cfg.IsSetup() && (r.URL.Path == "/" || r.URL.Path == "") {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		spa.ServeHTTP(w, r)
	})

	// Apply middleware
	var handler http.Handler = mux
	handler = AuthMiddleware(func() string { return deps.Cfg.JWTSecret }, handler)
	handler = LoggingMiddleware(handler)

	return handler, thumbWorker
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
