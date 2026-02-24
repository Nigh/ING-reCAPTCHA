package database

import (
	"database/sql"
	"fmt"
)

func (db *DB) AddImageSetLabel(setID int64, langCode, label string) error {
	_, err := db.Exec(`
		INSERT INTO image_set_labels (set_id, language_code, label)
		VALUES (?, ?, ?)
		ON CONFLICT(set_id, language_code) DO UPDATE SET
			label = excluded.label
	`, setID, langCode, label)
	if err != nil {
		return fmt.Errorf("failed to add image set label: %w", err)
	}
	return nil
}

func (db *DB) GetImageSetLabel(setID int64, langCode string) (string, error) {
	var label string
	err := db.QueryRow(
		"SELECT label FROM image_set_labels WHERE set_id = ? AND language_code = ?",
		setID, langCode,
	).Scan(&label)
	if err == sql.ErrNoRows {
		return "", sql.ErrNoRows
	}
	if err != nil {
		return "", fmt.Errorf("failed to get image set label: %w", err)
	}
	return label, nil
}

func (db *DB) GetImageSetLabels(setID int64) (map[string]string, error) {
	rows, err := db.Query(
		"SELECT language_code, label FROM image_set_labels WHERE set_id = ?",
		setID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query image set labels: %w", err)
	}
	defer rows.Close()

	labels := make(map[string]string)
	for rows.Next() {
		var langCode, label string
		if err := rows.Scan(&langCode, &label); err != nil {
			return nil, fmt.Errorf("failed to scan image set label: %w", err)
		}
		labels[langCode] = label
	}
	return labels, rows.Err()
}

func (db *DB) DeleteImageSetLabel(setID int64, langCode string) error {
	_, err := db.Exec(
		"DELETE FROM image_set_labels WHERE set_id = ? AND language_code = ?",
		setID, langCode,
	)
	if err != nil {
		return fmt.Errorf("failed to delete image set label: %w", err)
	}
	return nil
}

func (db *DB) GetLocalizedLabel(setID int64, userLang, defaultLang string) (string, error) {
	var label string
	err := db.QueryRow(
		"SELECT label FROM image_set_labels WHERE set_id = ? AND language_code = ?",
		setID, userLang,
	).Scan(&label)
	if err == nil {
		return label, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to get localized label: %w", err)
	}

	if userLang != defaultLang {
		err = db.QueryRow(
			"SELECT label FROM image_set_labels WHERE set_id = ? AND language_code = ?",
			setID, defaultLang,
		).Scan(&label)
		if err == nil {
			return label, nil
		}
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("failed to get default label: %w", err)
		}
	}

	err = db.QueryRow(
		"SELECT label FROM image_sets WHERE id = ?",
		setID,
	).Scan(&label)
	if err != nil {
		return "", fmt.Errorf("failed to get fallback label: %w", err)
	}
	return label, nil
}
