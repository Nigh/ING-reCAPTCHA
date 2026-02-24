package database

import (
	"database/sql"
	"fmt"
	"time"
)

type Image struct {
	ID        int64
	SetID     int64
	FileID    string
	FilePath  sql.NullString
	CreatedAt time.Time
}

func (db *DB) AddImage(setID int64, fileID, filePath string) (*Image, error) {
	var fp sql.NullString
	if filePath != "" {
		fp = sql.NullString{String: filePath, Valid: true}
	}

	result, err := db.Exec(
		"INSERT INTO images (set_id, file_id, file_path) VALUES (?, ?, ?)",
		setID, fileID, fp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add image: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &Image{
		ID:        id,
		SetID:     setID,
		FileID:    fileID,
		FilePath:  fp,
		CreatedAt: time.Now(),
	}, nil
}

func (db *DB) GetRandomImageFromSet(setID int64) (*Image, error) {
	var img Image
	err := db.QueryRow(
		"SELECT id, set_id, file_id, file_path, created_at FROM images WHERE set_id = ? ORDER BY RANDOM() LIMIT 1",
		setID,
	).Scan(&img.ID, &img.SetID, &img.FileID, &img.FilePath, &img.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get random image: %w", err)
	}
	return &img, nil
}

func (db *DB) UpdateImageFilePath(imageID int64, filePath string) error {
	_, err := db.Exec("UPDATE images SET file_path = ? WHERE id = ?", filePath, imageID)
	if err != nil {
		return fmt.Errorf("failed to update image file path: %w", err)
	}
	return nil
}

func (db *DB) DeleteImageByID(imageID int64) error {
	_, err := db.Exec("DELETE FROM images WHERE id = ?", imageID)
	if err != nil {
		return fmt.Errorf("failed to delete image by id: %w", err)
	}
	return nil
}

func (db *DB) ListImagesWithoutCache(limit int) ([]*Image, error) {
	query := "SELECT id, set_id, file_id, file_path, created_at FROM images WHERE file_path IS NULL OR file_path = '' ORDER BY id"

	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = db.Query(query+" LIMIT ?", limit)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list images without cache: %w", err)
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.SetID, &img.FileID, &img.FilePath, &img.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan uncached image: %w", err)
		}
		images = append(images, &img)
	}

	return images, rows.Err()
}

func (db *DB) GetImageCountBySet(setID int64) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM images WHERE set_id = ?", setID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count images: %w", err)
	}
	return count, nil
}

func (db *DB) GetAllImageStats() (map[string]int, error) {
	rows, err := db.Query(`
		SELECT s.label, COUNT(i.id) as count
		FROM image_sets s
		LEFT JOIN images i ON s.id = i.set_id
		GROUP BY s.id, s.label
		ORDER BY s.label
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query image stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats[label] = count
	}
	return stats, rows.Err()
}
