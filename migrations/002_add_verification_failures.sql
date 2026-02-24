-- Add table to track verification failures across sessions
CREATE TABLE IF NOT EXISTS verification_failures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    failed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chat_id, user_id)
);

-- Index for performance
CREATE INDEX IF NOT EXISTS idx_failures_lookup ON verification_failures(chat_id, user_id);
CREATE INDEX IF NOT EXISTS idx_failures_cleanup ON verification_failures(failed_at);
