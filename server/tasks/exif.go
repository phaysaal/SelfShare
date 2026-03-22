package tasks

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/faisal/selfshare/store"
	"github.com/rwcarlsen/goexif/exif"
)

// ExtractAndSaveMeta extracts metadata from a media file and saves it to the DB.
func ExtractAndSaveMeta(db *store.DB, fileID, absPath, mimeType string) {
	meta := &store.PhotoMeta{FileID: fileID}

	if strings.HasPrefix(mimeType, "image/") {
		extractImageMeta(absPath, meta)
	}
	// For video, we'd use ffprobe — skip for now, just save with dimensions=0

	if err := db.SavePhotoMeta(meta); err != nil {
		log.Printf("Failed to save photo meta for %s: %v", fileID, err)
	}
}

func extractImageMeta(path string, meta *store.PhotoMeta) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// Try EXIF extraction
	x, err := exif.Decode(f)
	if err == nil {
		// Date taken
		if dt, err := x.DateTime(); err == nil {
			meta.TakenAt = &dt
		}

		// Camera info
		if tag, err := x.Get(exif.Make); err == nil {
			meta.CameraMake, _ = tag.StringVal()
		}
		if tag, err := x.Get(exif.Model); err == nil {
			meta.CameraModel, _ = tag.StringVal()
		}

		// GPS
		if lat, lng, err := x.LatLong(); err == nil {
			meta.Lat = &lat
			meta.Lng = &lng
		}

		// Orientation
		if tag, err := x.Get(exif.Orientation); err == nil {
			if v, err := tag.Int(0); err == nil {
				meta.Orientation = v
			}
		}

		// Dimensions from EXIF
		if tag, err := x.Get(exif.PixelXDimension); err == nil {
			if v, err := tag.Int(0); err == nil {
				meta.Width = v
			}
		}
		if tag, err := x.Get(exif.PixelYDimension); err == nil {
			if v, err := tag.Int(0); err == nil {
				meta.Height = v
			}
		}
	}

	// If dimensions not from EXIF, decode image header
	if meta.Width == 0 || meta.Height == 0 {
		f.Seek(0, 0)
		cfg, _, err := image.DecodeConfig(f)
		if err == nil {
			meta.Width = cfg.Width
			meta.Height = cfg.Height
		}
	}

	// If no EXIF date, use file modification time
	if meta.TakenAt == nil {
		if stat, err := os.Stat(path); err == nil {
			t := stat.ModTime().UTC()
			meta.TakenAt = &t
		} else {
			now := time.Now().UTC()
			meta.TakenAt = &now
		}
	}
}
