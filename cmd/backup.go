package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/crypto"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/internal/managers"
	"github.com/harshalranjhani/stashr/internal/storage"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

var (
	managerFlag      string
	destinationFlag  string
	encryptionKey    string
	noEncrypt        bool
	promptEachBackup bool
	fullExport       bool
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup password manager vaults",
	Long: `Backup password manager vaults to configured storage destinations.

This command will:
1. Check password manager authentication
2. Export vault data
3. Compress and encrypt the data
4. Upload to configured storage backends
5. Apply retention policy to remove old backups`,
	Run: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().StringVarP(&managerFlag, "manager", "m", "all", "Password manager to backup (bitwarden, 1password, all)")
	backupCmd.Flags().StringVarP(&destinationFlag, "destination", "d", "all", "Destination to backup to (gdrive, usb, local, all)")
	backupCmd.Flags().StringVarP(&encryptionKey, "encryption-key", "k", "", "Path to encryption key (will prompt if not provided)")
	backupCmd.Flags().BoolVar(&noEncrypt, "no-encrypt", false, "Skip encryption (not recommended)")
	backupCmd.Flags().BoolVar(&promptEachBackup, "prompt-each", false, "Prompt for password for each manager (more secure)")
	backupCmd.Flags().BoolVar(&fullExport, "full-export", false, "Export full item details including passwords (slower, 1Password only)")
}

func runBackup(cmd *cobra.Command, args []string) {
	logger.Header("🔐 Password Manager Backup Tool")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Determine which managers to backup
	managersToBackup := getManagersToBackup(cfg)
	if len(managersToBackup) == 0 {
		logger.Failure("No password managers enabled or selected")
		return
	}

	// Determine which storage backends to use
	storageBackends := getStorageBackends(cfg)
	if len(storageBackends) == 0 {
		logger.Failure("No storage backends enabled or selected")
		return
	}

	// Get encryption password if needed (once for all backups)
	var password string
	if !noEncrypt && cfg.Backup.Encryption.Enabled && !promptEachBackup {
		logger.Warning("⚠️  CRITICAL: If you forget this password, your backups are LOST FOREVER!")
		logger.Info("💡 Store this password in your password manager or write it down securely")
		logger.Separator()
		password, err = utils.PromptForPassword("Enter encryption password: ")
		if err != nil {
			logger.PrintError(err)
			return
		}
		if password == "" {
			logger.Failure("Encryption password is required")
			return
		}

		// Confirm password
		confirmPassword, err := utils.PromptForPassword("Confirm encryption password: ")
		if err != nil {
			logger.PrintError(err)
			return
		}
		if password != confirmPassword {
			logger.Failure("Passwords do not match!")
			return
		}
	}

	// Backup each manager
	for _, mgr := range managersToBackup {
		logger.Separator()

		// Get password for this specific backup if prompt-each is enabled
		currentPassword := password
		if !noEncrypt && cfg.Backup.Encryption.Enabled && promptEachBackup {
			currentPassword, err = utils.PromptForPassword(fmt.Sprintf("Enter encryption password for %s: ", mgr.Name()))
			if err != nil {
				logger.PrintError(err)
				continue
			}
			if currentPassword == "" {
				logger.Failure("Encryption password is required")
				continue
			}
		}

		if err := backupManager(mgr, storageBackends, cfg, currentPassword); err != nil {
			logger.PrintError(err)
			// Continue with next manager
		}

		// Clear password from memory if prompting each time
		if promptEachBackup && currentPassword != "" {
			// Overwrite the password in memory
			for i := range currentPassword {
				_ = i // Use the variable to avoid compiler warning
			}
			currentPassword = ""
		}
	}

	logger.Separator()
	logger.Success("✅ Backup completed!")
}

