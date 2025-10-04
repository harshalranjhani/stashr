package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/crypto"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/internal/storage"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

var (
	restoreSource      string
	restoreBackupFile  string
	restoreOutputPath  string
	restoreDecryptOnly bool
)

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

	restoreCmd.MarkFlagRequired("file")
}

func runRestore(cmd *cobra.Command, args []string) {
	logger.Header("🔓 Restore Backup")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// If no source specified, try to find the backup
	var backupData []byte
	var sourceName string

	if restoreSource == "" {
		logger.Progress("Searching for backup file: %s", restoreBackupFile)
		backupData, sourceName, err = findBackupInAllSources(cfg, restoreBackupFile)
		if err != nil {
			logger.PrintError(err)
			return
		}
		logger.Success("✓ Found backup in %s", sourceName)
	} else {
		logger.Progress("Loading backup from %s...", restoreSource)
		backupData, err = downloadBackup(cfg, restoreSource, restoreBackupFile)
		if err != nil {
			logger.PrintError(err)
			return
		}
		sourceName = restoreSource
		logger.Success("✓ Loaded backup")
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
		baseName := strings.TrimSuffix(restoreBackupFile, ".enc")
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
	if strings.Contains(restoreBackupFile, "bitwarden") {
		logger.Info("  1. Open Bitwarden web vault or desktop app")
		logger.Info("  2. Go to Tools → Import Data")
		logger.Info("  3. Select 'Bitwarden (json)' as format")
		logger.Info("  4. Upload the file: %s", outputPath)
	} else if strings.Contains(restoreBackupFile, "1password") {
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
	logger.Warning("⚠️  Security reminder: Delete the decrypted file after importing!")
	logger.Info("  rm \"%s\"", outputPath)
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
