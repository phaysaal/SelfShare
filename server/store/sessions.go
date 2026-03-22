package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/faisal/selfshare/auth"
	"github.com/google/uuid"
)

// Session represents an active refresh token session.
type Session struct {
	ID          string
	UserID      string
	RefreshHash string
	DeviceName  string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CreateSession stores a new refresh token session.
func (db *DB) CreateSession(userID, refreshToken, deviceName string) (*Session, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	expires := now.Add(auth.RefreshTokenDuration)
	hash := auth.HashRefreshToken(refreshToken)

	_, err := db.Exec(`
		INSERT INTO sessions (id, user_id, refresh_hash, device_name, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, hash, deviceName, now.Format(timeFormat), expires.Format(timeFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{
		ID:          id,
		UserID:      userID,
		RefreshHash: hash,
		DeviceName:  deviceName,
		CreatedAt:   now,
		ExpiresAt:   expires,
	}, nil
}

// ValidateRefreshToken finds a session matching the given refresh token.
// Returns the session if valid, nil if not found or expired.
func (db *DB) ValidateRefreshToken(refreshToken string) (*Session, error) {
	hash := auth.HashRefreshToken(refreshToken)

	s := &Session{}
	var createdAt, expiresAt string

	err := db.QueryRow(`
		SELECT id, user_id, refresh_hash, device_name, created_at, expires_at
		FROM sessions WHERE refresh_hash = ?`, hash,
	).Scan(&s.ID, &s.UserID, &s.RefreshHash, &s.DeviceName, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("validate refresh token: %w", err)
	}

	s.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	s.ExpiresAt, _ = time.Parse(timeFormat, expiresAt)

	if time.Now().UTC().After(s.ExpiresAt) {
		// Expired — clean it up
		db.Exec("DELETE FROM sessions WHERE id = ?", s.ID)
		return nil, nil
	}

	return s, nil
}

// DeleteSession removes a session by ID.
func (db *DB) DeleteSession(id string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

// DeleteSessionByRefreshToken removes a session matching the given refresh token.
func (db *DB) DeleteSessionByRefreshToken(refreshToken string) error {
	hash := auth.HashRefreshToken(refreshToken)
	_, err := db.Exec("DELETE FROM sessions WHERE refresh_hash = ?", hash)
	return err
}

// DeleteUserSessions removes all sessions for a user (force logout everywhere).
func (db *DB) DeleteUserSessions(userID string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// CleanExpiredSessions removes all expired sessions.
func (db *DB) CleanExpiredSessions() (int64, error) {
	now := time.Now().UTC().Format(timeFormat)
	result, err := db.Exec("DELETE FROM sessions WHERE expires_at < ?", now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
