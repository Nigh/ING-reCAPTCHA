-- Convert expires_at from DATETIME to INTEGER (UNIX timestamp)
-- This migration handles existing data

-- Create a new table with the correct schema
CREATE TABLE IF NOT EXISTS pending_verifications_new (
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

-- Copy data from old table to new table, converting expires_at to UNIX timestamp
INSERT INTO pending_verifications_new (id, chat_id, user_id, message_id, correct_labels, current_step, user_answers, retry_count, expires_at, created_at)
SELECT id, chat_id, user_id, message_id, correct_labels, current_step, user_answers, retry_count,
       CAST(strftime('%s', expires_at) AS INTEGER), created_at
FROM pending_verifications;

-- Drop old table
DROP TABLE pending_verifications;

-- Rename new table to original name
ALTER TABLE pending_verifications_new RENAME TO pending_verifications;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_pending_user_chat ON pending_verifications(user_id, chat_id);
CREATE INDEX IF NOT EXISTS idx_pending_expires ON pending_verifications(expires_at);
