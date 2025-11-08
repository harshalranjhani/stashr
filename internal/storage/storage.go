package storage

import (
	"fmt"
	"strings"
	"time"
)

// Storage represents a storage backend interface
type Storage interface {
	// Name returns the name of the storage backend
	Name() string

	// IsAvailable checks if the storage backend is available
	IsAvailable() (bool, error)

	// Upload uploads a file to the storage backend
	Upload(filename string, data []byte) error

	// Download downloads a file from the storage backend
	Download(filename string) ([]byte, error)

	// List lists all backup files in the storage backend
	List() ([]BackupFile, error)

	// Delete deletes a file from the storage backend
	Delete(filename string) error
}

// BackupFile represents a backup file in storage
type BackupFile struct {
	Name         string
	Size         int64
	ModifiedTime time.Time
	Location     string
	StorageType  string
}

// StorageUnavailableError indicates the storage backend is unavailable
type StorageUnavailableError struct {
	Storage string
	Reason  string
}

func (e *StorageUnavailableError) Error() string {
	return fmt.Sprintf("%s storage is unavailable: %s", e.Storage, e.Reason)
}

// UploadError indicates an error during upload
type UploadError struct {
	Storage string
	File    string
	Err     error
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("%s upload failed for %s: %v", e.Storage, e.File, e.Err)
}

func (e *UploadError) Unwrap() error {
	return e.Err
}

// DownloadError indicates an error during download
type DownloadError struct {
	Storage string
	File    string
	Err     error
}

func (e *DownloadError) Error() string {
	return fmt.Sprintf("%s download failed for %s: %v", e.Storage, e.File, e.Err)
}

func (e *DownloadError) Unwrap() error {
	return e.Err
}

// ApplyRetentionPolicy applies a retention policy to a list of backups
func ApplyRetentionPolicy(backups []BackupFile, keepLast int, deleteFunc func(string) error) error {
	if len(backups) <= keepLast {
		return nil // Nothing to delete
	}

	// Sort backups by modification time (newest first)
	// Note: This assumes the list is already sorted, but we should sort it anyway
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].ModifiedTime.Before(backups[j].ModifiedTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// Delete old backups
	for i := keepLast; i < len(backups); i++ {
		if err := deleteFunc(backups[i].Name); err != nil {
			return fmt.Errorf("failed to delete %s: %w", backups[i].Name, err)
		}
	}

	return nil
}

// shouldIgnoreFile returns true if the file should be ignored when listing backups.
// This filters out macOS metadata files and other hidden system files.
func shouldIgnoreFile(filename string) bool {
	// Filter out macOS AppleDouble metadata files (._filename)
	if strings.HasPrefix(filename, "._") {
		return true
	}

	// Filter out macOS folder metadata
	if filename == ".DS_Store" {
		return true
	}

	// Filter out other common hidden/system files
	// Note: Legitimate backup files never start with a dot based on our naming convention
	if strings.HasPrefix(filename, ".") {
		return true
	}

	return false
}
