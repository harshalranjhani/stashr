package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/crypto"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/internal/storage"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

var (
	restoreSource        string
	restoreBackupFile    string
	restoreOutputPath    string
	restoreDecryptOnly   bool
	restoreLatest        bool
	restoreBefore        string
	restoreInteractive   bool
	restorePreview       bool
	restoreAutoDelete    bool
	restoreAutoDeleteMin int
)

// BackupWithSource combines a backup file with its source storage location
type BackupWithSource struct {
	Backup storage.BackupFile
	Source string
}

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore and decrypt backups",
	Long: `Restore and decrypt password manager backups.

This command will:
1. Download/locate the encrypted backup file
2. Decrypt the backup using your encryption password
3. Decompress the data
4. Save as readable JSON file

You can then manually import the JSON file into your password manager.`,
	Run: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVarP(&restoreSource, "source", "s", "", "Source to restore from (gdrive, usb, local)")
	restoreCmd.Flags().StringVarP(&restoreBackupFile, "file", "f", "", "Backup file name to restore")
	restoreCmd.Flags().StringVarP(&restoreOutputPath, "output", "o", "", "Output path for decrypted file (default: current directory)")
	restoreCmd.Flags().BoolVar(&restoreDecryptOnly, "decrypt-only", false, "Only decrypt, don't list available backups")
	restoreCmd.Flags().BoolVarP(&restoreLatest, "latest", "l", false, "Restore the most recent backup")
	restoreCmd.Flags().StringVarP(&restoreBefore, "before", "b", "", "Restore latest backup before specified date (format: 2006-01-02)")
	restoreCmd.Flags().BoolVarP(&restoreInteractive, "interactive", "i", false, "Interactive mode to select backup from list")
	restoreCmd.Flags().BoolVar(&restorePreview, "preview", false, "Preview backup metadata without decrypting")
	restoreCmd.Flags().BoolVar(&restoreAutoDelete, "auto-delete", false, "Auto-delete decrypted file after specified minutes")
	restoreCmd.Flags().IntVar(&restoreAutoDeleteMin, "auto-delete-minutes", 5, "Minutes before auto-delete (default: 5)")
}

func runRestore(cmd *cobra.Command, args []string) {
	logger.Header("🔓 Restore Backup")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Determine which backup file to restore
	selectedFile := restoreBackupFile
	selectedSource := restoreSource

	// Handle smart file selection
	if restoreLatest || restoreBefore != "" || restoreInteractive {
		file, source, err := handleSmartFileSelection(cfg)
		if err != nil {
			logger.PrintError(err)
			return
		}
		selectedFile = file
		selectedSource = source
	}

	// Validate that we have a file to restore
	if selectedFile == "" {
		logger.Failure("No backup file specified. Use --file, --latest, --before, or --interactive")
		return
	}

	// If no source specified, try to find the backup
	var backupData []byte
	var sourceName string

	if selectedSource == "" {
		logger.Progress("Searching for backup file: %s", selectedFile)
		backupData, sourceName, err = findBackupInAllSources(cfg, selectedFile)
		if err != nil {
			logger.PrintError(err)
			return
		}
		logger.Success("✓ Found backup in %s", sourceName)
	} else {
		logger.Progress("Loading backup from %s...", selectedSource)
		backupData, err = downloadBackup(cfg, selectedSource, selectedFile)
		if err != nil {
			logger.PrintError(err)
			return
		}
		sourceName = selectedSource
		logger.Success("✓ Loaded backup")
	}

	// Preview mode - show header info without decrypting
	if restorePreview {
		handlePreviewMode(backupData, selectedFile, sourceName)
		return
	}

	// Get encryption password
	fmt.Print("Enter encryption password: ")
	password, err := utils.PromptForPassword("")
	if err != nil {
		logger.PrintError(err)
		return
	}
	if password == "" {
		logger.Failure("Encryption password is required")
		return
	}

	// Decrypt backup
	logger.Progress("Decrypting backup...")
	decryptedData, err := crypto.Decrypt(backupData, password)
	if err != nil {
		logger.Failure("Failed to decrypt: %v", err)
		logger.Info("Make sure you're using the correct encryption password")
		return
	}
	logger.Success("✓ Decrypted successfully")

	// Decompress if needed
	var finalData []byte
	if cfg.Backup.Compression {
		logger.Progress("Decompressing data...")
		decompressedData, err := utils.DecompressData(decryptedData)
		if err != nil {
			logger.Warning("Failed to decompress: %v", err)
			logger.Info("Backup may not be compressed, using decrypted data as-is")
			finalData = decryptedData
		} else {
			finalData = decompressedData
			logger.Success("✓ Decompressed successfully")
		}
	} else {
		finalData = decryptedData
	}

	// Determine output path
	outputPath := restoreOutputPath
	if outputPath == "" {
		// Remove .enc extension and use current directory
		baseName := strings.TrimSuffix(selectedFile, ".enc")
		outputPath = filepath.Join(".", baseName)
	}

	// Write output file
	logger.Progress("Writing output file...")
	if err := os.WriteFile(outputPath, finalData, 0600); err != nil {
		logger.PrintError(err)
		return
	}
	logger.Success("✓ Output written to: %s", outputPath)

	// Provide next steps
	logger.Separator()
	logger.Info("✅ Backup restored successfully!")
	logger.Separator()
	logger.Info("Next steps:")

	// Determine manager from filename
	if strings.Contains(selectedFile, "bitwarden") {
		logger.Info("  1. Open Bitwarden web vault or desktop app")
		logger.Info("  2. Go to Tools → Import Data")
		logger.Info("  3. Select 'Bitwarden (json)' as format")
		logger.Info("  4. Upload the file: %s", outputPath)
	} else if strings.Contains(selectedFile, "1password") {
		logger.Info("  1. The JSON file contains your 1Password vault data")
		logger.Info("  2. You can inspect it manually or use 1Password CLI:")
		logger.Info("     op item create --vault <vault> --template <template> --title <title>")
		logger.Info("  3. Alternatively, contact 1Password support for import assistance")
		logger.Info("  4. File location: %s", outputPath)
	} else {
		logger.Info("  1. The decrypted file is at: %s", outputPath)
		logger.Info("  2. Import it into your password manager")
	}

	logger.Separator()

	// Auto-delete warning and option
	if restoreAutoDelete {
		handleAutoDelete(outputPath, restoreAutoDeleteMin)
	} else {
		logger.Warning("⚠️  SECURITY WARNING: Decrypted file contains your passwords!")
		logger.Info("")
		logger.Info("This file will NOT auto-delete. Please delete it manually after use:")
		logger.Info("  rm \"%s\"", outputPath)
		logger.Info("")
		logger.Info("Or press any key to delete it now...")

		// Wait for user input
		fmt.Println()
		if utils.ConfirmPrompt("Delete decrypted file now?") {
			if err := os.Remove(outputPath); err != nil {
				logger.Warning("Failed to delete file: %v", err)
			} else {
				logger.Success("✓ Decrypted file deleted")
			}
		}
	}
}

