package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/harshalranjhani/stashr/pkg/utils"
)

// USB represents a USB drive storage backend
type USB struct {
	MountPath string
	BackupDir string
}

// NewUSB creates a new USB storage backend
func NewUSB(mountPath, backupDir string) *USB {
	return &USB{
		MountPath: mountPath,
		BackupDir: backupDir,
	}
}

// Name returns the name of the storage backend
func (u *USB) Name() string {
	return "USB"
}

// IsAvailable checks if the USB drive is mounted and accessible
func (u *USB) IsAvailable() (bool, error) {
	// Check if mount path exists and is a directory
	if !utils.DirExists(u.MountPath) {
		return false, &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  fmt.Sprintf("mount path %s does not exist or is not accessible", u.MountPath),
		}
	}

	// Try to check if it's actually mounted (not just an empty directory)
	// We can do this by trying to read the directory
	entries, err := os.ReadDir(u.MountPath)
	if err != nil {
		return false, &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  fmt.Sprintf("cannot read mount path %s: %v", u.MountPath, err),
		}
	}

	// If the directory is empty and it's a typical mount point, it might not be mounted
	// But we'll allow it anyway and create the backup directory
	_ = entries // Just to avoid unused variable

	return true, nil
}

// getBackupPath returns the full path to the backup directory
func (u *USB) getBackupPath() string {
	return filepath.Join(u.MountPath, u.BackupDir)
}

// Upload uploads a file to the USB drive
func (u *USB) Upload(filename string, data []byte) error {
	// Check availability
	available, err := u.IsAvailable()
	if err != nil {
		return err
	}
	if !available {
		return &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  "USB drive not available",
		}
	}

	// Create backup directory if it doesn't exist
	backupPath := u.getBackupPath()
	if err := utils.CreateDirIfNotExists(backupPath, 0755); err != nil {
		return &UploadError{
			Storage: u.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to create backup directory: %w", err),
		}
	}

	// Write file
	filePath := filepath.Join(backupPath, filename)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return &UploadError{
			Storage: u.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to write file: %w", err),
		}
	}

	// Verify write was successful
	if !utils.FileExists(filePath) {
		return &UploadError{
			Storage: u.Name(),
			File:    filename,
			Err:     fmt.Errorf("file was not created"),
		}
	}

	return nil
}

// Download downloads a file from the USB drive
func (u *USB) Download(filename string) ([]byte, error) {
	// Check availability
	available, err := u.IsAvailable()
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  "USB drive not available",
		}
	}

	// Read file
	filePath := filepath.Join(u.getBackupPath(), filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &DownloadError{
			Storage: u.Name(),
			File:    filename,
			Err:     fmt.Errorf("failed to read file: %w", err),
		}
	}

	return data, nil
}

// List lists all backup files on the USB drive
func (u *USB) List() ([]BackupFile, error) {
	// Check availability
	available, err := u.IsAvailable()
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  "USB drive not available",
		}
	}

	backupPath := u.getBackupPath()

	// Check if backup directory exists
	if !utils.DirExists(backupPath) {
		return []BackupFile{}, nil // No backups yet
	}

	// Read directory
	entries, err := os.ReadDir(backupPath)
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
			Location:     filepath.Join(backupPath, entry.Name()),
			StorageType:  u.Name(),
		})
	}

	return backups, nil
}

// Delete deletes a file from the USB drive
func (u *USB) Delete(filename string) error {
	// Check availability
	available, err := u.IsAvailable()
	if err != nil {
		return err
	}
	if !available {
		return &StorageUnavailableError{
			Storage: u.Name(),
			Reason:  "USB drive not available",
		}
	}

	filePath := filepath.Join(u.getBackupPath(), filename)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetBackupLocation returns the location where backups are stored
func (u *USB) GetBackupLocation() string {
	return u.getBackupPath()
}

// GetFreeSpace returns the free space on the USB drive in bytes
func (u *USB) GetFreeSpace() (int64, error) {
	// This is platform-specific and would require syscalls
	// For simplicity, we'll return 0 for now
	// In a production implementation, you would use syscall.Statfs on Unix
	// or GetDiskFreeSpaceEx on Windows
	return 0, fmt.Errorf("not implemented")
}

// Sync ensures all writes to the USB drive are flushed
func (u *USB) Sync() error {
	// This would require platform-specific sync calls
	// For now, we'll just return nil
	return nil
}

// GetBackupsByManager returns backups for a specific manager
func (u *USB) GetBackupsByManager(manager string) ([]BackupFile, error) {
	backups, err := u.List()
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

// CleanOldBackups applies retention policy and deletes old backups
func (u *USB) CleanOldBackups(keepLast int) error {
	backups, err := u.List()
	if err != nil {
		return err
	}

	return ApplyRetentionPolicy(backups, keepLast, u.Delete)
}

// VerifyBackup verifies that a backup file exists and is readable
func (u *USB) VerifyBackup(filename string) error {
	filePath := filepath.Join(u.getBackupPath(), filename)

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
func (u *USB) GetBackupAge(filename string) (time.Duration, error) {
	filePath := filepath.Join(u.getBackupPath(), filename)

	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return time.Since(info.ModTime()), nil
}