func backupManager(mgr managers.Manager, storageBackends []storage.Storage, cfg *config.Config, password string) error {
	logger.Progress("Backing up %s...", mgr.Name())

	// Check if installed
	if !mgr.IsInstalled() {
		return fmt.Errorf("%s CLI is not installed", mgr.Name())
	}
	logger.Success("✓ %s CLI found", mgr.Name())

	// Check authentication
	authenticated, err := mgr.IsAuthenticated()
	if err != nil {
		return fmt.Errorf("authentication check failed: %w", err)
	}
	if !authenticated {
		return fmt.Errorf("%s is not authenticated. Please login first", mgr.Name())
	}
	logger.Success("✓ Authenticated")

	// Get item count (if available)
	itemCount, _ := mgr.GetItemCount()
	if itemCount > 0 {
		logger.Info("  Found %d items", itemCount)
	}

	// Create temporary file for export
	tmpFile, err := utils.GetTempFile(fmt.Sprintf("stashr-%s-*.json", mgr.Name()))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer utils.CleanupTempFile(tmpFile.Name())
	tmpFile.Close()

	// Export vault
	if fullExport {
		// Check if manager supports full export (1Password only)
		if op, ok := mgr.(*managers.OnePassword); ok {
			logger.Progress("Exporting vault data with full details (including passwords)...")
			logger.Warning("⚠️  This may take several minutes for large vaults...")

			// Progress callback
			currentItem := 0
			progressCallback := func(current, total int, itemTitle string) {
				currentItem = current
				if current%10 == 0 || current == total {
					logger.Info("  Processing item %d/%d: %s", current, total, itemTitle)
				}
			}

			if err := op.ExportFull(tmpFile.Name(), progressCallback); err != nil {
				return fmt.Errorf("full export failed: %w", err)
			}
			logger.Success("✓ Exported %d items with full details", currentItem)
		} else {
			logger.Warning("⚠️  Full export is only supported for 1Password. Using standard export for %s.", mgr.Name())
			if err := mgr.Export(tmpFile.Name()); err != nil {
				return fmt.Errorf("export failed: %w", err)
			}
		}
	} else {
		logger.Progress("Exporting vault data...")
		if err := mgr.Export(tmpFile.Name()); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
	}

	// Read exported data
	exportedData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to read exported data: %w", err)
	}
	originalSize := len(exportedData)
	logger.Success("✓ Exported vault data (%s)", utils.FormatBytes(int64(originalSize)))

	// Compress data if enabled
	var processedData []byte
	if cfg.Backup.Compression {
		logger.Progress("Compressing data...")
		compressedData, err := utils.CompressData(exportedData)
		if err != nil {
			return fmt.Errorf("compression failed: %w", err)
		}
		processedData = compressedData
		compressedSize := len(compressedData)
		logger.Success("✓ Compressed (%s → %s)", utils.FormatBytes(int64(originalSize)), utils.FormatBytes(int64(compressedSize)))
	} else {
		processedData = exportedData
	}

	// Encrypt data if enabled
	if !noEncrypt && cfg.Backup.Encryption.Enabled {
		logger.Progress("Encrypting backup...")
		encryptedData, err := crypto.Encrypt(processedData, password)
		if err != nil {
			return fmt.Errorf("encryption failed: %w", err)
		}
		processedData = encryptedData
		logger.Success("✓ Encrypted")
	}

	// Generate backup filename
	filenameFormat := cfg.Backup.FilenameFormat
	// If encryption is disabled, remove .enc extension
	if noEncrypt || !cfg.Backup.Encryption.Enabled {
		// Replace .enc extension with appropriate extension based on compression
		if cfg.Backup.Compression {
			filenameFormat = "backup_%s_%s.json.gz"
		} else {
			filenameFormat = "backup_%s_%s.json"
		}
	}
	filename := utils.GenerateBackupFilename(filenameFormat, mgr.Name())
	finalSize := len(processedData)

	// Upload to each storage backend
	successCount := 0
	for _, backend := range storageBackends {
		if err := uploadToBackend(backend, filename, processedData, cfg); err != nil {
			logger.Warning("⚠ %s: %v", backend.Name(), err)
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to upload to any storage backend")
	}

	logger.Success("✅ Backup completed for %s (%s)", mgr.Name(), utils.FormatBytes(int64(finalSize)))
	return nil
}

func uploadToBackend(backend storage.Storage, filename string, data []byte, cfg *config.Config) error {
	// Check availability
	available, err := backend.IsAvailable()
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("storage not available")
	}

	// Upload
	logger.Progress("Uploading to %s...", backend.Name())
	startTime := time.Now()

	if err := backend.Upload(filename, data); err != nil {
		return err
	}

	duration := time.Since(startTime)
	logger.Success("✓ Uploaded to %s (%.1fs)", backend.Name(), duration.Seconds())

	// Apply retention policy
	logger.Progress("Applying retention policy...")
	backups, err := backend.List()
	if err != nil {
		logger.Warning("Failed to list backups for retention: %v", err)
		return nil
	}

	if err := storage.ApplyRetentionPolicy(backups, cfg.Backup.Retention.KeepLast, backend.Delete); err != nil {
		logger.Warning("Failed to apply retention policy: %v", err)
	} else {
		deleted := len(backups) - cfg.Backup.Retention.KeepLast
		if deleted > 0 {
			logger.Info("  Deleted %d old backup(s)", deleted)
		}
	}

	return nil
}

func getManagersToBackup(cfg *config.Config) []managers.Manager {
	var mgrs []managers.Manager

	// Check which managers to backup based on flag
	if managerFlag == "all" || managerFlag == "bitwarden" {
		if cfg.PasswordManagers.Bitwarden.Enabled {
			mgrs = append(mgrs, managers.NewBitwarden(
				cfg.PasswordManagers.Bitwarden.CLIPath,
				cfg.PasswordManagers.Bitwarden.Email,
			))
		}
	}

	if managerFlag == "all" || managerFlag == "1password" {
		if cfg.PasswordManagers.OnePassword.Enabled {
			mgrs = append(mgrs, managers.NewOnePassword(
				cfg.PasswordManagers.OnePassword.CLIPath,
				cfg.PasswordManagers.OnePassword.Account,
			))
		}
	}

	return mgrs
}

func getStorageBackends(cfg *config.Config) []storage.Storage {
	var backends []storage.Storage

	// Check which storage backends to use based on flag
	if destinationFlag == "all" || destinationFlag == "gdrive" {
		if cfg.Storage.GoogleDrive.Enabled {
			backends = append(backends, storage.NewGoogleDrive(
				cfg.Storage.GoogleDrive.CredentialsPath,
				cfg.Storage.GoogleDrive.FolderID,
			))
		}
	}

	if destinationFlag == "all" || destinationFlag == "usb" {
		if cfg.Storage.USB.Enabled {
			backends = append(backends, storage.NewUSB(
				cfg.Storage.USB.MountPath,
				cfg.Storage.USB.BackupDir,
			))
		}
	}

	if destinationFlag == "all" || destinationFlag == "local" {
		if cfg.Storage.Local.Enabled {
			backends = append(backends, storage.NewLocal(
				cfg.Storage.Local.BackupPath,
			))
		}
	}

	return backends
}