func findBackupInAllSources(cfg *config.Config, filename string) ([]byte, string, error) {
	// Try local storage first (fastest)
	if cfg.Storage.Local.Enabled {
		local := storage.NewLocal(cfg.Storage.Local.BackupPath)
		if data, err := local.Download(filename); err == nil {
			return data, "Local Storage", nil
		}
	}

	// Try USB
	if cfg.Storage.USB.Enabled {
		usb := storage.NewUSB(cfg.Storage.USB.MountPath, cfg.Storage.USB.BackupDir)
		if available, _ := usb.IsAvailable(); available {
			if data, err := usb.Download(filename); err == nil {
				return data, "USB Storage", nil
			}
		}
	}

	// Try Google Drive
	if cfg.Storage.GoogleDrive.Enabled {
		gdrive := storage.NewGoogleDrive(cfg.Storage.GoogleDrive.CredentialsPath, cfg.Storage.GoogleDrive.FolderID)
		if available, _ := gdrive.IsAvailable(); available {
			if data, err := gdrive.Download(filename); err == nil {
				return data, "Google Drive", nil
			}
		}
	}

	return nil, "", fmt.Errorf("backup file '%s' not found in any storage location", filename)
}

func downloadBackup(cfg *config.Config, source, filename string) ([]byte, error) {
	switch source {
	case "local":
		if !cfg.Storage.Local.Enabled {
			return nil, fmt.Errorf("local storage is not enabled")
		}
		local := storage.NewLocal(cfg.Storage.Local.BackupPath)
		return local.Download(filename)

	case "usb":
		if !cfg.Storage.USB.Enabled {
			return nil, fmt.Errorf("USB storage is not enabled")
		}
		usb := storage.NewUSB(cfg.Storage.USB.MountPath, cfg.Storage.USB.BackupDir)
		return usb.Download(filename)

	case "gdrive":
		if !cfg.Storage.GoogleDrive.Enabled {
			return nil, fmt.Errorf("Google Drive storage is not enabled")
		}
		gdrive := storage.NewGoogleDrive(cfg.Storage.GoogleDrive.CredentialsPath, cfg.Storage.GoogleDrive.FolderID)
		return gdrive.Download(filename)

	default:
		return nil, fmt.Errorf("unknown source: %s (use: local, usb, or gdrive)", source)
	}
}

