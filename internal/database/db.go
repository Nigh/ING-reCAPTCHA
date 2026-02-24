package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Migrate(migrationsPath string) error {
	return db.migrateFromFile(migrationsPath)
}

func (db *DB) migrateFromFile(path string) error {
	migration := `
-- Enable WAL mode for better concurrency
PRAGMA journal_mode=WAL;

-- Image sets table
CREATE TABLE IF NOT EXISTS image_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Images table
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL,
    file_id TEXT NOT NULL,
    file_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (set_id) REFERENCES image_sets(id) ON DELETE CASCADE
);

-- Pending verifications table
CREATE TABLE IF NOT EXISTS pending_verifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    message_id INTEGER,
    correct_labels TEXT NOT NULL,
    current_step INTEGER DEFAULT 0,
    user_answers TEXT DEFAULT '[]',
    retry_count INTEGER DEFAULT 0,
    expires_at INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, user_id)
);

-- Admins table
CREATE TABLE IF NOT EXISTS admins (
    user_id INTEGER PRIMARY KEY,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User join history for anti-spam
CREATE TABLE IF NOT EXISTS user_join_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Verification failures table (tracks failures across sessions)
CREATE TABLE IF NOT EXISTS verification_failures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, user_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_images_set_id ON images(set_id);
CREATE INDEX IF NOT EXISTS idx_pending_user_chat ON pending_verifications(user_id, chat_id);
CREATE INDEX IF NOT EXISTS idx_pending_expires ON pending_verifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_join_history ON user_join_history(chat_id, user_id, joined_at);
CREATE INDEX IF NOT EXISTS idx_failures_lookup ON verification_failures(chat_id, user_id);
CREATE INDEX IF NOT EXISTS idx_failures_cleanup ON verification_failures(failed_at);

-- User language preferences
CREATE TABLE IF NOT EXISTS user_language_preferences (
    user_id INTEGER PRIMARY KEY,
    language_code TEXT NOT NULL DEFAULT 'zh',
    updated_at INTEGER DEFAULT (strftime('%s', 'now'))
);

-- Image set multilingual labels
CREATE TABLE IF NOT EXISTS image_set_labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL,
    language_code TEXT NOT NULL,
    label TEXT NOT NULL,
    created_at INTEGER DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (set_id) REFERENCES image_sets(id) ON DELETE CASCADE,
    UNIQUE(set_id, language_code)
);

CREATE INDEX IF NOT EXISTS idx_set_labels_lookup ON image_set_labels(set_id, language_code);

-- Migrate existing labels as Chinese
INSERT OR IGNORE INTO image_set_labels (set_id, language_code, label)
SELECT id, 'zh', label FROM image_sets;

-- Clean up any invalid pending_verifications with non-integer expires_at
-- (leftover from pre-migration-003 data)
DELETE FROM pending_verifications WHERE CAST(expires_at AS TEXT) LIKE '%-%';
`
	_, err := db.Exec(migration)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}
	return nil
}
