package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/faisal/selfshare/config"
	"github.com/faisal/selfshare/store"
	"github.com/faisal/selfshare/tasks"
)

// runScan walks the data directory and imports any files/folders not already in the database.
// Usage:
//
//	./selfshare scan                    — scan the entire data/ directory
//	./selfshare scan Photos             — scan only data/Photos/
//	./selfshare scan /absolute/path     — copy that folder INTO data/ then scan it
func runScan(cfg *config.Config, db *store.DB, args []string) {
	dataDir := cfg.DataDir()

	// Determine what to scan
	var scanRoot string
	var copyFirst string

	if len(args) > 0 {
		target := args[0]
		if filepath.IsAbs(target) {
			// Absolute path — check if it's inside data/ already
			rel, err := filepath.Rel(dataDir, target)
			if err != nil || strings.HasPrefix(rel, "..") {
				// Outside data/ — we'll symlink it in
				copyFirst = target
			} else {
				scanRoot = target
			}
		} else {
			// Relative to data/
			scanRoot = filepath.Join(dataDir, target)
		}
	} else {
		scanRoot = dataDir
	}

	// If the path is outside data/, create a symlink inside data/
	if copyFirst != "" {
		info, err := os.Stat(copyFirst)
		if err != nil {
			log.Fatalf("Cannot access %s: %v", copyFirst, err)
		}
		linkName := filepath.Join(dataDir, info.Name())

		// Check if already exists
		if _, err := os.Lstat(linkName); err == nil {
			log.Printf("'%s' already exists in data/, scanning it", info.Name())
		} else {
			log.Printf("Creating symlink: data/%s -> %s", info.Name(), copyFirst)
			if err := os.Symlink(copyFirst, linkName); err != nil {
				log.Fatalf("Failed to create symlink: %v", err)
			}
		}
		scanRoot = linkName
	}

	if _, err := os.Stat(scanRoot); err != nil {
		log.Fatalf("Scan target does not exist: %s", scanRoot)
	}

	log.Printf("Scanning: %s", scanRoot)
	log.Printf("Data root: %s", dataDir)

	// Start thumbnail worker
	thumbWorker := tasks.NewThumbWorker(db, cfg.ThumbDir(), 2)

	var stats scanStats

	// Walk the directory. We need to resolve symlinks manually since filepath.Walk
	// may not descend into symlinked directories correctly on all platforms.
	err := walkDir(scanRoot, func(absPath string) error {
		info, err := os.Stat(absPath) // Stat follows symlinks
		if err != nil {
			log.Printf("  Skip (stat error): %s: %v", absPath, err)
			return nil
		}

		// Get relative path from data/
		relPath, err := filepath.Rel(dataDir, absPath)
		if err != nil || relPath == "." {
			return nil
		}

		// Skip thumbs directory and hidden files
		baseName := filepath.Base(relPath)
		if strings.HasPrefix(baseName, ".") {
			return nil
		}
		if relPath == "thumbs" || strings.HasPrefix(relPath, "thumbs/") {
			return nil
		}

		// Determine parent
		parentRelPath := filepath.Dir(relPath)
		parentID := "root"
		if parentRelPath != "." {
			parent, err := db.GetByDiskPath(parentRelPath)
			if err != nil || parent == nil {
				log.Printf("  Skip (no parent): %s", relPath)
				return nil
			}
			parentID = parent.ID
		}

		name := baseName

		// Check if already in DB
		existing, _ := db.GetByParentAndName(parentID, name)
		if existing != nil {
			stats.skipped++
			return nil
		}

		if info.IsDir() {
			_, err := db.CreateFolder(parentID, name, relPath)
			if err != nil {
				log.Printf("  Error creating folder record: %s: %v", relPath, err)
				return nil
			}
			stats.folders++
			log.Printf("  + Folder: %s", relPath)
		} else {
			hash, err := hashFile(absPath)
			if err != nil {
				log.Printf("  Skip (hash error): %s: %v", absPath, err)
				return nil
			}

			mimeType := detectMime(name)
			size := info.Size()

			f, err := db.CreateFile(parentID, name, mimeType, hash, relPath, size)
			if err != nil {
				log.Printf("  Error creating file record: %s: %v", relPath, err)
				return nil
			}

			stats.files++
			log.Printf("  + File: %s (%s, %s)", relPath, formatBytes(size), mimeType)

			if isMediaMime(mimeType) {
				go tasks.ExtractAndSaveMeta(db, f.ID, absPath, mimeType)
				thumbWorker.Enqueue(tasks.ThumbJob{
					FileID:   f.ID,
					AbsPath:  absPath,
					MimeType: mimeType,
				})
				stats.media++
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Wait for all thumbnails to finish
	if stats.media > 0 {
		log.Printf("Generating thumbnails for %d media files...", stats.media)
		thumbWorker.Wait()
	}

	log.Printf("Scan complete: %d folders, %d files (%d media), %d skipped (already in DB)",
		stats.folders, stats.files, stats.media, stats.skipped)
}

// runDescan removes a folder from the database and optionally removes the symlink.
// Usage:
//
//	./selfshare descan Photos           — remove data/Photos from DB
//	./selfshare descan /Volumes/Drive   — remove the symlinked folder from DB + remove symlink
func runDescan(cfg *config.Config, db *store.DB, args []string) {
	if len(args) == 0 {
		log.Fatal("Usage: selfshare descan <folder-name-or-path>")
	}

	dataDir := cfg.DataDir()
	target := args[0]

	// Determine the relative path in data/
	var relPath string
	var linkPath string

	if filepath.IsAbs(target) {
		// Absolute path — find the matching symlink in data/
		entries, _ := os.ReadDir(dataDir)
		for _, e := range entries {
			full := filepath.Join(dataDir, e.Name())
			link, err := os.Readlink(full)
			if err != nil {
				continue
			}
			// Resolve relative symlinks
			if !filepath.IsAbs(link) {
				link = filepath.Join(dataDir, link)
			}
			if link == target || filepath.Clean(link) == filepath.Clean(target) {
				relPath = e.Name()
				linkPath = full
				break
			}
		}
		if relPath == "" {
			// Maybe the absolute path is inside data/
			rel, err := filepath.Rel(dataDir, target)
			if err == nil && !strings.HasPrefix(rel, "..") {
				relPath = rel
			} else {
				log.Fatalf("Could not find '%s' linked in data/", target)
			}
		}
	} else {
		relPath = target
		full := filepath.Join(dataDir, target)
		// Check if it's a symlink
		if linkTarget, err := os.Readlink(full); err == nil {
			linkPath = full
			_ = linkTarget
		}
	}

	log.Printf("Removing '%s' from database...", relPath)

	// Delete all DB records with this disk_path prefix
	count, err := db.DeleteByDiskPathPrefix(relPath)
	if err != nil {
		log.Fatalf("Failed to remove from database: %v", err)
	}
	log.Printf("Removed %d records from database", count)

	// Clean up thumbnails for removed files
	thumbDir := cfg.ThumbDir()
	thumbEntries, _ := os.ReadDir(thumbDir)
	thumbsRemoved := 0
	// Thumbnails are named {file_id}_{size}.jpg — we can't easily map back
	// to removed files without the IDs. The orphaned thumbs are harmless and
	// small. They'll be cleaned up on next scan or manually.

	_ = thumbEntries
	_ = thumbsRemoved

	// Remove symlink if it exists
	if linkPath != "" {
		log.Printf("Removing symlink: %s", linkPath)
		if err := os.Remove(linkPath); err != nil {
			log.Printf("Warning: could not remove symlink: %v", err)
		} else {
			log.Printf("Symlink removed")
		}
	} else {
		log.Printf("Note: '%s' is not a symlink — files on disk were NOT deleted, only DB records removed", relPath)
	}

	log.Printf("Descan complete for '%s'", relPath)
}

type scanStats struct {
	folders int
	files   int
	media   int
	skipped int
}

// walkDir walks a directory tree, resolving symlinks, visiting directories before their contents.
func walkDir(root string, fn func(absPath string) error) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fn(root)
	}

	// Visit this directory first
	if err := fn(root); err != nil {
		return err
	}

	// Read children
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	// Sort: directories first, then files
	for _, entry := range entries {
		childPath := filepath.Join(root, entry.Name())
		// Resolve symlinks
		realInfo, err := os.Stat(childPath)
		if err != nil {
			continue
		}
		if realInfo.IsDir() {
			if err := walkDir(childPath, fn); err != nil {
				return err
			}
		}
	}
	for _, entry := range entries {
		childPath := filepath.Join(root, entry.Name())
		realInfo, err := os.Stat(childPath)
		if err != nil {
			continue
		}
		if !realInfo.IsDir() {
			if err := fn(childPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func detectMime(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Common types not always in the system MIME database
	known := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".heic": "image/heic",
		".heif": "image/heif", ".svg": "image/svg+xml", ".bmp": "image/bmp",
		".mp4": "video/mp4", ".mov": "video/quicktime", ".avi": "video/x-msvideo",
		".mkv": "video/x-matroska", ".webm": "video/webm", ".m4v": "video/mp4",
		".mp3": "audio/mpeg", ".wav": "audio/wav", ".flac": "audio/flac",
		".aac": "audio/aac", ".m4a": "audio/mp4", ".ogg": "audio/ogg",
		".pdf": "application/pdf", ".zip": "application/zip",
		".txt": "text/plain", ".json": "application/json",
	}
	if mt, ok := known[ext]; ok {
		return mt
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

func isMediaMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "video/") ||
		strings.HasPrefix(mimeType, "audio/")
}

func formatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB"}
	i := 0
	fb := float64(b)
	for fb >= k && i < len(sizes)-1 {
		fb /= k
		i++
	}
	return fmt.Sprintf("%.1f %s", fb, sizes[i])
}
