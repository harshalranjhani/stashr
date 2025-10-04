package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long: `Initialize stashr configuration by creating a config file
and setting up necessary credentials.

This interactive wizard will guide you through:
- Detecting installed password manager CLIs
- Configuring storage backends (Google Drive, USB)
- Setting up encryption preferences
- Creating the configuration file`,
	Run: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) {
	logger.Header("🔐 Password Manager Backup Tool - Setup")

	// Check if config already exists
	configPath, err := config.GetConfigPath()
	if err != nil {
		logger.PrintError(err)
		return
	}

	if utils.FileExists(configPath) {
		logger.Warning("Configuration file already exists at: %s", configPath)
		if !utils.ConfirmPrompt("Do you want to overwrite it?") {
			logger.Info("Setup cancelled")
			return
		}
	}

	// Create default config
	cfg := config.GetDefault()

	reader := bufio.NewReader(os.Stdin)

	// Configure password managers
	logger.Separator()
	logger.Progress("Detecting password managers...")
	logger.Separator()

	// Bitwarden
	if utils.CommandExists("bw") {
		logger.Success("Bitwarden CLI detected")
		if promptYesNo(reader, "Enable Bitwarden backups?") {
			cfg.PasswordManagers.Bitwarden.Enabled = true
			path, _ := utils.RunCommand("which", "bw")
			cfg.PasswordManagers.Bitwarden.CLIPath = strings.TrimSpace(string(path))

			email := promptInput(reader, "Bitwarden email (optional)")
			if email != "" {
				cfg.PasswordManagers.Bitwarden.Email = email
			}
		}
	} else {
		logger.Warning("Bitwarden CLI not found")
		logger.Info("Install from: https://bitwarden.com/help/cli/")
	}

	// 1Password
	if utils.CommandExists("op") {
		logger.Success("1Password CLI detected")
		if promptYesNo(reader, "Enable 1Password backups?") {
			cfg.PasswordManagers.OnePassword.Enabled = true
			path, _ := utils.RunCommand("which", "op")
			cfg.PasswordManagers.OnePassword.CLIPath = strings.TrimSpace(string(path))

			account := promptInput(reader, "1Password account (e.g., my.1password.com, optional)")
			if account != "" {
				cfg.PasswordManagers.OnePassword.Account = account
			}
		}
	} else {
		logger.Warning("1Password CLI not found")
		logger.Info("Install from: https://developer.1password.com/docs/cli/")
	}

	// Configure storage backends
	logger.Separator()
	logger.Progress("Configuring storage backends...")
	logger.Separator()

	// Google Drive
	if promptYesNo(reader, "Enable Google Drive storage?") {
		cfg.Storage.GoogleDrive.Enabled = true

		logger.Info("Google Drive requires OAuth2 credentials.")
		logger.Info("You'll need to create a project and download credentials from:")
		logger.Info("https://console.cloud.google.com/apis/credentials")

		credsPath := promptInput(reader, "Path to Google Drive credentials JSON file")
		if credsPath != "" {
			cfg.Storage.GoogleDrive.CredentialsPath = credsPath
		}

		logger.Info("You can create a dedicated backup folder in Google Drive.")
		logger.Info("Leave empty to store backups in root directory.")
		folderID := promptInput(reader, "Google Drive folder ID (optional)")
		if folderID != "" {
			cfg.Storage.GoogleDrive.FolderID = folderID
		}
	}

	// USB Storage
	if promptYesNo(reader, "Enable USB storage?") {
		cfg.Storage.USB.Enabled = true

		mountPath := promptInput(reader, "USB mount path (e.g., /media/backup)")
		if mountPath != "" {
			cfg.Storage.USB.MountPath = mountPath
		}

		backupDir := promptInput(reader, "Backup directory name (default: stashr)")
		if backupDir != "" {
			cfg.Storage.USB.BackupDir = backupDir
		} else {
			cfg.Storage.USB.BackupDir = "stashr"
		}
	}

	// Local Storage (Fallback)
	logger.Separator()
	logger.Info("Local storage serves as a reliable fallback when cloud/USB is unavailable")
	if promptYesNo(reader, "Enable local storage? (recommended as fallback)") {
		cfg.Storage.Local.Enabled = true

		defaultPath := "~/.stashr/backups"
		backupPath := promptInput(reader, fmt.Sprintf("Local backup path (default: %s)", defaultPath))
		if backupPath != "" {
			cfg.Storage.Local.BackupPath = backupPath
		} else {
			cfg.Storage.Local.BackupPath = defaultPath
		}
	}

	// Configure backup settings
	logger.Separator()
	logger.Progress("Configuring backup settings...")
	logger.Separator()

	if promptYesNo(reader, "Enable encryption? (recommended)") {
		cfg.Backup.Encryption.Enabled = true
		cfg.Backup.Encryption.Algorithm = "AES-256-GCM"
	} else {
		cfg.Backup.Encryption.Enabled = false
		logger.Warning("Backups will NOT be encrypted!")
	}

	if promptYesNo(reader, "Enable compression?") {
		cfg.Backup.Compression = true
	}

	retentionInput := promptInput(reader, "Number of backups to keep (default: 10)")
	if retentionInput != "" {
		var retention int
		if _, err := fmt.Sscanf(retentionInput, "%d", &retention); err == nil && retention > 0 {
			cfg.Backup.Retention.KeepLast = retention
		}
	}

	// Validate configuration
	logger.Separator()
	logger.Progress("Validating configuration...")
	if err := cfg.Validate(); err != nil {
		logger.Failure("Configuration validation failed: %v", err)
		return
	}
	logger.Success("Configuration is valid")

	// Save configuration
	if err := config.Save(cfg); err != nil {
		logger.PrintError(err)
		return
	}

	logger.Separator()
	logger.Success("Configuration saved to: %s", configPath)
	logger.Separator()
	logger.Info("Next steps:")
	logger.Info("  1. Ensure your password manager CLI is authenticated")
	logger.Info("  2. If using Google Drive, run a test backup to complete OAuth2 flow")
	logger.Info("  3. Run 'stashr backup' to create your first backup")
	logger.Separator()
}

func promptYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s (y/n): ", prompt)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func promptInput(reader *bufio.Reader, prompt string) string {
	fmt.Printf("%s: ", prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
