-- Core file/folder tree
CREATE TABLE IF NOT EXISTS files (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT REFERENCES files(id),
    name        TEXT NOT NULL,
    is_dir      INTEGER NOT NULL DEFAULT 0,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    mime_type   TEXT,
    sha256      TEXT,
    disk_path   TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    deleted_at  TEXT,
    UNIQUE(parent_id, name)
);

CREATE INDEX IF NOT EXISTS idx_files_parent ON files(parent_id);
CREATE INDEX IF NOT EXISTS idx_files_sha256 ON files(sha256);
CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(deleted_at);

-- Photo-specific metadata extracted from EXIF
CREATE TABLE IF NOT EXISTS photo_meta (
    file_id      TEXT PRIMARY KEY REFERENCES files(id),
    taken_at     TEXT,
    camera_make  TEXT,
    camera_model TEXT,
    lat          REAL,
    lng          REAL,
    width        INTEGER,
    height       INTEGER,
    orientation  INTEGER,
    duration_sec REAL
);

CREATE INDEX IF NOT EXISTS idx_photo_taken ON photo_meta(taken_at);

-- Thumbnail tracking
CREATE TABLE IF NOT EXISTS thumbs (
    file_id      TEXT NOT NULL REFERENCES files(id),
    size         TEXT NOT NULL,
    disk_path    TEXT NOT NULL,
    generated_at TEXT NOT NULL,
    PRIMARY KEY (file_id, size)
);

-- Share links
CREATE TABLE IF NOT EXISTS shares (
    id             TEXT PRIMARY KEY,
    file_id        TEXT NOT NULL REFERENCES files(id),
    token          TEXT NOT NULL UNIQUE,
    password_hash  TEXT,
    expires_at     TEXT,
    max_downloads  INTEGER,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    revoked_at     TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_shares_token ON shares(token);

-- Chunked upload sessions
CREATE TABLE IF NOT EXISTS uploads (
    id           TEXT PRIMARY KEY,
    parent_id    TEXT NOT NULL REFERENCES files(id),
    filename     TEXT NOT NULL,
    total_size   INTEGER NOT NULL,
    chunk_size   INTEGER NOT NULL,
    total_chunks INTEGER NOT NULL,
    received     TEXT NOT NULL DEFAULT '[]',
    status       TEXT NOT NULL DEFAULT 'active',
    created_at   TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);

-- User accounts
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL
);

-- Active sessions for refresh token tracking
CREATE TABLE IF NOT EXISTS sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    refresh_hash TEXT NOT NULL,
    device_name  TEXT,
    created_at   TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);

-- Delta sync log
CREATE TABLE IF NOT EXISTS sync_log (
    seq       INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id   TEXT NOT NULL,
    action    TEXT NOT NULL,
    timestamp TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sync_seq ON sync_log(seq);

-- Insert the virtual root folder
INSERT OR IGNORE INTO files (id, parent_id, name, is_dir, disk_path, created_at, updated_at)
VALUES ('root', NULL, 'Root', 1, '', datetime('now'), datetime('now'));
