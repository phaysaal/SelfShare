package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultChunkSize   = 5 * 1024 * 1024 // 5MB
	UploadExpirePeriod = 24 * time.Hour
)

// Upload represents a chunked upload session.
type Upload struct {
	ID          string
	ParentID    string
	Filename    string
	TotalSize   int64
	ChunkSize   int
	TotalChunks int
	Received    []int // chunk numbers that have been received
	Status      string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CreateUpload starts a new chunked upload session.
func (db *DB) CreateUpload(parentID, filename string, totalSize int64, chunkSize int) (*Upload, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	expires := now.Add(UploadExpirePeriod)
	totalChunks := int(math.Ceil(float64(totalSize) / float64(chunkSize)))

	received, _ := json.Marshal([]int{})

	_, err := db.Exec(`
		INSERT INTO uploads (id, parent_id, filename, total_size, chunk_size, total_chunks, received, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		id, parentID, filename, totalSize, chunkSize, totalChunks, string(received),
		now.Format(timeFormat), expires.Format(timeFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("insert upload: %w", err)
	}

	return &Upload{
		ID:          id,
		ParentID:    parentID,
		Filename:    filename,
		TotalSize:   totalSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		Received:    []int{},
		Status:      "active",
		CreatedAt:   now,
		ExpiresAt:   expires,
	}, nil
}

// GetUpload retrieves an upload session by ID.
func (db *DB) GetUpload(id string) (*Upload, error) {
	u := &Upload{}
	var receivedJSON, createdAt, expiresAt string

	err := db.QueryRow(`
		SELECT id, parent_id, filename, total_size, chunk_size, total_chunks, received, status, created_at, expires_at
		FROM uploads WHERE id = ?`, id,
	).Scan(&u.ID, &u.ParentID, &u.Filename, &u.TotalSize, &u.ChunkSize, &u.TotalChunks,
		&receivedJSON, &u.Status, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get upload: %w", err)
	}

	json.Unmarshal([]byte(receivedJSON), &u.Received)
	u.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	u.ExpiresAt, _ = time.Parse(timeFormat, expiresAt)
	return u, nil
}

// MarkChunkReceived adds a chunk number to the received list.
func (db *DB) MarkChunkReceived(id string, chunkNum int) error {
	u, err := db.GetUpload(id)
	if err != nil {
		return err
	}
	if u == nil {
		return fmt.Errorf("upload not found")
	}

	// Check if already received
	for _, n := range u.Received {
		if n == chunkNum {
			return nil
		}
	}

	u.Received = append(u.Received, chunkNum)
	receivedJSON, _ := json.Marshal(u.Received)

	_, err = db.Exec("UPDATE uploads SET received = ? WHERE id = ?", string(receivedJSON), id)
	return err
}

// SetUploadStatus updates the status of an upload session.
func (db *DB) SetUploadStatus(id, status string) error {
	_, err := db.Exec("UPDATE uploads SET status = ? WHERE id = ?", status, id)
	return err
}

// DeleteUpload removes an upload session.
func (db *DB) DeleteUpload(id string) error {
	_, err := db.Exec("DELETE FROM uploads WHERE id = ?", id)
	return err
}

// CleanStaleUploads removes expired upload sessions and returns their IDs.
func (db *DB) CleanStaleUploads() ([]string, error) {
	now := time.Now().UTC().Format(timeFormat)

	rows, err := db.Query("SELECT id FROM uploads WHERE expires_at < ? AND status = 'active'", now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) > 0 {
		_, err = db.Exec("DELETE FROM uploads WHERE expires_at < ? AND status = 'active'", now)
	}
	return ids, err
}

// IsComplete returns true if all chunks have been received.
func (u *Upload) IsComplete() bool {
	return len(u.Received) >= u.TotalChunks
}

// MissingChunks returns chunk numbers that haven't been received yet.
func (u *Upload) MissingChunks() []int {
	received := make(map[int]bool)
	for _, n := range u.Received {
		received[n] = true
	}
	var missing []int
	for i := 0; i < u.TotalChunks; i++ {
		if !received[i] {
			missing = append(missing, i)
		}
	}
	return missing
}
