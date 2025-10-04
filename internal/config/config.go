package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	PasswordManagers PasswordManagers `yaml:"password_managers" mapstructure:"password_managers"`
	Storage          Storage          `yaml:"storage" mapstructure:"storage"`
	Backup           BackupConfig     `yaml:"backup" mapstructure:"backup"`
}

// PasswordManagers holds configuration for all password managers
type PasswordManagers struct {
	Bitwarden   BitwardenConfig   `yaml:"bitwarden" mapstructure:"bitwarden"`
	OnePassword OnePasswordConfig `yaml:"onepassword" mapstructure:"onepassword"`
}

// BitwardenConfig holds Bitwarden-specific configuration
type BitwardenConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	CLIPath string `yaml:"cli_path" mapstructure:"cli_path"`
	Email   string `yaml:"email" mapstructure:"email"`
}

// OnePasswordConfig holds 1Password-specific configuration
type OnePasswordConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	CLIPath string `yaml:"cli_path" mapstructure:"cli_path"`
	Account string `yaml:"account" mapstructure:"account"`
}

// Storage holds configuration for all storage backends
type Storage struct {
	GoogleDrive GoogleDriveConfig `yaml:"google_drive" mapstructure:"google_drive"`
	USB         USBConfig         `yaml:"usb" mapstructure:"usb"`
	Local       LocalConfig       `yaml:"local" mapstructure:"local"`
}

// GoogleDriveConfig holds Google Drive-specific configuration
type GoogleDriveConfig struct {
	Enabled         bool   `yaml:"enabled" mapstructure:"enabled"`
	FolderID        string `yaml:"folder_id" mapstructure:"folder_id"`
	CredentialsPath string `yaml:"credentials_path" mapstructure:"credentials_path"`
}

// USBConfig holds USB drive-specific configuration
type USBConfig struct {
	Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
	MountPath string `yaml:"mount_path" mapstructure:"mount_path"`
	BackupDir string `yaml:"backup_dir" mapstructure:"backup_dir"`
}

// LocalConfig holds local storage-specific configuration
type LocalConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	BackupPath string `yaml:"backup_path" mapstructure:"backup_path"`
}

// BackupConfig holds backup-specific configuration
type BackupConfig struct {
	Encryption     EncryptionConfig `yaml:"encryption" mapstructure:"encryption"`
	Compression    bool             `yaml:"compression" mapstructure:"compression"`
	Retention      RetentionConfig  `yaml:"retention" mapstructure:"retention"`
	FilenameFormat string           `yaml:"filename_format" mapstructure:"filename_format"`
}

// EncryptionConfig holds encryption-specific configuration
type EncryptionConfig struct {
	Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
	Algorithm string `yaml:"algorithm" mapstructure:"algorithm"`
}

// RetentionConfig holds retention policy configuration
type RetentionConfig struct {
	KeepLast int `yaml:"keep_last" mapstructure:"keep_last"`
}

const (
	// DefaultConfigDir is the default directory for configuration files
	DefaultConfigDir = ".credstash"
	// DefaultConfigFile is the default configuration file name
	DefaultConfigFile = "config.yaml"
)

// GetConfigDir returns the full path to the configuration directory
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir), nil
}

// GetConfigPath returns the full path to the configuration file
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, DefaultConfigFile), nil
}

// Load loads the configuration from the config file
func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found at %s. Run 'credstash init' to create one", configPath)
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Read environment variables with PWBACKUP_ prefix
	viper.SetEnvPrefix("PWBACKUP")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand home directory in paths
	if err := expandPaths(&cfg); err != nil {
		return nil, fmt.Errorf("failed to expand paths: %w", err)
	}

	return &cfg, nil
}

// Save saves the configuration to the config file
func Save(cfg *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config file with restrictive permissions
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// expandPaths expands ~ to home directory in all path fields
func expandPaths(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Expand Google Drive credentials path
	if cfg.Storage.GoogleDrive.CredentialsPath != "" {
		cfg.Storage.GoogleDrive.CredentialsPath = expandHome(cfg.Storage.GoogleDrive.CredentialsPath, home)
	}

	// Expand USB mount path
	if cfg.Storage.USB.MountPath != "" {
		cfg.Storage.USB.MountPath = expandHome(cfg.Storage.USB.MountPath, home)
	}

	// Expand local backup path
	if cfg.Storage.Local.BackupPath != "" {
		cfg.Storage.Local.BackupPath = expandHome(cfg.Storage.Local.BackupPath, home)
	}

	return nil
}

// expandHome expands ~ to home directory in a path
func expandHome(path, home string) string {
	if len(path) > 0 && path[0] == '~' {
		return filepath.Join(home, path[1:])
	}
	return path
}

// GetDefault returns a default configuration
func GetDefault() *Config {
	return &Config{
		PasswordManagers: PasswordManagers{
			Bitwarden: BitwardenConfig{
				Enabled: false,
				CLIPath: "/usr/local/bin/bw",
				Email:   "",
			},
			OnePassword: OnePasswordConfig{
				Enabled: false,
				CLIPath: "/usr/local/bin/op",
				Account: "",
			},
		},
		Storage: Storage{
			GoogleDrive: GoogleDriveConfig{
				Enabled:         false,
				FolderID:        "",
				CredentialsPath: "~/.credstash/gdrive-credentials.json",
			},
			USB: USBConfig{
				Enabled:   false,
				MountPath: "/media/backup",
				BackupDir: "credstash",
			},
			Local: LocalConfig{
				Enabled:    false,
				BackupPath: "~/.credstash/backups",
			},
		},
		Backup: BackupConfig{
			Encryption: EncryptionConfig{
				Enabled:   true,
				Algorithm: "AES-256-GCM",
			},
			Compression:    true,
			Retention:      RetentionConfig{KeepLast: 10},
			FilenameFormat: "backup_%s_%s.json.enc",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Check if at least one password manager is enabled
	if !c.PasswordManagers.Bitwarden.Enabled && !c.PasswordManagers.OnePassword.Enabled {
		return fmt.Errorf("at least one password manager must be enabled")
	}

	// Check if at least one storage backend is enabled
	if !c.Storage.GoogleDrive.Enabled && !c.Storage.USB.Enabled && !c.Storage.Local.Enabled {
		return fmt.Errorf("at least one storage backend must be enabled")
	}

	// Validate Bitwarden configuration
	if c.PasswordManagers.Bitwarden.Enabled {
		if c.PasswordManagers.Bitwarden.CLIPath == "" {
			return fmt.Errorf("bitwarden CLI path is required when bitwarden is enabled")
		}
	}

	// Validate 1Password configuration
	if c.PasswordManagers.OnePassword.Enabled {
		if c.PasswordManagers.OnePassword.CLIPath == "" {
			return fmt.Errorf("1password CLI path is required when 1password is enabled")
		}
	}

	// Validate Google Drive configuration
	if c.Storage.GoogleDrive.Enabled {
		if c.Storage.GoogleDrive.CredentialsPath == "" {
			return fmt.Errorf("google drive credentials path is required when google drive is enabled")
		}
	}

	// Validate USB configuration
	if c.Storage.USB.Enabled {
		if c.Storage.USB.MountPath == "" {
			return fmt.Errorf("USB mount path is required when USB is enabled")
		}
	}

	// Validate Local storage configuration
	if c.Storage.Local.Enabled {
		if c.Storage.Local.BackupPath == "" {
			return fmt.Errorf("local backup path is required when local storage is enabled")
		}
	}

	// Validate retention policy
	if c.Backup.Retention.KeepLast < 1 {
		return fmt.Errorf("retention keep_last must be at least 1")
	}

	return nil
}
