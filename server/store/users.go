package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/faisal/selfshare/auth"
	"github.com/google/uuid"
)

// User represents a user account.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	DisplayName  string
	IsAdmin      bool
	CreatedAt    time.Time
}

// CreateUser creates a new user with a hashed password.
func (db *DB) CreateUser(username, password, displayName string, isAdmin bool) (*User, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(timeFormat)
	admin := 0
	if isAdmin {
		admin = 1
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, password_hash, display_name, is_admin, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, username, hash, displayName, admin, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return db.GetUser(id)
}

// GetUser retrieves a user by ID.
func (db *DB) GetUser(id string) (*User, error) {
	u := &User{}
	var isAdmin int
	var createdAt string

	err := db.QueryRow(`
		SELECT id, username, password_hash, display_name, is_admin, created_at
		FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &isAdmin, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	u.IsAdmin = isAdmin == 1
	u.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	return u, nil
}

// GetUserByUsername retrieves a user by username.
func (db *DB) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	var isAdmin int
	var createdAt string

	err := db.QueryRow(`
		SELECT id, username, password_hash, display_name, is_admin, created_at
		FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &isAdmin, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}

	u.IsAdmin = isAdmin == 1
	u.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	return u, nil
}

// Authenticate verifies username/password and returns the user if valid.
func (db *DB) Authenticate(username, password string) (*User, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	if err := auth.CheckPassword(user.PasswordHash, password); err != nil {
		return nil, nil
	}

	return user, nil
}

// UserCount returns the number of users in the database.
func (db *DB) UserCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}
