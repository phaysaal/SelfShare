package store

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Tag represents a user-defined tag.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Count int    `json:"count"` // number of files with this tag (populated by ListTags)
}

// CreateTag creates a new tag. Returns existing tag if name already exists.
func (db *DB) CreateTag(name, color string) (*Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("tag name cannot be empty")
	}
	if color == "" {
		color = "#4a8aff"
	}

	// Check if exists
	existing, _ := db.GetTagByName(name)
	if existing != nil {
		return existing, nil
	}

	id := uuid.New().String()
	_, err := db.Exec("INSERT INTO tags (id, name, color) VALUES (?, ?, ?)", id, name, color)
	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &Tag{ID: id, Name: name, Color: color}, nil
}

// GetTag retrieves a tag by ID.
func (db *DB) GetTag(id string) (*Tag, error) {
	t := &Tag{}
	err := db.QueryRow("SELECT id, name, color FROM tags WHERE id = ?", id).Scan(&t.ID, &t.Name, &t.Color)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetTagByName retrieves a tag by name (case-insensitive).
func (db *DB) GetTagByName(name string) (*Tag, error) {
	t := &Tag{}
	err := db.QueryRow("SELECT id, name, color FROM tags WHERE name = ? COLLATE NOCASE", name).Scan(&t.ID, &t.Name, &t.Color)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ListTags returns all tags with their file counts.
func (db *DB) ListTags() ([]Tag, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.color, COUNT(ft.file_id) as cnt
		FROM tags t
		LEFT JOIN file_tags ft ON t.id = ft.tag_id
		LEFT JOIN files f ON ft.file_id = f.id AND f.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY t.name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Count); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// UpdateTag updates a tag's name and/or color.
func (db *DB) UpdateTag(id, name, color string) error {
	if name != "" {
		if _, err := db.Exec("UPDATE tags SET name = ? WHERE id = ?", strings.TrimSpace(name), id); err != nil {
			return err
		}
	}
	if color != "" {
		if _, err := db.Exec("UPDATE tags SET color = ? WHERE id = ?", color, id); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTag removes a tag and all its file associations.
func (db *DB) DeleteTag(id string) error {
	if _, err := db.Exec("DELETE FROM file_tags WHERE tag_id = ?", id); err != nil {
		return err
	}
	_, err := db.Exec("DELETE FROM tags WHERE id = ?", id)
	return err
}

// TagFile associates a tag with a file.
func (db *DB) TagFile(fileID, tagID string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO file_tags (file_id, tag_id) VALUES (?, ?)", fileID, tagID)
	return err
}

// UntagFile removes a tag from a file.
func (db *DB) UntagFile(fileID, tagID string) error {
	_, err := db.Exec("DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?", fileID, tagID)
	return err
}

// GetFileTags returns all tags for a given file.
func (db *DB) GetFileTags(fileID string) ([]Tag, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.color
		FROM tags t JOIN file_tags ft ON t.id = ft.tag_id
		WHERE ft.file_id = ?
		ORDER BY t.name`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ListFilesByTag returns all non-deleted files with a given tag.
func (db *DB) ListFilesByTag(tagID string) ([]*File, error) {
	rows, err := db.Query(`
		SELECT f.id, f.parent_id, f.name, f.is_dir, f.size_bytes, f.mime_type, f.sha256,
		       f.disk_path, f.created_at, f.updated_at, f.deleted_at
		FROM files f JOIN file_tags ft ON f.id = ft.file_id
		WHERE ft.tag_id = ? AND f.deleted_at IS NULL
		ORDER BY f.name`, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}
