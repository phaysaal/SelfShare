package api

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/faisal/selfshare/auth"
	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
)

// PublicShareHandler serves public share pages (no auth required).
type PublicShareHandler struct {
	DB    *store.DB
	Files *storage.FileStore
}

// ViewShare handles GET /s/{token} — render the share page.
func (h *PublicShareHandler) ViewShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	sf, err := h.DB.GetShareWithFile(token)
	if err != nil {
		log.Printf("ViewShare error: %v", err)
		renderShareError(w, "Something went wrong")
		return
	}
	if sf == nil {
		renderShareError(w, "Share link not found")
		return
	}

	if msg := sf.ValidateAccess(); msg != "" {
		renderShareError(w, msg)
		return
	}

	// Password check via cookie
	if sf.HasPassword() {
		cookie, err := r.Cookie("share_" + sf.Token)
		if err != nil || cookie.Value != "ok" {
			renderPasswordPage(w, sf.Token)
			return
		}
	}

	renderSharePage(w, sf)
}

// DownloadShare handles GET /s/{token}/download — download the shared file.
func (h *PublicShareHandler) DownloadShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	sf, err := h.DB.GetShareWithFile(token)
	if err != nil || sf == nil {
		http.Error(w, "Not found", 404)
		return
	}

	if msg := sf.ValidateAccess(); msg != "" {
		http.Error(w, msg, 410)
		return
	}

	if sf.HasPassword() {
		cookie, err := r.Cookie("share_" + sf.Token)
		if err != nil || cookie.Value != "ok" {
			http.Error(w, "Authentication required", 401)
			return
		}
	}

	if sf.FileIsDir {
		http.Error(w, "Folder download not yet supported", 400)
		return
	}

	file, err := h.Files.Open(sf.Share.FileID)
	if err != nil {
		// FileID is the file record ID, we need the disk path
		f, err2 := h.DB.GetFile(sf.FileID)
		if err2 != nil || f == nil {
			http.Error(w, "File not found", 404)
			return
		}
		file, err = h.Files.Open(f.DiskPath)
		if err != nil {
			http.Error(w, "File not found", 404)
			return
		}
	}
	defer file.Close()

	h.DB.IncrementDownloadCount(sf.ID)

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sf.FileName))
	if sf.MimeType != nil {
		w.Header().Set("Content-Type", *sf.MimeType)
	}

	stat, _ := file.Stat()
	http.ServeContent(w, r, sf.FileName, stat.ModTime(), file)
}

// ViewShareInline handles GET /s/{token}/view — view the shared file inline (for images/video).
func (h *PublicShareHandler) ViewShareInline(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	sf, err := h.DB.GetShareWithFile(token)
	if err != nil || sf == nil {
		http.Error(w, "Not found", 404)
		return
	}

	if msg := sf.ValidateAccess(); msg != "" {
		http.Error(w, msg, 410)
		return
	}

	if sf.HasPassword() {
		cookie, err := r.Cookie("share_" + sf.Token)
		if err != nil || cookie.Value != "ok" {
			http.Error(w, "Authentication required", 401)
			return
		}
	}

	f, err := h.DB.GetFile(sf.FileID)
	if err != nil || f == nil {
		http.Error(w, "File not found", 404)
		return
	}

	file, err := h.Files.Open(f.DiskPath)
	if err != nil {
		http.Error(w, "File not found", 404)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sf.FileName))
	if sf.MimeType != nil {
		w.Header().Set("Content-Type", *sf.MimeType)
	}

	stat, _ := file.Stat()
	http.ServeContent(w, r, sf.FileName, stat.ModTime(), file)
}

// AuthShare handles POST /s/{token}/auth — verify share password.
func (h *PublicShareHandler) AuthShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	sf, err := h.DB.GetShareWithFile(token)
	if err != nil || sf == nil {
		http.Error(w, "Not found", 404)
		return
	}

	r.ParseForm()
	password := r.FormValue("password")

	if !sf.HasPassword() || auth.CheckPassword(*sf.PasswordHash, password) == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "share_" + sf.Token,
			Value:    "ok",
			Path:     "/s/" + sf.Token,
			MaxAge:   3600,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/s/"+sf.Token, http.StatusSeeOther)
		return
	}

	renderPasswordPage(w, sf.Token)
}