// handleSmartFileSelection handles --latest, --before, and --interactive flags
func handleSmartFileSelection(cfg *config.Config) (string, string, error) {
	// Collect all backups from all sources
	allBackups := make(map[string][]storage.BackupFile)

	storageBackends := getStorageBackendsForRestore(cfg)
	if len(storageBackends) == 0 {
		return "", "", fmt.Errorf("no storage backends available")
	}

	for _, backend := range storageBackends {
		available, err := backend.IsAvailable()
		if err != nil || !available {
			continue
		}

		backups, err := backend.List()
		if err != nil {
			continue
		}

		allBackups[backend.Name()] = backups
	}

	// Flatten all backups into a single list with source info
	var flatBackups []BackupWithSource

	for sourceName, backups := range allBackups {
		for _, backup := range backups {
			flatBackups = append(flatBackups, BackupWithSource{
				Backup: backup,
				Source: sourceName,
			})
		}
	}

	if len(flatBackups) == 0 {
		return "", "", fmt.Errorf("no backups found")
	}

	// Sort by modification time (newest first)
	sort.Slice(flatBackups, func(i, j int) bool {
		return flatBackups[i].Backup.ModifiedTime.After(flatBackups[j].Backup.ModifiedTime)
	})

	// Handle --latest flag
	if restoreLatest {
		latest := flatBackups[0]
		logger.Info("Selected latest backup: %s", latest.Backup.Name)
		logger.Info("  Source: %s", latest.Source)
		logger.Info("  Modified: %s", latest.Backup.ModifiedTime.Format("2006-01-02 15:04:05"))
		logger.Info("  Size: %s", utils.FormatBytes(latest.Backup.Size))
		return latest.Backup.Name, mapSourceToFlag(latest.Source), nil
	}

	// Handle --before flag
	if restoreBefore != "" {
		beforeDate, err := time.Parse("2006-01-02", restoreBefore)
		if err != nil {
			return "", "", fmt.Errorf("invalid date format for --before (use YYYY-MM-DD): %w", err)
		}

		// Find latest backup before the specified date
		for _, item := range flatBackups {
			if item.Backup.ModifiedTime.Before(beforeDate) {
				logger.Info("Selected backup before %s: %s", restoreBefore, item.Backup.Name)
				logger.Info("  Source: %s", item.Source)
				logger.Info("  Modified: %s", item.Backup.ModifiedTime.Format("2006-01-02 15:04:05"))
				logger.Info("  Size: %s", utils.FormatBytes(item.Backup.Size))
				return item.Backup.Name, mapSourceToFlag(item.Source), nil
			}
		}
		return "", "", fmt.Errorf("no backups found before %s", restoreBefore)
	}

	// Handle --interactive flag
	if restoreInteractive {
		return handleInteractiveRestore(flatBackups)
	}

	return "", "", fmt.Errorf("no selection method specified")
}

// handleInteractiveRestore shows a menu of backups for the user to select
func handleInteractiveRestore(backups []BackupWithSource) (string, string, error) {
	logger.Info("📋 Available Backups:")
	logger.Separator()

	// Group by manager
	managerGroups := make(map[string][]BackupWithSource)
	for _, item := range backups {
		var manager string
		if strings.Contains(item.Backup.Name, "bitwarden") {
			manager = "Bitwarden"
		} else if strings.Contains(item.Backup.Name, "1password") {
			manager = "1Password"
		} else {
			manager = "Other"
		}
		managerGroups[manager] = append(managerGroups[manager], item)
	}

	// Display backups grouped by manager
	choices := []BackupWithSource{}
	choiceNum := 1

	for manager, items := range managerGroups {
		logger.Info("\n%s Backups:", manager)
		for _, item := range items {
			age := formatAge(time.Since(item.Backup.ModifiedTime))
			logger.Info("  %d. %s", choiceNum, item.Backup.Name)
			logger.Info("     Source: %s | Size: %s | Age: %s",
				item.Source,
				utils.FormatBytes(item.Backup.Size),
				age)
			choices = append(choices, item)
			choiceNum++
		}
	}

	logger.Separator()
	fmt.Printf("Enter choice (1-%d): ", len(choices))
	var choice int
	fmt.Scanln(&choice)

	if choice < 1 || choice > len(choices) {
		return "", "", fmt.Errorf("invalid choice: %d", choice)
	}

	selected := choices[choice-1]
	logger.Success("✓ Selected: %s", selected.Backup.Name)
	return selected.Backup.Name, mapSourceToFlag(selected.Source), nil
}

