package database

import (
	"fmt"
	"time"
)

// AddTag adds a tag to a backup
func AddTag(filename, tag string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT OR IGNORE INTO tags (backup_filename, tag, created_at)
		VALUES (?, ?, ?)
	`, filename, tag, time.Now())

	if err != nil {
		return fmt.Errorf("failed to add tag: %w", err)
	}

	return nil
}

// RemoveTag removes a tag from a backup
func RemoveTag(filename, tag string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	result, err := db.Exec(`
		DELETE FROM tags
		WHERE backup_filename = ? AND tag = ?
	`, filename, tag)

	if err != nil {
		return fmt.Errorf("failed to remove tag: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("tag not found")
	}

	return nil
}

// GetTags returns all tags for a backup
func GetTags(filename string) ([]string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT tag FROM tags
		WHERE backup_filename = ?
		ORDER BY created_at
	`, filename)

	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// ListAllTags returns all unique tags in the database
func ListAllTags() ([]string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT DISTINCT tag FROM tags
		ORDER BY tag
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// GetBackupsByTag returns all backups with a specific tag
func GetBackupsByTag(tag string) ([]string, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT backup_filename FROM tags
		WHERE tag = ?
		ORDER BY created_at DESC
	`, tag)

	if err != nil {
		return nil, fmt.Errorf("failed to get backups by tag: %w", err)
	}
	defer rows.Close()

	var backups []string
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, fmt.Errorf("failed to scan filename: %w", err)
		}
		backups = append(backups, filename)
	}

	return backups, nil
}
