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
    expires_at DATETIME NOT NULL,
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

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_images_set_id ON images(set_id);
CREATE INDEX IF NOT EXISTS idx_pending_user_chat ON pending_verifications(user_id, chat_id);
CREATE INDEX IF NOT EXISTS idx_pending_expires ON pending_verifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_join_history ON user_join_history(chat_id, user_id, joined_at);
