package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/harshalranjhani/stashr/pkg/utils"
)

// Local represents a local storage backend
type Local struct {
	BackupPath string
}

// NewLocal creates a new local storage backend
func NewLocal(backupPath string) *Local {
	return &Local{
		BackupPath: backupPath,
	}
}

// Name returns the name of the storage backend
func (l *Local) Name() string {
	return "Local"
}

// IsAvailable checks if the local storage is available
func (l *Local) IsAvailable() (bool, error) {
	// Local storage is always available, just need to ensure directory can be created
	return true, nil
}

// Upload uploads a file to local storage
func (l *Local) Upload(filename string, data []byte) error {
	// Create backup directory if it doesn't exist
	if err := utils.CreateDirIfNotExists(l.BackupPath, 0700); err != nil {
		return &UploadError{
			Storage: l.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to create backup directory: %w", err),
		}
	}

	// Write file
	filePath := filepath.Join(l.BackupPath, filename)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return &UploadError{
			Storage: l.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to write file: %w", err),
		}
	}

	// Verify write was successful
	if !utils.FileExists(filePath) {
		return &UploadError{
			Storage: l.Name(),
			File:    filename,
			Err:     fmt.Errorf("file was not created"),
		}
	}

	return nil
}

// Download downloads a file from local storage
func (l *Local) Download(filename string) ([]byte, error) {
	filePath := filepath.Join(l.BackupPath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &DownloadError{
			Storage: l.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to read file: %w", err),
		}
	}

	return data, nil
}

// List lists all backup files in local storage
func (l *Local) List() ([]BackupFile, error) {
	// Check if backup directory exists
	if !utils.DirExists(l.BackupPath) {
		return []BackupFile{}, nil // No backups yet
	}

	// Read directory
	entries, err := os.ReadDir(l.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip hidden/system files (e.g., ._ files, .DS_Store)
		if shouldIgnoreFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupFile{
			Name:         entry.Name(),
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
			Location:     filepath.Join(l.BackupPath, entry.Name()),
			StorageType:  l.Name(),
		})
	}

	return backups, nil
}

// Delete deletes a file from local storage
func (l *Local) Delete(filename string) error {
	filePath := filepath.Join(l.BackupPath, filename)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetBackupLocation returns the location where backups are stored
func (l *Local) GetBackupLocation() string {
	return l.BackupPath
}

// GetFreeSpace returns the free space in bytes
func (l *Local) GetFreeSpace() (int64, error) {
	// This is platform-specific and would require syscalls
	// For simplicity, we'll return 0 for now
	return 0, fmt.Errorf("not implemented")
}

// CleanOldBackups applies retention policy and deletes old backups
func (l *Local) CleanOldBackups(keepLast int) error {
	backups, err := l.List()
	if err != nil {
		return err
	}

	return ApplyRetentionPolicy(backups, keepLast, l.Delete)
}

// VerifyBackup verifies that a backup file exists and is readable
func (l *Local) VerifyBackup(filename string) error {
	filePath := filepath.Join(l.BackupPath, filename)

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("backup verification failed: %w", err)
	}

	if info.Size() == 0 {
		return fmt.Errorf("backup file is empty")
	}

	// Try to read a few bytes to ensure it's readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("backup file is not readable: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 4)
	_, err = file.Read(buf)
	if err != nil {
		return fmt.Errorf("backup file is not readable: %w", err)
	}

	return nil
}

// GetBackupAge returns the age of a backup file
func (l *Local) GetBackupAge(filename string) (time.Duration, error) {
	filePath := filepath.Join(l.BackupPath, filename)

	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return time.Since(info.ModTime()), nil
}

// GetBackupsByManager returns backups for a specific manager
func (l *Local) GetBackupsByManager(manager string) ([]BackupFile, error) {
	backups, err := l.List()
	if err != nil {
		return nil, err
	}

	var filtered []BackupFile
	for _, backup := range backups {
		// Check if filename contains the manager name
		if len(backup.Name) > len(manager) && backup.Name[:len(manager)] == "backup_"+manager {
			filtered = append(filtered, backup)
		}
	}

	return filtered, nil
}

// EnsureBackupPath ensures the backup directory exists with proper permissions
func (l *Local) EnsureBackupPath() error {
	if err := utils.CreateDirIfNotExists(l.BackupPath, 0700); err != nil {
		return fmt.Errorf("failed to ensure backup path exists: %w", err)
	}
	return nil
}
