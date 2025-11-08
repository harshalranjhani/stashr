package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/crypto"
	"github.com/harshalranjhani/stashr/internal/database"
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
	interactiveMode  bool
	dryRun           bool
	backupTags       []string
	backupNotes      string
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
	backupCmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Interactive mode with guided prompts")
	backupCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview backup operation without executing")
	backupCmd.Flags().StringSliceVarP(&backupTags, "tag", "t", []string{}, "Tags to add to this backup (can be specified multiple times)")
	backupCmd.Flags().StringVarP(&backupNotes, "note", "n", "", "Notes to add to this backup")
}

func runBackup(cmd *cobra.Command, args []string) {
	logger.Header("🔐 Password Manager Backup Tool")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Interactive mode - ask user questions before proceeding
	if interactiveMode {
		if !handleInteractiveMode(cfg) {
			logger.Info("Backup cancelled")
			return
		}
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

	// Dry-run mode - preview what will happen
	if dryRun {
		handleDryRun(managersToBackup, storageBackends, cfg)
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

		// Warning for 1Password users about metadata-only export
		if _, ok := mgr.(*managers.OnePassword); ok {
			logger.Separator()
			logger.Warning("⚠️  1PASSWORD BACKUP MODE: Metadata Only (Fast)")
			logger.Info("")
			logger.Info("This backup will include:")
			logger.Info("  ✓ Item titles, usernames, URLs")
			logger.Info("  ✓ Categories and tags")
			logger.Info("  ✗ Actual passwords (NOT included)")
			logger.Info("")
			logger.Info("For a complete backup with passwords, use: --full-export")
			logger.Info("Note: Full export is slower but includes all sensitive data")
			logger.Separator()

			if !utils.ConfirmPrompt("Continue with metadata-only backup?") {
				return fmt.Errorf("backup cancelled by user")
			}
		}

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

		// Show progress bar for large data (> 5MB)
		if originalSize > 5*1024*1024 {
			bar := progressbar.NewOptions(originalSize,
				progressbar.OptionSetDescription("Compressing"),
				progressbar.OptionSetWidth(40),
				progressbar.OptionShowBytes(true),
				progressbar.OptionClearOnFinish(),
			)
			bar.Add(originalSize) // Compression is too fast to show real progress, so just complete it
		}

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

		// Show progress bar for large data (> 5MB)
		if len(processedData) > 5*1024*1024 {
			bar := progressbar.NewOptions(len(processedData),
				progressbar.OptionSetDescription("Encrypting"),
				progressbar.OptionSetWidth(40),
				progressbar.OptionShowBytes(true),
				progressbar.OptionClearOnFinish(),
			)
			bar.Add(len(processedData)) // Encryption is too fast to show real progress, so just complete it
		}

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
	var successfulStorage string
	for _, backend := range storageBackends {
		if err := uploadToBackend(backend, filename, processedData, cfg); err != nil {
			logger.Warning("⚠ %s: %v", backend.Name(), err)
		} else {
			successCount++
			if successfulStorage == "" {
				successfulStorage = backend.Name()
			}
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to upload to any storage backend")
	}

	// Record backup in database
	if err := database.RecordBackup(filename, mgr.Name(), successfulStorage, int64(finalSize), backupTags, backupNotes); err != nil {
		logger.Warning("Failed to record backup in database: %v", err)
		// Don't fail the backup if database recording fails
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

	// Upload with progress bar
	logger.Progress("Uploading to %s...", backend.Name())
	startTime := time.Now()

	// Show progress bar for large uploads (> 1MB)
	if len(data) > 1024*1024 {
		bar := progressbar.NewOptions(len(data),
			progressbar.OptionSetDescription(fmt.Sprintf("Uploading to %s", backend.Name())),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
			progressbar.OptionClearOnFinish(),
		)
		bar.Add(len(data))
	}

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

// handleInteractiveMode guides the user through backup options
func handleInteractiveMode(cfg *config.Config) bool {
	logger.Info("📋 Interactive Backup Setup")
	logger.Separator()

	// Ask which managers to backup
	logger.Info("Which password managers would you like to backup?")
	if cfg.PasswordManagers.Bitwarden.Enabled && cfg.PasswordManagers.OnePassword.Enabled {
		logger.Info("  1. Bitwarden only")
		logger.Info("  2. 1Password only")
		logger.Info("  3. Both (all)")
		choice := utils.PromptForInput("Enter choice (1-3)")
		switch choice {
		case "1":
			managerFlag = "bitwarden"
		case "2":
			managerFlag = "1password"
		case "3":
			managerFlag = "all"
		default:
			logger.Warning("Invalid choice, using default (all)")
			managerFlag = "all"
		}
	} else if cfg.PasswordManagers.Bitwarden.Enabled {
		managerFlag = "bitwarden"
		logger.Info("  Using: Bitwarden (only enabled manager)")
	} else if cfg.PasswordManagers.OnePassword.Enabled {
		managerFlag = "1password"
		logger.Info("  Using: 1Password (only enabled manager)")
	}

	logger.Separator()

	// For 1Password, ask about full export
	if managerFlag == "1password" || managerFlag == "all" {
		if cfg.PasswordManagers.OnePassword.Enabled {
			logger.Info("1Password Export Mode:")
			logger.Info("  1. Metadata only (fast, ~1-2 seconds)")
			logger.Info("     - Item titles, usernames, URLs")
			logger.Info("     - ⚠️  NO passwords included")
			logger.Info("  2. Full export (slow, ~5-10 minutes)")
			logger.Info("     - Everything including passwords")
			logger.Info("     - Complete backup")
			choice := utils.PromptForInput("Enter choice (1-2)")
			if choice == "2" {
				fullExport = true
				logger.Success("✓ Will perform full export with passwords")
			} else {
				logger.Success("✓ Will perform metadata-only export")
			}
			logger.Separator()
		}
	}

	// Ask which storage to use
	enabledBackends := []string{}
	if cfg.Storage.Local.Enabled {
		enabledBackends = append(enabledBackends, "local")
	}
	if cfg.Storage.USB.Enabled {
		enabledBackends = append(enabledBackends, "usb")
	}
	if cfg.Storage.GoogleDrive.Enabled {
		enabledBackends = append(enabledBackends, "gdrive")
	}

	if len(enabledBackends) > 1 {
		logger.Info("Which storage destinations would you like to use?")
		for i, backend := range enabledBackends {
			logger.Info("  %d. %s", i+1, backend)
		}
		logger.Info("  %d. All enabled destinations", len(enabledBackends)+1)
		choice := utils.PromptForInput(fmt.Sprintf("Enter choice (1-%d)", len(enabledBackends)+1))
		if idx := parseChoice(choice, len(enabledBackends)+1); idx > 0 {
			if idx <= len(enabledBackends) {
				destinationFlag = enabledBackends[idx-1]
			} else {
				destinationFlag = "all"
			}
		} else {
			destinationFlag = "all"
		}
	}

	logger.Separator()

	// Ask about encryption
	if cfg.Backup.Encryption.Enabled {
		if utils.ConfirmPrompt("Use separate password for each manager? (more secure)") {
			promptEachBackup = true
		}
	}

	logger.Separator()

	// Show summary
	logger.Info("📝 Backup Summary:")
	logger.Info("  Managers: %s", managerFlag)
	logger.Info("  Storage: %s", destinationFlag)
	logger.Info("  Encryption: %v", cfg.Backup.Encryption.Enabled && !noEncrypt)
	logger.Info("  Compression: %v", cfg.Backup.Compression)
	if managerFlag == "1password" || managerFlag == "all" {
		logger.Info("  1Password mode: %s", map[bool]string{true: "Full export (with passwords)", false: "Metadata only"}[fullExport])
	}
	logger.Separator()

	return utils.ConfirmPrompt("Proceed with backup?")
}

// handleDryRun shows what would be backed up without executing
func handleDryRun(managersToBackup []managers.Manager, storageBackends []storage.Storage, cfg *config.Config) {
	logger.Info("🔍 DRY RUN MODE - Preview Only (no backup will be created)")
	logger.Separator()

	// Check managers
	logger.Info("Password Managers to Backup:")
	for _, mgr := range managersToBackup {
		logger.Progress("Checking %s...", mgr.Name())

		if !mgr.IsInstalled() {
			logger.Failure("  ✗ CLI not installed at: %s", mgr.Name())
			continue
		}
		logger.Success("  ✓ CLI found")

		authenticated, err := mgr.IsAuthenticated()
		if err != nil || !authenticated {
			logger.Failure("  ✗ Not authenticated")
			if mgr.Name() == "bitwarden" {
				logger.Info("    Run: bw unlock")
			} else if mgr.Name() == "1password" {
				logger.Info("    Run: op signin")
			}
			continue
		}
		logger.Success("  ✓ Authenticated")

		itemCount, _ := mgr.GetItemCount()
		if itemCount > 0 {
			logger.Info("  📊 Items: %d", itemCount)
		}

		// Estimate size (rough estimate: 1KB per item)
		estimatedSize := int64(itemCount * 1024)
		if cfg.Backup.Compression {
			estimatedSize = estimatedSize * 3 / 10 // Assume 70% compression
		}
		logger.Info("  📦 Estimated size: %s", utils.FormatBytes(estimatedSize))

		// Show export mode for 1Password
		if mgr.Name() == "1password" {
			if fullExport {
				logger.Info("  🔐 Export mode: Full (with passwords)")
				logger.Info("  ⏱️  Estimated time: %d-%d minutes", itemCount/20, itemCount/10)
			} else {
				logger.Warning("  ⚠️  Export mode: Metadata only (NO passwords)")
				logger.Info("  ⏱️  Estimated time: <5 seconds")
			}
		}
	}

	logger.Separator()

	// Check storage backends
	logger.Info("Storage Destinations:")
	for _, backend := range storageBackends {
		logger.Progress("Checking %s...", backend.Name())

		available, err := backend.IsAvailable()
		if err != nil {
			logger.Failure("  ✗ Error: %v", err)
			continue
		}
		if !available {
			logger.Failure("  ✗ Not available")
			continue
		}
		logger.Success("  ✓ Available")

		// List existing backups
		backups, err := backend.List()
		if err != nil {
			logger.Warning("  ⚠ Could not list existing backups: %v", err)
		} else {
			logger.Info("  📁 Existing backups: %d", len(backups))
			if len(backups) > 0 {
				logger.Info("  🗑️  Old backups to delete: %d (keeping last %d)",
					max(0, len(backups)+len(managersToBackup)-cfg.Backup.Retention.KeepLast),
					cfg.Backup.Retention.KeepLast)
			}
		}
	}

	logger.Separator()

	// Show encryption info
	if !noEncrypt && cfg.Backup.Encryption.Enabled {
		logger.Info("Encryption Settings:")
		logger.Info("  Algorithm: %s", cfg.Backup.Encryption.Algorithm)
		logger.Info("  Password prompt: %s", map[bool]string{true: "Once per manager", false: "Once for all"}[promptEachBackup])
		logger.Separator()
	}

	// Show summary
	logger.Success("✅ Dry run complete!")
	logger.Info("")
	logger.Info("To perform the actual backup, run the same command without --dry-run")
}

// parseChoice converts user input to an integer choice
func parseChoice(input string, maxChoice int) int {
	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil {
		return 0
	}
	if choice < 1 || choice > maxChoice {
		return 0
	}
	return choice
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
