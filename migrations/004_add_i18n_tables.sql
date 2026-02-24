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
