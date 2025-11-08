package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db     *sql.DB
	dbOnce sync.Once
	dbErr  error
)

// GetDB returns the singleton database instance
func GetDB() (*sql.DB, error) {
	dbOnce.Do(func() {
		dbPath, err := getDBPath()
		if err != nil {
			dbErr = err
			return
		}

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
			dbErr = fmt.Errorf("failed to create database directory: %w", err)
			return
		}

		// Open database
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			dbErr = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// Set pragmas for better performance and safety
		pragmas := []string{
			"PRAGMA foreign_keys = ON",
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = NORMAL",
		}

		for _, pragma := range pragmas {
			if _, err := db.Exec(pragma); err != nil {
				dbErr = fmt.Errorf("failed to set pragma: %w", err)
				return
			}
		}

		// Initialize schema
		if err := initSchema(db); err != nil {
			dbErr = fmt.Errorf("failed to initialize schema: %w", err)
			return
		}
	})

	return db, dbErr
}

// getDBPath returns the path to the database file
func getDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".stashr", "metadata.db"), nil
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
