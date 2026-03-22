package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/faisal/selfshare/auth"
	"github.com/google/uuid"
)

// Share represents a share link.
type Share struct {
	ID            string
	FileID        string
	Token         string
	PasswordHash  *string
	ExpiresAt     *time.Time
	MaxDownloads  *int
	DownloadCount int
	CreatedAt     time.Time
	RevokedAt     *time.Time
}

// ShareWithFile combines share info with the associated file.
type ShareWithFile struct {
	Share
	FileName  string
	FileIsDir bool
	FileSize  int64
	MimeType  *string
}

// GenerateShareToken creates a URL-safe random token (128-bit).
func GenerateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateShare creates a new share link for a file or folder.
func (db *DB) CreateShare(fileID string, password string, expiresIn *int, maxDownloads *int) (*Share, error) {
	token, err := GenerateShareToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	var passwordHash *string
	if password != "" {
		h, err := auth.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = &h
	}

	var expiresAt *time.Time
	if expiresIn != nil && *expiresIn > 0 {
		t := now.Add(time.Duration(*expiresIn) * time.Second)
		expiresAt = &t
	}

	_, err = db.Exec(`
		INSERT INTO shares (id, file_id, token, password_hash, expires_at, max_downloads, download_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		id, fileID, token, passwordHash, formatTimePtr(expiresAt), maxDownloads, now.Format(timeFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("insert share: %w", err)
	}

	return db.GetShare(id)
}

// GetShare retrieves a share by ID.
func (db *DB) GetShare(id string) (*Share, error) {
	return db.scanShare(db.QueryRow(`
		SELECT id, file_id, token, password_hash, expires_at, max_downloads, download_count, created_at, revoked_at
		FROM shares WHERE id = ?`, id))
}

// GetShareByToken retrieves a share by its public token.
func (db *DB) GetShareByToken(token string) (*Share, error) {
	return db.scanShare(db.QueryRow(`
		SELECT id, file_id, token, password_hash, expires_at, max_downloads, download_count, created_at, revoked_at
		FROM shares WHERE token = ?`, token))
}

// GetShareWithFile retrieves a share with its associated file info.
func (db *DB) GetShareWithFile(token string) (*ShareWithFile, error) {
	s := &ShareWithFile{}
	var expiresAt, revokedAt sql.NullString
	var createdAt string
	var maxDl sql.NullInt64
	var isDir int

	err := db.QueryRow(`
		SELECT s.id, s.file_id, s.token, s.password_hash, s.expires_at, s.max_downloads,
		       s.download_count, s.created_at, s.revoked_at,
		       f.name, f.is_dir, f.size_bytes, f.mime_type
		FROM shares s JOIN files f ON s.file_id = f.id
		WHERE s.token = ?`, token,
	).Scan(&s.ID, &s.FileID, &s.Token, &s.PasswordHash, &expiresAt, &maxDl,
		&s.DownloadCount, &createdAt, &revokedAt,
		&s.FileName, &isDir, &s.FileSize, &s.MimeType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share with file: %w", err)
	}

	s.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	s.FileIsDir = isDir == 1
	if expiresAt.Valid {
		t, _ := time.Parse(timeFormat, expiresAt.String)
		s.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t, _ := time.Parse(timeFormat, revokedAt.String)
		s.RevokedAt = &t
	}
	if maxDl.Valid {
		v := int(maxDl.Int64)
		s.MaxDownloads = &v
	}

	return s, nil
}

// ListShares returns all active (non-revoked) shares.
func (db *DB) ListShares() ([]ShareWithFile, error) {
	rows, err := db.Query(`
		SELECT s.id, s.file_id, s.token, s.password_hash, s.expires_at, s.max_downloads,
		       s.download_count, s.created_at, s.revoked_at,
		       f.name, f.is_dir, f.size_bytes, f.mime_type
		FROM shares s JOIN files f ON s.file_id = f.id
		WHERE s.revoked_at IS NULL
		ORDER BY s.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	var shares []ShareWithFile
	for rows.Next() {
		s := ShareWithFile{}
		var expiresAt, revokedAt sql.NullString
		var createdAt string
		var maxDl sql.NullInt64
		var isDir int

		err := rows.Scan(&s.ID, &s.FileID, &s.Token, &s.PasswordHash, &expiresAt, &maxDl,
			&s.DownloadCount, &createdAt, &revokedAt,
			&s.FileName, &isDir, &s.FileSize, &s.MimeType)
		if err != nil {
			return nil, fmt.Errorf("scan share: %w", err)
		}

		s.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		s.FileIsDir = isDir == 1
		if expiresAt.Valid {
			t, _ := time.Parse(timeFormat, expiresAt.String)
			s.ExpiresAt = &t
		}
		if maxDl.Valid {
			v := int(maxDl.Int64)
			s.MaxDownloads = &v
		}

		shares = append(shares, s)
	}
	return shares, rows.Err()
}

// RevokeShare marks a share as revoked.
func (db *DB) RevokeShare(id string) error {
	now := time.Now().UTC().Format(timeFormat)
	_, err := db.Exec("UPDATE shares SET revoked_at = ? WHERE id = ?", now, id)
	return err
}

// IncrementDownloadCount increments the download counter for a share.
func (db *DB) IncrementDownloadCount(id string) error {
	_, err := db.Exec("UPDATE shares SET download_count = download_count + 1 WHERE id = ?", id)
	return err
}

// ValidateShareAccess checks if a share is valid for access.
// Returns an error string if invalid, empty string if valid.
func (s *Share) ValidateAccess() string {
	if s.RevokedAt != nil {
		return "This link has been revoked"
	}
	if s.ExpiresAt != nil && time.Now().UTC().After(*s.ExpiresAt) {
		return "This link has expired"
	}
	if s.MaxDownloads != nil && s.DownloadCount >= *s.MaxDownloads {
		return "Download limit reached"
	}
	return ""
}

// HasPassword returns true if the share is password-protected.
func (s *Share) HasPassword() bool {
	return s.PasswordHash != nil && *s.PasswordHash != ""
}

func (db *DB) scanShare(row *sql.Row) (*Share, error) {
	s := &Share{}
	var expiresAt, revokedAt sql.NullString
	var createdAt string
	var maxDl sql.NullInt64

	err := row.Scan(&s.ID, &s.FileID, &s.Token, &s.PasswordHash, &expiresAt, &maxDl,
		&s.DownloadCount, &createdAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan share: %w", err)
	}

	s.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	if expiresAt.Valid {
		t, _ := time.Parse(timeFormat, expiresAt.String)
		s.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t, _ := time.Parse(timeFormat, revokedAt.String)
		s.RevokedAt = &t
	}
	if maxDl.Valid {
		v := int(maxDl.Int64)
		s.MaxDownloads = &v
	}

	return s, nil
}
