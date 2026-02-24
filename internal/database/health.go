package database

import "fmt"

func (db *DB) HasTable(name string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		name,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check table %q: %w", name, err)
	}
	return count > 0, nil
}
