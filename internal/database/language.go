package database

import (
	"database/sql"
	"fmt"
)

func (db *DB) GetUserLanguage(userID int64) (string, error) {
	var langCode string
	err := db.QueryRow("SELECT language_code FROM user_language_preferences WHERE user_id = ?", userID).Scan(&langCode)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user language: %w", err)
	}
	return langCode, nil
}

func (db *DB) SetUserLanguage(userID int64, langCode string) error {
	_, err := db.Exec(`
		INSERT INTO user_language_preferences (user_id, language_code, updated_at)
		VALUES (?, ?, strftime('%s', 'now'))
		ON CONFLICT(user_id) DO UPDATE SET
			language_code = excluded.language_code,
			updated_at = strftime('%s', 'now')
	`, userID, langCode)
	if err != nil {
		return fmt.Errorf("failed to set user language: %w", err)
	}
	return nil
}
