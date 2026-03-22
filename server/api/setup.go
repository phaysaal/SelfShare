package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/faisal/selfshare/auth"
	"github.com/faisal/selfshare/config"
	"github.com/faisal/selfshare/store"
	"github.com/google/uuid"
)

// SetupHandler manages first-run server configuration.
type SetupHandler struct {
	DB         *store.DB
	Cfg        *config.Config
	ConfigPath string
	OnComplete func() // called after setup to reload config in the server
}

// HandleSetupPage serves the setup HTML page.
func (h *SetupHandler) HandleSetupPage(w http.ResponseWriter, r *http.Request) {
	if h.Cfg.IsSetup() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(setupHTML))
}

// HandleSetupAPI handles POST /api/v1/setup — create admin and configure server.
func (h *SetupHandler) HandleSetupAPI(w http.ResponseWriter, r *http.Request) {
	if h.Cfg.IsSetup() {
		writeError(w, http.StatusConflict, "server is already set up")
		return
	}

	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Generate JWT secret
	secret, err := auth.GenerateSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate secret")
		log.Printf("GenerateSecret error: %v", err)
		return
	}

	// Generate server ID
	serverID := uuid.New().String()

	// Create admin user
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}
	user, err := h.DB.CreateUser(req.Username, req.Password, displayName, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		log.Printf("CreateUser error: %v", err)
		return
	}

	// Save config
	h.Cfg.JWTSecret = secret
	h.Cfg.ServerID = serverID
	if err := h.Cfg.Save(h.ConfigPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		log.Printf("SaveConfig error: %v", err)
		return
	}

	log.Printf("Setup complete. Admin user '%s' created. Server ID: %s", user.Username, serverID)

	if h.OnComplete != nil {
		h.OnComplete()
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":    "setup complete",
		"server_id": serverID,
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"is_admin":     user.IsAdmin,
		},
	})
}

const setupHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SelfShare — Setup</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #0a0a0a; color: #e0e0e0;
            display: flex; justify-content: center; align-items: center;
            min-height: 100vh; padding: 20px;
        }
        .setup-card {
            background: #111; border: 1px solid #222; border-radius: 12px;
            padding: 40px; max-width: 420px; width: 100%;
        }
        h1 { margin-bottom: 8px; color: #fff; font-size: 24px; }
        .subtitle { color: #888; margin-bottom: 32px; font-size: 14px; }
        .field { margin-bottom: 20px; }
        label { display: block; margin-bottom: 6px; font-size: 14px; color: #aaa; }
        input {
            width: 100%; padding: 10px 14px; border: 1px solid #333; border-radius: 8px;
            background: #1a1a1a; color: #e0e0e0; font-size: 15px; outline: none;
        }
        input:focus { border-color: #4a8aff; }
        button {
            width: 100%; padding: 12px; border: none; border-radius: 8px;
            background: #4a8aff; color: #fff; font-size: 15px; font-weight: 600;
            cursor: pointer; margin-top: 8px;
        }
        button:hover { background: #3a7aef; }
        button:disabled { opacity: 0.5; cursor: not-allowed; }
        #error { color: #ff6b6b; font-size: 14px; margin-top: 12px; display: none; }
        #success { color: #6bff6b; font-size: 14px; margin-top: 12px; display: none; }
    </style>
</head>
<body>
    <div class="setup-card">
        <h1>Welcome to SelfShare</h1>
        <p class="subtitle">Create your admin account to get started.</p>

        <form id="setupForm" onsubmit="doSetup(event)">
            <div class="field">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required autocomplete="username" autofocus>
            </div>
            <div class="field">
                <label for="displayName">Display Name (optional)</label>
                <input type="text" id="displayName" name="display_name" autocomplete="name">
            </div>
            <div class="field">
                <label for="password">Password (8+ characters)</label>
                <input type="password" id="password" name="password" required minlength="8" autocomplete="new-password">
            </div>
            <div class="field">
                <label for="confirmPassword">Confirm Password</label>
                <input type="password" id="confirmPassword" required minlength="8" autocomplete="new-password">
            </div>
            <button type="submit" id="submitBtn">Create Account & Start</button>
            <div id="error"></div>
            <div id="success"></div>
        </form>
    </div>

    <script>
    async function doSetup(e) {
        e.preventDefault();
        const errEl = document.getElementById('error');
        const successEl = document.getElementById('success');
        errEl.style.display = 'none';
        successEl.style.display = 'none';

        const password = document.getElementById('password').value;
        const confirm = document.getElementById('confirmPassword').value;
        if (password !== confirm) {
            errEl.textContent = 'Passwords do not match.';
            errEl.style.display = 'block';
            return;
        }

        const btn = document.getElementById('submitBtn');
        btn.disabled = true;
        btn.textContent = 'Setting up...';

        try {
            const resp = await fetch('/api/v1/setup', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    username: document.getElementById('username').value,
                    password: password,
                    display_name: document.getElementById('displayName').value
                })
            });
            const data = await resp.json();
            if (!resp.ok) {
                throw new Error(data.error || 'Setup failed');
            }
            successEl.textContent = 'Setup complete! Redirecting...';
            successEl.style.display = 'block';
            setTimeout(() => { window.location.href = '/'; }, 1500);
        } catch (e) {
            errEl.textContent = e.message;
            errEl.style.display = 'block';
            btn.disabled = false;
            btn.textContent = 'Create Account & Start';
        }
    }
    </script>
</body>
</html>`
