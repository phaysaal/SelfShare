# SelfShare

Self-hosted file server and photo/video manager. A single binary replaces Dropbox and Google Photos for your home network.

## Features

- **File Management** — Upload, download, organize files and folders through a web UI or mobile app
- **Photo Gallery** — Browse photos by date with auto-generated thumbnails and EXIF metadata extraction
- **Video Streaming** — Watch videos directly in the browser with seeking support (HTTP Range requests)
- **Link Sharing** — Share files via URL with optional password protection, expiry, and download limits
- **Chunked Uploads** — Reliable upload of large files (videos) with resume support
- **Folder Import** — Point SelfShare at existing folders on your drive — no copying needed
- **Mobile App** — Android (and iOS) app with file browser, photo gallery, and media viewer
- **Single Binary** — Go server with embedded web UI, SQLite database, zero external dependencies
- **Real Directory Structure** — Files on disk mirror what you see in the app — browse them in Finder too

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 18+ (for building the web UI)

### Build & Run

```bash
git clone https://github.com/phaysaal/SelfShare.git
cd SelfShare

# Build web UI
cd web && npm install && npx vite build && cd ..

# Build and run server
cd server && go build -o selfshare .
./selfshare -listen :8080
```

Open **http://localhost:8080** — create your admin account on the first visit.

### Custom Storage Location

```bash
# Use an external drive
./selfshare -listen :8080 -storage /Volumes/MyDrive/SelfShare
```

Default storage is `~/.selfshare/`.

## Import Existing Photos/Videos

Point SelfShare at an existing folder — it creates a symlink and indexes everything without copying:

```bash
# Import a folder from an external drive
./selfshare scan /Volumes/MyDrive/Photos

# Import a folder already inside the data directory
./selfshare scan Vacation

# Scan everything in the data directory
./selfshare scan
```

The scan is safe to re-run — it skips files already in the database. Photos get EXIF extraction and thumbnails automatically.

## Storage Layout

Files on disk mirror the app's directory structure:

```
~/.selfshare/
├── data/                    ← your files
│   ├── Photos/
│   │   ├── Vacation/
│   │   │   └── beach.jpg
│   │   └── sunset.jpg
│   ├── Documents/
│   │   └── report.pdf
│   └── thumbs/              ← auto-generated thumbnails
├── temp/uploads/            ← in-progress chunked uploads
├── selfshare.db             ← SQLite database
└── config.json              ← server configuration
```

## HTTPS (Let's Encrypt)

For public internet access with automatic TLS:

```bash
./selfshare -tls -domain selfshare.example.com
```

Requires ports 80 and 443, and a domain pointing to your server's public IP.

## API

All endpoints are under `/api/v1`. Authentication uses JWT Bearer tokens.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/auth/login` | Login, returns JWT tokens |
| `POST` | `/auth/refresh` | Refresh access token |
| `GET` | `/files` | List root directory |
| `GET` | `/files/{id}/children` | List folder contents |
| `POST` | `/files` | Upload file (multipart) or create folder (JSON) |
| `GET` | `/files/{id}/download` | Download file |
| `GET` | `/files/{id}/view` | View file inline (streaming) |
| `GET` | `/files/{id}/thumb?size=sm\|md\|lg` | Get thumbnail |
| `DELETE` | `/files/{id}` | Delete file |
| `PUT` | `/files/{id}` | Rename or move file |
| `POST` | `/uploads` | Start chunked upload |
| `PUT` | `/uploads/{id}/{chunk}` | Upload chunk |
| `POST` | `/uploads/{id}/complete` | Finalize chunked upload |
| `GET` | `/photos` | List photos (paginated, by date) |
| `GET` | `/photos/timeline` | Photos grouped by year/month |
| `POST` | `/shares` | Create share link |
| `GET` | `/shares` | List active shares |
| `DELETE` | `/shares/{id}` | Revoke share link |
| `GET` | `/s/{token}` | Public share page |

## Mobile App

The Flutter app is in `mobile/`. Build for Android:

```bash
cd mobile
flutter build apk --release
```

The APK will be at `mobile/build/app/outputs/flutter-apk/app-release.apk`.

## Project Structure

```
SelfShare/
├── server/                  ← Go server
│   ├── main.go              ← entry point, CLI flags
│   ├── scan.go              ← folder import command
│   ├── api/                 ← HTTP handlers + embedded SPA
│   ├── auth/                ← JWT + bcrypt
│   ├── config/              ← configuration
│   ├── storage/             ← file store (real directory layout)
│   ├── store/               ← SQLite database layer
│   └── tasks/               ← EXIF extraction + thumbnail worker
├── web/                     ← SolidJS web frontend
│   └── src/
│       ├── api/client.ts    ← typed API client
│       ├── components/      ← FileList, PhotoGallery, MediaViewer, ShareDialog
│       ├── pages/           ← Login
│       └── stores/          ← auth, files
└── mobile/                  ← Flutter mobile app
    └── lib/
        ├── api/client.dart  ← API client
        ├── models/          ← data models
        └── screens/         ← connect, login, files, gallery, viewer
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Server | Go (stdlib net/http) |
| Database | SQLite (WAL mode) |
| Web UI | SolidJS + TypeScript + Vite |
| Mobile | Flutter (Dart) |
| Thumbnails | disintegration/imaging + ffmpeg |
| EXIF | rwcarlsen/goexif |
| Auth | JWT (HMAC-SHA256) + bcrypt |
| TLS | Let's Encrypt (autocert) |

## License

MIT
