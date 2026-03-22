package store

import (
	"database/sql"
	"fmt"
	"time"
)

// PhotoMeta holds EXIF and media metadata for a file.
type PhotoMeta struct {
	FileID      string
	TakenAt     *time.Time
	CameraMake  string
	CameraModel string
	Lat         *float64
	Lng         *float64
	Width       int
	Height      int
	Orientation int
	DurationSec float64
}

// PhotoWithFile combines file info with photo metadata.
type PhotoWithFile struct {
	File
	TakenAt     *time.Time
	CameraMake  string
	CameraModel string
	Width       int
	Height      int
}

// TimelineGroup represents photos grouped by year-month.
type TimelineGroup struct {
	Year   int              `json:"year"`
	Month  int              `json:"month"`
	Label  string           `json:"label"`
	Count  int              `json:"count"`
	Photos []PhotoWithFile  `json:"-"` // populated on detail request
}

// SavePhotoMeta inserts or replaces photo metadata for a file.
func (db *DB) SavePhotoMeta(meta *PhotoMeta) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO photo_meta (file_id, taken_at, camera_make, camera_model, lat, lng, width, height, orientation, duration_sec)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		meta.FileID, formatTimePtr(meta.TakenAt), meta.CameraMake, meta.CameraModel,
		meta.Lat, meta.Lng, meta.Width, meta.Height, meta.Orientation, meta.DurationSec,
	)
	if err != nil {
		return fmt.Errorf("save photo meta: %w", err)
	}
	return nil
}

// GetPhotoMeta retrieves photo metadata for a file.
func (db *DB) GetPhotoMeta(fileID string) (*PhotoMeta, error) {
	m := &PhotoMeta{FileID: fileID}
	var takenAt sql.NullString
	var lat, lng sql.NullFloat64

	err := db.QueryRow(`
		SELECT taken_at, camera_make, camera_model, lat, lng, width, height, orientation, duration_sec
		FROM photo_meta WHERE file_id = ?`, fileID,
	).Scan(&takenAt, &m.CameraMake, &m.CameraModel, &lat, &lng, &m.Width, &m.Height, &m.Orientation, &m.DurationSec)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get photo meta: %w", err)
	}

	if takenAt.Valid {
		t, _ := time.Parse(timeFormat, takenAt.String)
		m.TakenAt = &t
	}
	if lat.Valid {
		m.Lat = &lat.Float64
	}
	if lng.Valid {
		m.Lng = &lng.Float64
	}

	return m, nil
}

// ListPhotos returns photos sorted by taken_at (or created_at), paginated.
func (db *DB) ListPhotos(limit, offset int) ([]PhotoWithFile, int, error) {
	// Get total count
	var total int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM files f
		JOIN photo_meta p ON f.id = p.file_id
		WHERE f.deleted_at IS NULL AND f.is_dir = 0
		AND (f.mime_type LIKE 'image/%' OR f.mime_type LIKE 'video/%')
	`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count photos: %w", err)
	}

	rows, err := db.Query(`
		SELECT f.id, f.parent_id, f.name, f.is_dir, f.size_bytes, f.mime_type, f.sha256, f.disk_path,
		       f.created_at, f.updated_at, f.deleted_at,
		       p.taken_at, p.camera_make, p.camera_model, p.width, p.height
		FROM files f
		JOIN photo_meta p ON f.id = p.file_id
		WHERE f.deleted_at IS NULL AND f.is_dir = 0
		AND (f.mime_type LIKE 'image/%' OR f.mime_type LIKE 'video/%')
		ORDER BY COALESCE(p.taken_at, f.created_at) DESC
		LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list photos: %w", err)
	}
	defer rows.Close()

	var photos []PhotoWithFile
	for rows.Next() {
		var p PhotoWithFile
		var isDir int
		var createdAt, updatedAt string
		var deletedAt, takenAt sql.NullString

		err := rows.Scan(&p.ID, &p.ParentID, &p.Name, &isDir, &p.SizeBytes, &p.MimeType, &p.SHA256, &p.DiskPath,
			&createdAt, &updatedAt, &deletedAt,
			&takenAt, &p.CameraMake, &p.CameraModel, &p.Width, &p.Height)
		if err != nil {
			return nil, 0, fmt.Errorf("scan photo: %w", err)
		}

		p.IsDir = isDir == 1
		p.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		p.UpdatedAt, _ = time.Parse(timeFormat, updatedAt)
		if takenAt.Valid {
			t, _ := time.Parse(timeFormat, takenAt.String)
			p.TakenAt = &t
		}

		photos = append(photos, p)
	}

	return photos, total, rows.Err()
}

// ListPhotoTimeline returns photo counts grouped by year-month.
func (db *DB) ListPhotoTimeline() ([]TimelineGroup, error) {
	rows, err := db.Query(`
		SELECT
			CAST(strftime('%Y', COALESCE(p.taken_at, f.created_at)) AS INTEGER) as year,
			CAST(strftime('%m', COALESCE(p.taken_at, f.created_at)) AS INTEGER) as month,
			COUNT(*) as cnt
		FROM files f
		JOIN photo_meta p ON f.id = p.file_id
		WHERE f.deleted_at IS NULL AND f.is_dir = 0
		AND (f.mime_type LIKE 'image/%' OR f.mime_type LIKE 'video/%')
		GROUP BY year, month
		ORDER BY year DESC, month DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("timeline query: %w", err)
	}
	defer rows.Close()

	months := []string{"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}

	var groups []TimelineGroup
	for rows.Next() {
		var g TimelineGroup
		if err := rows.Scan(&g.Year, &g.Month, &g.Count); err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		if g.Month >= 1 && g.Month <= 12 {
			g.Label = fmt.Sprintf("%s %d", months[g.Month], g.Year)
		} else {
			g.Label = fmt.Sprintf("%d", g.Year)
		}
		groups = append(groups, g)
	}

	return groups, rows.Err()
}

// SaveThumb records a generated thumbnail.
func (db *DB) SaveThumb(fileID, size, diskPath string) error {
	now := time.Now().UTC().Format(timeFormat)
	_, err := db.Exec(`
		INSERT OR REPLACE INTO thumbs (file_id, size, disk_path, generated_at)
		VALUES (?, ?, ?, ?)`, fileID, size, diskPath, now)
	return err
}

// GetThumbPath returns the disk path of a thumbnail, or empty if not generated.
func (db *DB) GetThumbPath(fileID, size string) (string, error) {
	var path string
	err := db.QueryRow("SELECT disk_path FROM thumbs WHERE file_id = ? AND size = ?", fileID, size).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return path, err
}

func formatTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(timeFormat)
}