// --- Templates ---

func renderShareError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(410)
	shareTmpl.Execute(w, map[string]any{"Error": msg})
}

func renderPasswordPage(w http.ResponseWriter, token string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	shareTmpl.Execute(w, map[string]any{"NeedPassword": true, "Token": token})
}

func renderSharePage(w http.ResponseWriter, sf *store.ShareWithFile) {
	mime := ""
	if sf.MimeType != nil {
		mime = *sf.MimeType
	}

	data := map[string]any{
		"FileName":  sf.FileName,
		"FileSize":  formatBytes(sf.FileSize),
		"Token":     sf.Token,
		"IsImage":   strings.HasPrefix(mime, "image/"),
		"IsVideo":   strings.HasPrefix(mime, "video/"),
		"IsAudio":   strings.HasPrefix(mime, "audio/"),
		"MimeType":  mime,
		"IsDir":     sf.FileIsDir,
		"ViewURL":   "/s/" + sf.Token + "/view",
		"DownloadURL": "/s/" + sf.Token + "/download",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	shareTmpl.Execute(w, data)
}

func formatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	fb := float64(b)
	for fb >= k && i < len(sizes)-1 {
		fb /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", fb, sizes[i])
}

// fileExists checks the real path on disk
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var shareTmpl = template.Must(template.New("share").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SelfShare</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0a0a0a;color:#e0e0e0;display:flex;justify-content:center;align-items:center;min-height:100vh;padding:20px}
.card{background:#111;border:1px solid #222;border-radius:12px;padding:40px;max-width:520px;width:100%;text-align:center}
h1{font-size:20px;margin-bottom:8px;color:#fff}
.meta{color:#888;font-size:14px;margin-bottom:24px}
.error-msg{color:#ff6b6b;font-size:16px;margin:20px 0}
.preview{margin:20px 0;border-radius:8px;overflow:hidden}
.preview img{max-width:100%;max-height:400px;border-radius:8px}
.preview video{max-width:100%;max-height:400px;border-radius:8px}
.preview audio{width:100%}
.btn{display:inline-block;padding:12px 28px;border:none;border-radius:8px;background:#4a8aff;color:#fff;font-size:15px;font-weight:600;cursor:pointer;text-decoration:none;margin:4px}
.btn:hover{background:#3a7aef}
.btn-outline{background:transparent;border:1px solid #333;color:#e0e0e0}
.btn-outline:hover{background:#1a1a1a}
input[type=password]{width:100%;padding:10px 14px;border:1px solid #333;border-radius:8px;background:#1a1a1a;color:#e0e0e0;font-size:15px;outline:none;margin-bottom:12px}
input:focus{border-color:#4a8aff}
.branding{color:#555;font-size:12px;margin-top:24px}
</style>
</head>
<body>
<div class="card">
{{if .Error}}
  <h1>Unavailable</h1>
  <div class="error-msg">{{.Error}}</div>
  <p class="branding">Shared via SelfShare</p>
{{else if .NeedPassword}}
  <h1>Password Required</h1>
  <p class="meta">This file is protected. Enter the password to continue.</p>
  <form method="POST" action="/s/{{.Token}}/auth">
    <input type="password" name="password" placeholder="Enter password" required autofocus>
    <button class="btn" type="submit">Unlock</button>
  </form>
  <p class="branding">Shared via SelfShare</p>
{{else}}
  <h1>{{.FileName}}</h1>
  <p class="meta">{{.FileSize}}</p>

  {{if .IsImage}}
  <div class="preview"><img src="{{.ViewURL}}" alt="{{.FileName}}"></div>
  {{end}}

  {{if .IsVideo}}
  <div class="preview"><video controls playsinline><source src="{{.ViewURL}}" type="{{.MimeType}}"></video></div>
  {{end}}

  {{if .IsAudio}}
  <div class="preview"><audio controls><source src="{{.ViewURL}}" type="{{.MimeType}}"></audio></div>
  {{end}}

  <div>
    <a class="btn" href="{{.DownloadURL}}">Download</a>
  </div>
  <p class="branding">Shared via SelfShare</p>
{{end}}
</div>
</body>
</html>`))
