package store

import (
	"database/sql"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
)

// File represents a file or folder in the database.
type File struct {
	ID        string
	ParentID  *string
	Name      string
	IsDir     bool
	SizeBytes int64
	MimeType  *string
	SHA256    *string
	DiskPath  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

const timeFormat = "2006-01-02T15:04:05Z"

// GetDiskPath computes the relative disk path for a child under a given parent.
// Root has disk_path "", so a file "photo.jpg" under root → "photo.jpg".
// A file "beach.jpg" under folder "Photos" → "Photos/beach.jpg".
func (db *DB) GetDiskPath(parentID, name string) (string, error) {
	parent, err := db.GetFile(parentID)
	if err != nil {
		return "", err
	}
	if parent == nil {
		return "", fmt.Errorf("parent not found")
	}
	if parent.DiskPath == "" {
		return name, nil
	}
	return path.Join(parent.DiskPath, name), nil
}

// CreateFile inserts a new file record. diskPath should be the real relative path.
func (db *DB) CreateFile(parentID, name, mimeType, sha256Hash, diskPath string, size int64) (*File, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(timeFormat)

	_, err := db.Exec(`
		INSERT INTO files (id, parent_id, name, is_dir, size_bytes, mime_type, sha256, disk_path, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?, ?, ?, ?, ?)`,
		id, parentID, name, size, mimeType, sha256Hash, diskPath, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert file: %w", err)
	}

	return db.GetFile(id)
}

// CreateFolder inserts a new folder record with the real disk path.
func (db *DB) CreateFolder(parentID, name, diskPath string) (*File, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(timeFormat)

	_, err := db.Exec(`
		INSERT INTO files (id, parent_id, name, is_dir, disk_path, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)`,
		id, parentID, name, diskPath, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert folder: %w", err)
	}

	return db.GetFile(id)
}

// GetFile retrieves a file by ID.
func (db *DB) GetFile(id string) (*File, error) {
	f := &File{}
	var isDir int
	var createdAt, updatedAt string
	var deletedAt sql.NullString

	err := db.QueryRow(`
		SELECT id, parent_id, name, is_dir, size_bytes, mime_type, sha256, disk_path, created_at, updated_at, deleted_at
		FROM files WHERE id = ?`, id,
	).Scan(&f.ID, &f.ParentID, &f.Name, &isDir, &f.SizeBytes, &f.MimeType, &f.SHA256, &f.DiskPath, &createdAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	f.IsDir = isDir == 1
	f.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	f.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
	if deletedAt.Valid {
		t, _ := time.Parse(timeFormat, deletedAt.String)
		f.DeletedAt = &t
	}

	return f, nil
}

// ListChildren returns all non-deleted children of a folder.
func (db *DB) ListChildren(parentID string) ([]*File, error) {
	rows, err := db.Query(`
		SELECT id, parent_id, name, is_dir, size_bytes, mime_type, sha256, disk_path, created_at, updated_at, deleted_at
		FROM files
		WHERE parent_id = ? AND deleted_at IS NULL
		ORDER BY is_dir DESC, name ASC`, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()

	return scanFiles(rows)
}

// GetByParentAndName finds a file by parent folder and name.
func (db *DB) GetByParentAndName(parentID, name string) (*File, error) {
	f := &File{}
	var isDir int
	var createdAt, updatedAt string
	var deletedAt sql.NullString

	err := db.QueryRow(`
		SELECT id, parent_id, name, is_dir, size_bytes, mime_type, sha256, disk_path, created_at, updated_at, deleted_at
		FROM files WHERE parent_id = ? AND name = ? AND deleted_at IS NULL`, parentID, name,
	).Scan(&f.ID, &f.ParentID, &f.Name, &isDir, &f.SizeBytes, &f.MimeType, &f.SHA256, &f.DiskPath, &createdAt, &updatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get by parent and name: %w", err)
	}

	f.IsDir = isDir == 1
	f.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	f.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
	if deletedAt.Valid {
		t, _ := time.Parse(timeFormat, deletedAt.String)
		f.DeletedAt = &t
	}

	return f, nil
}

// SoftDeleteFile sets deleted_at on a file (move to trash).
func (db *DB) SoftDeleteFile(id string) error {
	now := time.Now().UTC().Format(timeFormat)
	result, err := db.Exec(`UPDATE files SET deleted_at = ?, updated_at = ? WHERE id = ? AND id != 'root'`, now, now, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("file not found or is root")
	}
	return nil
}

// RenameFile updates the name and disk_path of a file or folder,
// and recursively updates all descendant disk paths.
func (db *DB) RenameFile(id, newName string) (oldPath, newPath string, err error) {
	f, err := db.GetFile(id)
	if err != nil || f == nil {
		return "", "", fmt.Errorf("file not found")
	}

	oldPath = f.DiskPath
	// Compute new path: replace last component
	parent := path.Dir(f.DiskPath)
	if parent == "." {
		newPath = newName
	} else {
		newPath = path.Join(parent, newName)
	}

	now := time.Now().UTC().Format(timeFormat)
	_, err = db.Exec(`UPDATE files SET name = ?, disk_path = ?, updated_at = ? WHERE id = ?`,
		newName, newPath, now, id)
	if err != nil {
		return "", "", fmt.Errorf("rename: %w", err)
	}

	// If it's a directory, update all descendant disk_paths
	if f.IsDir {
		err = db.updateDescendantPaths(oldPath, newPath)
		if err != nil {
			return "", "", fmt.Errorf("update descendants: %w", err)
		}
	}

	return oldPath, newPath, nil
}

// MoveFile moves a file to a new parent folder, updating disk paths.
func (db *DB) MoveFile(id, newParentID string) (oldPath, newPath string, err error) {
	f, err := db.GetFile(id)
	if err != nil || f == nil {
		return "", "", fmt.Errorf("file not found")
	}

	newParent, err := db.GetFile(newParentID)
	if err != nil || newParent == nil {
		return "", "", fmt.Errorf("parent not found")
	}

	oldPath = f.DiskPath
	if newParent.DiskPath == "" {
		newPath = f.Name
	} else {
		newPath = path.Join(newParent.DiskPath, f.Name)
	}

	now := time.Now().UTC().Format(timeFormat)
	_, err = db.Exec(`UPDATE files SET parent_id = ?, disk_path = ?, updated_at = ? WHERE id = ? AND id != 'root'`,
		newParentID, newPath, now, id)
	if err != nil {
		return "", "", fmt.Errorf("move: %w", err)
	}

	// If it's a directory, update all descendant disk_paths
	if f.IsDir {
		err = db.updateDescendantPaths(oldPath, newPath)
		if err != nil {
			return "", "", fmt.Errorf("update descendants: %w", err)
		}
	}

	return oldPath, newPath, nil
}

// updateDescendantPaths replaces oldPrefix with newPrefix in all descendant disk_paths.
func (db *DB) updateDescendantPaths(oldPrefix, newPrefix string) error {
	// Use SQL REPLACE on disk_path for items that start with oldPrefix/
	_, err := db.Exec(`
		UPDATE files SET disk_path = ? || substr(disk_path, ?)
		WHERE disk_path LIKE ? AND id != 'root'`,
		newPrefix, len(oldPrefix)+1, oldPrefix+"%")
	return err
}

func scanFiles(rows *sql.Rows) ([]*File, error) {
	var files []*File
	for rows.Next() {
		f := &File{}
		var isDir int
		var createdAt, updatedAt string
		var deletedAt sql.NullString

		err := rows.Scan(&f.ID, &f.ParentID, &f.Name, &isDir, &f.SizeBytes, &f.MimeType, &f.SHA256, &f.DiskPath, &createdAt, &updatedAt, &deletedAt)
		if err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}

		f.IsDir = isDir == 1
		f.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		f.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
		if deletedAt.Valid {
			t, _ := time.Parse(timeFormat, deletedAt.String)
			f.DeletedAt = &t
		}

		files = append(files, f)
	}
	return files, rows.Err()
}
