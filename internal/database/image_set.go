package database

import (
	"database/sql"
	"fmt"
	"time"
)

type ImageSet struct {
	ID        int64
	Label     string
	CreatedAt time.Time
}

func (db *DB) CreateImageSet(label string) (*ImageSet, error) {
	result, err := db.Exec("INSERT INTO image_sets (label) VALUES (?)", label)
	if err != nil {
		return nil, fmt.Errorf("failed to create image set: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &ImageSet{
		ID:        id,
		Label:     label,
		CreatedAt: time.Now(),
	}, nil
}

func (db *DB) GetImageSetByLabel(label string) (*ImageSet, error) {
	var set ImageSet
	err := db.QueryRow("SELECT id, label, created_at FROM image_sets WHERE label = ?", label).
		Scan(&set.ID, &set.Label, &set.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get image set: %w", err)
	}
	return &set, nil
}

func (db *DB) GetAllImageSets() ([]*ImageSet, error) {
	rows, err := db.Query("SELECT id, label, created_at FROM image_sets ORDER BY label")
	if err != nil {
		return nil, fmt.Errorf("failed to query image sets: %w", err)
	}
	defer rows.Close()

	var sets []*ImageSet
	for rows.Next() {
		var set ImageSet
		if err := rows.Scan(&set.ID, &set.Label, &set.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan image set: %w", err)
		}
		sets = append(sets, &set)
	}
	return sets, rows.Err()
}

func (db *DB) DeleteImageSet(label string) error {
	_, err := db.Exec("DELETE FROM image_sets WHERE label = ?", label)
	if err != nil {
		return fmt.Errorf("failed to delete image set: %w", err)
	}
	return nil
}

func (db *DB) GetRandomImageSets(count int) ([]*ImageSet, error) {
	rows, err := db.Query("SELECT id, label, created_at FROM image_sets ORDER BY RANDOM() LIMIT ?", count)
	if err != nil {
		return nil, fmt.Errorf("failed to query random image sets: %w", err)
	}
	defer rows.Close()

	var sets []*ImageSet
	for rows.Next() {
		var set ImageSet
		if err := rows.Scan(&set.ID, &set.Label, &set.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan image set: %w", err)
		}
		sets = append(sets, &set)
	}
	return sets, rows.Err()
}

func (db *DB) GetImageSetCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM image_sets").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count image sets: %w", err)
	}
	return count, nil
}
