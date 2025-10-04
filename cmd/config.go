package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/harshalranjhani/credstash/internal/config"
	"github.com/harshalranjhani/credstash/internal/logger"
	"github.com/harshalranjhani/credstash/internal/managers"
	"github.com/harshalranjhani/credstash/internal/storage"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage credstash configuration.

Subcommands:
  show     - Display current configuration
  validate - Validate configuration and test connections`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration (sensitive data will be redacted).`,
	Run:   runConfigShow,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long: `Validate the configuration file and test connections to password managers
and storage backends.`,
	Run: runConfigValidate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) {
	logger.Header("⚙️  Configuration")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Get config path
	configPath, _ := config.GetConfigPath()
	logger.Info("Configuration file: %s", configPath)
	logger.Separator()

	// Marshal to YAML for display
	data, err := yaml.Marshal(cfg)
	if err != nil {
		logger.PrintError(err)
		return
	}

	fmt.Println(string(data))
	logger.Separator()
}

func runConfigValidate(cmd *cobra.Command, args []string) {
	logger.Header("✓ Configuration Validation")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	logger.Progress("Validating configuration file...")

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Failure("Configuration validation failed: %v", err)
		return
	}
	logger.Success("✓ Configuration is valid")

	// Test password managers
	logger.Separator()
	logger.Progress("Testing password managers...")
	logger.Separator()

	var managersOK int
	var managersTotal int

	if cfg.PasswordManagers.Bitwarden.Enabled {
		managersTotal++
		bw := managers.NewBitwarden(cfg.PasswordManagers.Bitwarden.CLIPath, cfg.PasswordManagers.Bitwarden.Email)

		if !bw.IsInstalled() {
			logger.Failure("✗ Bitwarden: CLI not found at %s", cfg.PasswordManagers.Bitwarden.CLIPath)
		} else {
			logger.Success("✓ Bitwarden: CLI found")

			authenticated, err := bw.IsAuthenticated()
			if err != nil {
				logger.Warning("  ⚠ Authentication check failed: %v", err)
			} else if !authenticated {
				logger.Warning("  ⚠ Not authenticated")
			} else {
				logger.Success("  ✓ Authenticated")
				managersOK++
			}
		}
	}

	if cfg.PasswordManagers.OnePassword.Enabled {
		managersTotal++
		op := managers.NewOnePassword(cfg.PasswordManagers.OnePassword.CLIPath, cfg.PasswordManagers.OnePassword.Account)

		if !op.IsInstalled() {
			logger.Failure("✗ 1Password: CLI not found at %s", cfg.PasswordManagers.OnePassword.CLIPath)
		} else {
			logger.Success("✓ 1Password: CLI found")

			authenticated, err := op.IsAuthenticated()
			if err != nil {
				logger.Warning("  ⚠ Authentication check failed: %v", err)
			} else if !authenticated {
				logger.Warning("  ⚠ Not authenticated")
			} else {
				logger.Success("  ✓ Authenticated")
				managersOK++
			}
		}
	}

	// Test storage backends
	logger.Separator()
	logger.Progress("Testing storage backends...")
	logger.Separator()

	var storageOK int
	var storageTotal int

	if cfg.Storage.GoogleDrive.Enabled {
		storageTotal++
		gdrive := storage.NewGoogleDrive(cfg.Storage.GoogleDrive.CredentialsPath, cfg.Storage.GoogleDrive.FolderID)

		available, err := gdrive.IsAvailable()
		if err != nil {
			logger.Failure("✗ Google Drive: %v", err)
		} else if !available {
			logger.Failure("✗ Google Drive: Not available")
		} else {
			logger.Success("✓ Google Drive: Available")
			storageOK++
		}
	}

	if cfg.Storage.USB.Enabled {
		storageTotal++
		usb := storage.NewUSB(cfg.Storage.USB.MountPath, cfg.Storage.USB.BackupDir)

		available, err := usb.IsAvailable()
		if err != nil {
			logger.Warning("⚠ USB: %v", err)
			logger.Info("  (USB drives may not always be connected)")
		} else if !available {
			logger.Warning("⚠ USB: Not available")
			logger.Info("  (USB drives may not always be connected)")
		} else {
			logger.Success("✓ USB: Available at %s", cfg.Storage.USB.MountPath)
			storageOK++
		}
	}

	if cfg.Storage.Local.Enabled {
		storageTotal++
		local := storage.NewLocal(cfg.Storage.Local.BackupPath)

		available, err := local.IsAvailable()
		if err != nil {
			logger.Failure("✗ Local: %v", err)
		} else if !available {
			logger.Failure("✗ Local: Not available")
		} else {
			logger.Success("✓ Local: Available at %s", cfg.Storage.Local.BackupPath)
			storageOK++
		}
	}

	// Summary
	logger.Separator()
	logger.Info("Summary:")
	logger.Info("  Password Managers: %d/%d ready", managersOK, managersTotal)
	logger.Info("  Storage Backends: %d/%d available", storageOK, storageTotal)
	logger.Separator()

	if managersOK == managersTotal && storageOK > 0 {
		logger.Success("✅ All systems ready!")
	} else if managersOK == 0 {
		logger.Failure("❌ No password managers are ready")
		logger.Info("Please authenticate your password manager CLIs before backing up")
	} else if storageOK == 0 {
		logger.Failure("❌ No storage backends are available")
		logger.Info("Please configure at least one storage backend")
	} else {
		logger.Warning("⚠ Some systems are not ready")
		logger.Info("Review the messages above for details")
	}
}