// handlePreviewMode shows backup metadata without decrypting
func handlePreviewMode(backupData []byte, filename, source string) {
	logger.Info("🔍 Backup Preview (without decryption)")
	logger.Separator()

	logger.Info("File Information:")
	logger.Info("  Name: %s", filename)
	logger.Info("  Source: %s", source)
	logger.Info("  Size: %s", utils.FormatBytes(int64(len(backupData))))
	logger.Separator()

	// Try to read header information
	if len(backupData) < 60 {
		logger.Warning("File too small to contain valid header")
		return
	}

	// Check magic bytes
	magic := string(backupData[0:4])
	if magic != "PWBK" {
		logger.Warning("File does not appear to be an encrypted stashr backup")
		logger.Info("Magic bytes: %s (expected: PWBK)", magic)
		return
	}

	logger.Info("Encryption Header:")
	logger.Info("  Format: Valid stashr encrypted backup")
	logger.Info("  Magic: %s ✓", magic)

	// Read version
	version := uint16(backupData[4])<<8 | uint16(backupData[5])
	logger.Info("  Version: %d", version)

	// Read algorithm
	algorithm := uint16(backupData[6])<<8 | uint16(backupData[7])
	algorithmName := "Unknown"
	if algorithm == 1 {
		algorithmName = "AES-256-GCM"
	}
	logger.Info("  Algorithm: %s", algorithmName)

	logger.Separator()

	// Determine manager from filename
	var manager string
	if strings.Contains(filename, "bitwarden") {
		manager = "Bitwarden"
	} else if strings.Contains(filename, "1password") {
		manager = "1Password"
	} else {
		manager = "Unknown"
	}
	logger.Info("Detected Manager: %s", manager)

	// Extract timestamp from filename
	if strings.Contains(filename, "_") {
		parts := strings.Split(filename, "_")
		if len(parts) >= 3 {
			dateStr := parts[len(parts)-2]
			timeStr := strings.TrimSuffix(parts[len(parts)-1], ".json.enc")
			if len(dateStr) == 8 && len(timeStr) == 6 {
				timestamp, err := time.Parse("20060102_150405", dateStr+"_"+timeStr)
				if err == nil {
					logger.Info("Backup Date: %s", timestamp.Format("2006-01-02 15:04:05"))
					logger.Info("Backup Age: %s", formatAge(time.Since(timestamp)))
				}
			}
		}
	}

	logger.Separator()
	logger.Info("To decrypt this backup, run:")
	logger.Info("  stashr restore --file %s", filename)
}

// handleAutoDelete schedules auto-deletion of the decrypted file
func handleAutoDelete(filepath string, minutes int) {
	logger.Warning("⚠️  SECURITY: Auto-delete enabled")
	logger.Info("")
	logger.Info("Decrypted file will be automatically deleted in %d minute(s)", minutes)
	logger.Info("File location: %s", filepath)
	logger.Info("")
	logger.Info("Press Ctrl+C to cancel auto-delete")
	logger.Separator()

	// Countdown
	for i := minutes; i > 0; i-- {
		if i == 1 {
			logger.Warning("⚠️  1 minute remaining until auto-delete...")
		} else if i <= 5 {
			logger.Info("%d minutes remaining...", i)
		}
		time.Sleep(1 * time.Minute)
	}

	// Delete the file
	logger.Progress("Deleting decrypted file...")
	if err := os.Remove(filepath); err != nil {
		logger.Failure("Failed to delete file: %v", err)
		logger.Info("Please delete manually: rm \"%s\"", filepath)
	} else {
		logger.Success("✓ Decrypted file deleted successfully")
	}
}

// getStorageBackendsForRestore returns all available storage backends
func getStorageBackendsForRestore(cfg *config.Config) []storage.Storage {
	var backends []storage.Storage

	if cfg.Storage.GoogleDrive.Enabled {
		backends = append(backends, storage.NewGoogleDrive(
			cfg.Storage.GoogleDrive.CredentialsPath,
			cfg.Storage.GoogleDrive.FolderID,
		))
	}

	if cfg.Storage.USB.Enabled {
		backends = append(backends, storage.NewUSB(
			cfg.Storage.USB.MountPath,
			cfg.Storage.USB.BackupDir,
		))
	}

	if cfg.Storage.Local.Enabled {
		backends = append(backends, storage.NewLocal(
			cfg.Storage.Local.BackupPath,
		))
	}

	return backends
}

// mapSourceToFlag maps storage backend name to command flag
func mapSourceToFlag(source string) string {
	switch source {
	case "Google Drive":
		return "gdrive"
	case "USB Storage":
		return "usb"
	case "Local Storage":
		return "local"
	default:
		return ""
	}
}
