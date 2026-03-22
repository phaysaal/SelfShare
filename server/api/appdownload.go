package api

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
)

// AppDownloadHandler serves the mobile app download page.
type AppDownloadHandler struct {
	StoragePath string
}

func (h *AppDownloadHandler) apkPath() string {
	return filepath.Join(h.StoragePath, "selfshare.apk")
}

// ServePage renders the app download page.
func (h *AppDownloadHandler) ServePage(w http.ResponseWriter, r *http.Request) {
	hasAPK := false
	var apkSize string
	if info, err := os.Stat(h.apkPath()); err == nil {
		hasAPK = true
		apkSize = formatBytes(info.Size())
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	appDownloadTmpl.Execute(w, map[string]any{
		"HasAPK":  hasAPK,
		"APKSize": apkSize,
	})
}

// ServeAPK serves the APK file for download.
func (h *AppDownloadHandler) ServeAPK(w http.ResponseWriter, r *http.Request) {
	path := h.apkPath()
	if _, err := os.Stat(path); err != nil {
		http.Error(w, "APK not available", 404)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "Failed to open APK", 500)
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="selfshare.apk"`))
	http.ServeContent(w, r, "selfshare.apk", stat.ModTime(), f)
}

var appDownloadTmpl = template.Must(template.New("app").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SelfShare — Get the App</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0a0a0a;color:#e0e0e0;display:flex;justify-content:center;align-items:center;min-height:100vh;padding:20px}
.card{background:#111;border:1px solid #222;border-radius:12px;padding:40px;max-width:420px;width:100%;text-align:center}
h1{font-size:24px;margin-bottom:8px;color:#fff}
.subtitle{color:#888;font-size:14px;margin-bottom:32px}
.icon{font-size:64px;margin-bottom:16px}
.btn{display:inline-block;padding:14px 32px;border:none;border-radius:8px;background:#4a8aff;color:#fff;font-size:16px;font-weight:600;cursor:pointer;text-decoration:none}
.btn:hover{background:#3a7aef}
.size{color:#888;font-size:13px;margin-top:12px}
.not-available{color:#888;font-size:15px;margin:20px 0}
.instructions{text-align:left;margin-top:24px;padding-top:20px;border-top:1px solid #222;color:#aaa;font-size:13px;line-height:1.6}
.instructions code{background:#1a1a1a;padding:2px 6px;border-radius:4px;color:#6ba3ff}
.back{margin-top:20px}
.back a{color:#6ba3ff;text-decoration:none;font-size:14px}
</style>
</head>
<body>
<div class="card">
  <div class="icon">&#128241;</div>
  <h1>SelfShare for Android</h1>
  <p class="subtitle">Access your files on the go</p>

  {{if .HasAPK}}
  <a class="btn" href="/app/download">Download APK</a>
  <p class="size">{{.APKSize}}</p>
  <div class="instructions">
    <strong>To install:</strong><br>
    1. Download the APK on your Android phone<br>
    2. Open it and allow "Install from unknown sources"<br>
    3. Open the app and enter this server's address
  </div>
  {{else}}
  <p class="not-available">APK not available yet.</p>
  <div class="instructions">
    <strong>To make it available:</strong><br>
    Place <code>selfshare.apk</code> in your storage directory:<br>
    <code>cp selfshare.apk ~/.selfshare/selfshare.apk</code>
  </div>
  {{end}}

  <div class="back"><a href="/">Back to SelfShare</a></div>
</div>
</body>
</html>`))
