package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileStore handles file storage mirroring the app's directory structure.
// Files on disk match exactly what users see in the app:
//
//	data/Photos/vacation.jpg
//	data/Work/report.pdf
type FileStore struct {
	root string // root directory for file storage (the "data" dir)
}

// NewFileStore creates a new file store at the given root directory.
func NewFileStore(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("create file store root: %w", err)
	}
	return &FileStore{root: root}, nil
}

// Store writes data from r to the given relative path under the store root.
// Creates parent directories as needed.
// Returns the SHA256 hash and number of bytes written.
func (fs *FileStore) Store(relPath string, r io.Reader) (hash string, size int64, err error) {
	absPath := fs.AbsPath(relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", 0, fmt.Errorf("create parent dirs: %w", err)
	}

	// Write to a temp file first, then rename (atomic-ish)
	tmpFile, err := os.CreateTemp(filepath.Dir(absPath), ".tmp-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	size, err = io.Copy(writer, r)
	if err != nil {
		tmpFile.Close()
		return "", 0, fmt.Errorf("write file: %w", err)
	}
	tmpFile.Close()

	hash = hex.EncodeToString(hasher.Sum(nil))

	if err := os.Rename(tmpPath, absPath); err != nil {
		return "", 0, fmt.Errorf("rename to final path: %w", err)
	}

	return hash, size, nil
}

// Open returns a reader for the file at the given relative path.
func (fs *FileStore) Open(relPath string) (*os.File, error) {
	f, err := os.Open(fs.AbsPath(relPath))
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", relPath, err)
	}
	return f, nil
}

// MkdirAll creates a directory (and parents) at the given relative path.
func (fs *FileStore) MkdirAll(relPath string) error {
	return os.MkdirAll(fs.AbsPath(relPath), 0755)
}

// Rename moves/renames a file or directory on disk.
func (fs *FileStore) Rename(oldRel, newRel string) error {
	oldAbs := fs.AbsPath(oldRel)
	newAbs := fs.AbsPath(newRel)

	// Ensure new parent directory exists
	if err := os.MkdirAll(filepath.Dir(newAbs), 0755); err != nil {
		return fmt.Errorf("create target parent: %w", err)
	}

	return os.Rename(oldAbs, newAbs)
}

// Delete removes a file from disk.
func (fs *FileStore) Delete(relPath string) error {
	return os.Remove(fs.AbsPath(relPath))
}

// DeleteAll removes a file or directory (and contents) from disk.
func (fs *FileStore) DeleteAll(relPath string) error {
	return os.RemoveAll(fs.AbsPath(relPath))
}

// Exists checks if a file or directory exists at the given relative path.
func (fs *FileStore) Exists(relPath string) bool {
	_, err := os.Stat(fs.AbsPath(relPath))
	return err == nil
}

// AbsPath returns the absolute filesystem path for a relative path.
func (fs *FileStore) AbsPath(relPath string) string {
	return filepath.Join(fs.root, relPath)
}

// Root returns the store's root directory.
func (fs *FileStore) Root() string {
	return fs.root
}
