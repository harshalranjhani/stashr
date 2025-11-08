package database

import (
	"database/sql"
	"fmt"
)

const schema = `
CREATE TABLE IF NOT EXISTS backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL UNIQUE,
    manager TEXT NOT NULL,
    storage_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    modified_at DATETIME,
    checksum TEXT,
    notes TEXT,
    UNIQUE(filename)
);

CREATE INDEX IF NOT EXISTS idx_backups_filename ON backups(filename);
CREATE INDEX IF NOT EXISTS idx_backups_manager ON backups(manager);
CREATE INDEX IF NOT EXISTS idx_backups_storage ON backups(storage_type);
CREATE INDEX IF NOT EXISTS idx_backups_created ON backups(created_at);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backup_filename TEXT NOT NULL,
    tag TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (backup_filename) REFERENCES backups(filename) ON DELETE CASCADE,
    UNIQUE(backup_filename, tag)
);

CREATE INDEX IF NOT EXISTS idx_tags_backup ON tags(backup_filename);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
`

// initSchema initializes the database schema
func initSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}
