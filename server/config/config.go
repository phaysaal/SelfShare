package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	// StoragePath is the root directory for all data (blobs, thumbs, db).
	StoragePath string `json:"storage_path"`

	// ListenAddr is the address the server listens on (e.g. ":8080").
	ListenAddr string `json:"listen_addr"`

	// ServerID is a unique identifier for this server instance.
	ServerID string `json:"server_id"`

	// JWTSecret is the HMAC-SHA256 signing key for JWT tokens.
	JWTSecret string `json:"jwt_secret"`

	// TLSEnabled enables automatic HTTPS via Let's Encrypt.
	TLSEnabled bool `json:"tls_enabled"`

	// TLSDomain is the domain name for Let's Encrypt certificates.
	TLSDomain string `json:"tls_domain"`
}

// IsSetup returns true if the server has been configured (has a JWT secret).
func (c *Config) IsSetup() bool {
	return c.JWTSecret != ""
}

// DefaultConfig returns a config suitable for local development.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		StoragePath: filepath.Join(home, ".selfshare"),
		ListenAddr:  ":8080",
	}
}

// Load reads config from a JSON file. Missing file returns default config.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes config to a JSON file.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// DataDir returns the path to the data directory under storage.
func (c *Config) DataDir() string {
	return filepath.Join(c.StoragePath, "data")
}

// ThumbDir returns the path to the thumbnail directory.
func (c *Config) ThumbDir() string {
	return filepath.Join(c.DataDir(), "thumbs")
}

// TempUploadDir returns the path for in-progress chunked uploads.
func (c *Config) TempUploadDir() string {
	return filepath.Join(c.StoragePath, "temp", "uploads")
}

// DBPath returns the path to the SQLite database file.
func (c *Config) DBPath() string {
	return filepath.Join(c.StoragePath, "selfshare.db")
}
