package tasks

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/faisal/selfshare/store"
)

// ThumbSize defines a thumbnail size preset.
type ThumbSize struct {
	Name    string // "sm", "md", "lg"
	MaxSide int    // max dimension of longest side
}

var ThumbSizes = []ThumbSize{
	{"sm", 200},
	{"md", 800},
	{"lg", 1600},
}

// ThumbJob represents a thumbnail generation request.
type ThumbJob struct {
	FileID   string
	AbsPath  string
	MimeType string
}

// ThumbWorker processes thumbnail generation jobs from a channel.
type ThumbWorker struct {
	DB       *store.DB
	ThumbDir string
	Jobs     chan ThumbJob
}

// NewThumbWorker creates a worker and starts numWorkers goroutines.
func NewThumbWorker(db *store.DB, thumbDir string, numWorkers int) *ThumbWorker {
	w := &ThumbWorker{
		DB:       db,
		ThumbDir: thumbDir,
		Jobs:     make(chan ThumbJob, 100),
	}

	os.MkdirAll(thumbDir, 0755)

	for i := 0; i < numWorkers; i++ {
		go w.run()
	}

	log.Printf("Thumbnail worker started (%d goroutines)", numWorkers)
	return w
}

// Enqueue adds a thumbnail job to the queue.
func (w *ThumbWorker) Enqueue(job ThumbJob) {
	select {
	case w.Jobs <- job:
	default:
		log.Printf("Thumbnail queue full, dropping job for %s", job.FileID)
	}
}

func (w *ThumbWorker) run() {
	for job := range w.Jobs {
		w.processJob(job)
	}
}

func (w *ThumbWorker) processJob(job ThumbJob) {
	mime := job.MimeType

	if strings.HasPrefix(mime, "image/") {
		w.generateImageThumbs(job)
	} else if strings.HasPrefix(mime, "video/") {
		w.generateVideoThumb(job)
	}
}

func (w *ThumbWorker) generateImageThumbs(job ThumbJob) {
	// Skip SVG and other non-raster formats
	if job.MimeType == "image/svg+xml" {
		return
	}

	src, err := imaging.Open(job.AbsPath, imaging.AutoOrientation(true))
	if err != nil {
		log.Printf("Thumb: failed to open image %s: %v", job.FileID, err)
		return
	}

	for _, size := range ThumbSizes {
		thumbFilename := fmt.Sprintf("%s_%s.jpg", job.FileID, size.Name)
		thumbPath := filepath.Join(w.ThumbDir, thumbFilename)

		// Skip if already exists
		if _, err := os.Stat(thumbPath); err == nil {
			continue
		}

		thumb := imaging.Fit(src, size.MaxSide, size.MaxSide, imaging.Lanczos)
		if err := imaging.Save(thumb, thumbPath, imaging.JPEGQuality(80)); err != nil {
			log.Printf("Thumb: failed to save %s %s: %v", job.FileID, size.Name, err)
			continue
		}

		relPath := filepath.Join("thumbs", thumbFilename)
		if err := w.DB.SaveThumb(job.FileID, size.Name, relPath); err != nil {
			log.Printf("Thumb: failed to record %s %s: %v", job.FileID, size.Name, err)
		}
	}
}

func (w *ThumbWorker) generateVideoThumb(job ThumbJob) {
	// Video thumbnail requires ffmpeg — extract frame at 1 second
	// Check if ffmpeg is available
	ffmpegPath, err := findExecutable("ffmpeg")
	if err != nil {
		return // ffmpeg not installed, skip silently
	}

	thumbFilename := fmt.Sprintf("%s_sm.jpg", job.FileID)
	thumbPath := filepath.Join(w.ThumbDir, thumbFilename)

	if _, err := os.Stat(thumbPath); err == nil {
		return // already exists
	}

	// Extract frame: ffmpeg -i input -ss 00:00:01 -frames:v 1 -q:v 2 output.jpg
	tmpFrame := filepath.Join(w.ThumbDir, fmt.Sprintf(".tmp_%s.jpg", job.FileID))
	cmd := newCommand(ffmpegPath, "-i", job.AbsPath, "-ss", "00:00:01", "-frames:v", "1", "-q:v", "2", "-y", tmpFrame)
	if err := cmd.Run(); err != nil {
		// Try at 0 seconds if video is shorter than 1s
		cmd = newCommand(ffmpegPath, "-i", job.AbsPath, "-frames:v", "1", "-q:v", "2", "-y", tmpFrame)
		if err := cmd.Run(); err != nil {
			log.Printf("Thumb: ffmpeg failed for %s: %v", job.FileID, err)
			return
		}
	}

	// Now resize the extracted frame into all thumb sizes
	src, err := imaging.Open(tmpFrame, imaging.AutoOrientation(true))
	os.Remove(tmpFrame)
	if err != nil {
		return
	}

	for _, size := range ThumbSizes {
		fn := fmt.Sprintf("%s_%s.jpg", job.FileID, size.Name)
		tp := filepath.Join(w.ThumbDir, fn)

		thumb := imaging.Fit(src, size.MaxSide, size.MaxSide, imaging.Lanczos)
		if err := imaging.Save(thumb, tp, imaging.JPEGQuality(80)); err != nil {
			continue
		}

		relPath := filepath.Join("thumbs", fn)
		w.DB.SaveThumb(job.FileID, size.Name, relPath)
	}
}
