package database

import (
	"database/sql"
	"fmt"
	"time"
)

// BackupRecord represents a backup in the database
type BackupRecord struct {
	ID           int64
	Filename     string
	Manager      string
	StorageType  string
	Size         int64
	CreatedAt    time.Time
	ModifiedAt   *time.Time
	Checksum     *string
	Notes        *string
	Tags         []string
}

// RecordBackup records a backup in the database
func RecordBackup(filename, manager, storageType string, size int64, tags []string, notes string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert or update backup record
	now := time.Now()
	result, err := tx.Exec(`
		INSERT INTO backups (filename, manager, storage_type, size, created_at, notes)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(filename) DO UPDATE SET
			modified_at = ?,
			size = excluded.size,
			notes = excluded.notes
	`, filename, manager, storageType, size, now, sql.NullString{String: notes, Valid: notes != ""}, now)

	if err != nil {
		return fmt.Errorf("failed to insert backup: %w", err)
	}

	// Get backup ID
	var backupID int64
	if result != nil {
		backupID, _ = result.LastInsertId()
	}
	if backupID == 0 {
		// If it was an update, fetch the ID
		err = tx.QueryRow("SELECT id FROM backups WHERE filename = ?", filename).Scan(&backupID)
		if err != nil {
			return fmt.Errorf("failed to get backup id: %w", err)
		}
	}

	// Add tags if provided
	if len(tags) > 0 {
		for _, tag := range tags {
			_, err := tx.Exec(`
				INSERT OR IGNORE INTO tags (backup_filename, tag, created_at)
				VALUES (?, ?, ?)
			`, filename, tag, now)
			if err != nil {
				return fmt.Errorf("failed to insert tag: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetBackup retrieves a backup record by filename
func GetBackup(filename string) (*BackupRecord, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	var record BackupRecord
	var modifiedAt sql.NullTime
	var checksum, notes sql.NullString

	err = db.QueryRow(`
		SELECT id, filename, manager, storage_type, size, created_at, modified_at, checksum, notes
		FROM backups WHERE filename = ?
	`, filename).Scan(
		&record.ID,
		&record.Filename,
		&record.Manager,
		&record.StorageType,
		&record.Size,
		&record.CreatedAt,
		&modifiedAt,
		&checksum,
		&notes,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get backup: %w", err)
	}

	if modifiedAt.Valid {
		record.ModifiedAt = &modifiedAt.Time
	}
	if checksum.Valid {
		record.Checksum = &checksum.String
	}
	if notes.Valid {
		record.Notes = &notes.String
	}

	// Get tags
	record.Tags, err = GetTags(filename)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// ListBackups lists all backups with optional filters
func ListBackups(manager, storageType string, tags []string) ([]BackupRecord, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT DISTINCT b.id, b.filename, b.manager, b.storage_type, b.size,
		       b.created_at, b.modified_at, b.checksum, b.notes
		FROM backups b
	`

	args := []interface{}{}
	conditions := []string{}

	// Add tag filter if specified
	if len(tags) > 0 {
		query += ` INNER JOIN tags t ON b.filename = t.backup_filename`
		placeholders := ""
		for i, tag := range tags {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf("t.tag IN (%s)", placeholders))
	}

	// Add manager filter
	if manager != "" {
		conditions = append(conditions, "b.manager = ?")
		args = append(args, manager)
	}

	// Add storage type filter
	if storageType != "" {
		conditions = append(conditions, "b.storage_type = ?")
		args = append(args, storageType)
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY b.created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var record BackupRecord
		var modifiedAt sql.NullTime
		var checksum, notes sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.Filename,
			&record.Manager,
			&record.StorageType,
			&record.Size,
			&record.CreatedAt,
			&modifiedAt,
			&checksum,
			&notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backup: %w", err)
		}

		if modifiedAt.Valid {
			record.ModifiedAt = &modifiedAt.Time
		}
		if checksum.Valid {
			record.Checksum = &checksum.String
		}
		if notes.Valid {
			record.Notes = &notes.String
		}

		// Get tags for this backup
		record.Tags, _ = GetTags(record.Filename)

		records = append(records, record)
	}

	return records, nil
}

// DeleteBackup deletes a backup record
func DeleteBackup(filename string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM backups WHERE filename = ?", filename)
	if err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	return nil
}

// UpdateBackupNotes updates the notes for a backup
func UpdateBackupNotes(filename, notes string) error {
	db, err := GetDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE backups SET notes = ?, modified_at = ?
		WHERE filename = ?
	`, sql.NullString{String: notes, Valid: notes != ""}, time.Now(), filename)

	if err != nil {
		return fmt.Errorf("failed to update notes: %w", err)
	}

	return nil
}
