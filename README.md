# 🔐 Password Manager Backup Tool

A production-ready CLI tool for securely backing up password manager vaults to multiple destinations. Built with Go, focusing on security, reliability, and ease of use.

## Features

- **Multiple Password Managers**: Supports Bitwarden and 1Password
- **Multiple Storage Backends**: Google Drive, USB, and local storage
- **Local Fallback**: Automatic local storage when cloud/USB is unavailable
- **Strong Encryption**: AES-256-GCM encryption for all backups
- **Compression**: Gzip compression to reduce backup size
- **Retention Policy**: Automatic cleanup of old backups
- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Easy Configuration**: Interactive setup wizard
- **Secure by Default**: File permissions, encrypted backups, and secure key handling

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage](#usage)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [Future Enhancements](#future-enhancements)
- [License](#license)

## Installation

### Prerequisites

- Go 1.21 or later
- Password manager CLI tools:
  - [Bitwarden CLI](https://bitwarden.com/help/cli/) (if using Bitwarden)
  - [1Password CLI](https://developer.1password.com/docs/cli/) (if using 1Password)
- Google Drive API credentials (if using Google Drive storage)

### From Source

```bash
# Clone the repository
git clone https://github.com/harshalranjhani/credstash.git
cd credstash

# Build the binary
go build -o credstash

# Install to your PATH (optional)
sudo mv credstash /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/harshalranjhani/credstash@latest
```

## Quick Start

### 1. Initialize Configuration

Run the interactive setup wizard:

```bash
credstash init
```

This will:
- Detect installed password manager CLIs
- Guide you through storage backend setup
- Create a configuration file at `~/.credstash/config.yaml`
- Set up encryption preferences

### 2. Configure Your Password Manager

#### Bitwarden

```bash
# Login
bw login your@email.com

# Unlock vault
bw unlock

# Export session token (copy the output)
export BW_SESSION="your-session-token"
```

#### 1Password

```bash
# Sign in
op signin

# Verify authentication
op whoami
```

### 3. Set Up Google Drive (Optional)

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API
4. Create OAuth 2.0 credentials (Desktop application)
5. Download the credentials JSON file
6. Save it to `~/.credstash/gdrive-credentials.json`

### 4. Run Your First Backup

```bash
credstash backup
```

You'll be prompted for an encryption password. This password will be used to encrypt your backups.

## Configuration

### Configuration File

The configuration file is located at `~/.credstash/config.yaml`. Here's an example:

```yaml
password_managers:
  bitwarden:
    enabled: true
    cli_path: "/usr/local/bin/bw"
    email: "user@example.com"
  onepassword:
    enabled: false
    cli_path: "/usr/local/bin/op"
    account: "my.1password.com"

storage:
  google_drive:
    enabled: true
    folder_id: ""
    credentials_path: "~/.credstash/gdrive-credentials.json"
  usb:
    enabled: true
    mount_path: "/media/backup"
    backup_dir: "credstash"
  local:
    enabled: true
    backup_path: "~/.credstash/backups"  # Local fallback storage

backup:
  encryption:
    enabled: true
    algorithm: "AES-256-GCM"
  compression: true
  retention:
    keep_last: 10
  filename_format: "backup_%s_%s.json.enc"
```

### Environment Variables

You can override configuration values using environment variables with the `PWBACKUP_` prefix:

```bash
export PWBACKUP_BACKUP_ENCRYPTION_ENABLED=true
export PWBACKUP_BACKUP_RETENTION_KEEPLAST=5
```

## Usage

### Commands

#### `credstash init`

Initialize configuration with an interactive setup wizard.

```bash
credstash init
```

#### `credstash backup`

Backup password manager vaults.

```bash
# Backup all configured managers to all destinations
credstash backup

# Backup specific manager
credstash backup --manager bitwarden

# Backup to specific destination
credstash backup --destination gdrive

# FULL EXPORT with passwords (1Password only, SLOW but complete)
credstash backup --full-export

# Backup without encryption (not recommended)
credstash backup --no-encrypt

# Verbose output
credstash backup --verbose
```

**Options:**
- `-m, --manager`: Password manager to backup (bitwarden, 1password, all)
- `-d, --destination`: Destination to backup to (gdrive, usb, local, all)
- `-k, --encryption-key`: Path to encryption key file
- `--no-encrypt`: Skip encryption (not recommended)
- `--prompt-each`: Prompt for password for each manager (more secure, recommended)
- `--full-export`: Export with actual passwords (1Password only, slower) ⭐ **NEW**
- `-v, --verbose`: Verbose output

**Export Modes (1Password):**
- **Default (Fast)**: Metadata only - titles, usernames, URLs (no passwords)
- **`--full-export` (Slow)**: Complete export including passwords and all fields

**Security Modes:**
- **Default**: Asks for password once, uses same password for all managers
- **`--prompt-each`**: Asks for password for each manager separately (recommended for maximum security)

#### `credstash list`

List all backups from storage destinations.

```bash
# List all backups
credstash list

# List from specific destination
credstash list --destination gdrive
```

**Options:**
- `-d, --destination`: Destination to list from (gdrive, usb, local, all)

#### `credstash config`

Manage configuration.

```bash
# Show current configuration
credstash config show

# Validate configuration and test connections
credstash config validate
```

#### `credstash restore`

Restore and decrypt backups for manual import.

```bash
# Restore a backup (auto-search all locations)
credstash restore --file backup_bitwarden_20251004_143022.json.enc

# Restore from specific location
credstash restore --file backup_1password_20251004_143022.json.enc --source local

# Specify output location
credstash restore --file backup_bitwarden_20251004_143022.json.enc --output ~/Downloads/vault.json
```

**Options:**
- `-f, --file`: Backup file name to restore (required)
- `-s, --source`: Source to restore from (gdrive, usb, local) - auto-detects if not specified
- `-o, --output`: Output path for decrypted file (default: current directory)

**What it does:**
1. Downloads the encrypted `.enc` backup file
2. Decrypts it with your encryption password
3. Decompresses the data
4. Saves as readable JSON file

**Importing the restored backup:**

For **Bitwarden**:
1. Open Bitwarden web vault or desktop app
2. Go to Tools → Import Data
3. Select "Bitwarden (json)" as format
4. Upload the decrypted JSON file

For **1Password**:
1. The JSON contains your vault data in 1Password's export format
2. Use 1Password CLI or contact support for import assistance
3. Alternatively, manually recreate important items

**⚠️ Security Note**: Delete the decrypted JSON file immediately after importing!

### Example Workflow

```bash
# 1. Initialize
credstash init

# 2. Validate setup
credstash config validate

# 3. Create backup
credstash backup

# 4. List backups
credstash list

# 5. Backup specific manager to specific destination
credstash backup --manager bitwarden --destination usb

# 6. Restore a backup when needed
credstash restore --file backup_bitwarden_20251004_143022.json.enc
```

## Security Considerations

### Encryption

- **Algorithm**: AES-256-GCM (authenticated encryption)
- **Key Derivation**: PBKDF2 with 100,000 iterations
- **Random Salt**: New random salt for each backup
- **Random Nonce**: New random nonce for each encryption
- **Authentication**: GCM provides built-in authentication

### Password Handling

**Important**: The encryption password is **NEVER stored** anywhere. It's only held in memory during the backup operation.

**Two Security Modes:**

1. **Default (Convenience)**: Password asked once per backup session
   - ✅ Better UX - less typing
   - ✅ Same password for all backups
   - ⚠️ Password stays in process memory longer
   ```bash
   credstash backup  # Asks once
   ```

2. **`--prompt-each` (Maximum Security)**: Password asked for each manager
   - ✅ Password in memory for minimal time
   - ✅ Cleared after each manager backup
   - ✅ Better protection against memory dumps
   - ⚠️ More typing required
   ```bash
   credstash backup --prompt-each  # Asks for each manager
   ```

**Recommendation**: Use `--prompt-each` for maximum security, especially on shared systems or when paranoid about memory attacks.

### File Permissions

- Configuration files: `0600` (read/write for owner only)
- Backup files: `0600` (read/write for owner only)
- Configuration directory: `0700` (full access for owner only)

### Storage Backends

#### Local Storage (Fallback)
- **Always Available**: Works even when cloud/USB is unavailable
- **Fast**: No network latency or USB connection required
- **Secure**: Files stored with restrictive permissions (0600)
- **Default Location**: `~/.credstash/backups`
- **Use Case**: Reliable fallback when other storage is unavailable

#### Google Drive
- **Cloud Storage**: Remote backup for disaster recovery
- **OAuth2**: Secure authentication
- **Folder Support**: Organize backups in dedicated folders

#### USB Storage
- **Portable**: Physical backup on external drive
- **Offline**: Works without internet connection
- **Mount Detection**: Automatically detects if USB is connected

### Best Practices

1. **Use Strong Passwords**: Choose a strong encryption password
2. **Keep Passwords Secure**: Never share or write down encryption passwords
3. **Regular Backups**: Schedule regular backups (e.g., weekly)
4. **Test Restores**: Periodically test that backups can be decrypted
5. **Multiple Destinations**: Use at least two storage backends for redundancy (e.g., local + cloud)
6. **Secure Storage**: Keep Google Drive credentials secure
7. **Monitor Access**: Regularly check Google Drive access logs
8. **Enable Local Fallback**: Always enable local storage as a reliable fallback

### What's Protected

- ✅ Vault data encrypted at rest
- ✅ Encrypted backups in transit
- ✅ Configuration files have restrictive permissions
- ✅ No plaintext secrets in logs
- ✅ Secure key derivation

### What's Not Protected

- ⚠️ Encryption password (you must remember it)
- ⚠️ Process memory (during backup operation)
- ⚠️ Password manager CLI authentication tokens

## Troubleshooting

### Common Issues

#### "Configuration file not found"

Run `credstash init` to create a configuration file.

#### "Bitwarden CLI not found"

Install Bitwarden CLI:
```bash
# macOS
brew install bitwarden-cli

# Linux
snap install bw

# Windows
choco install bitwarden-cli
```

#### "1Password CLI not found"

Install 1Password CLI:
```bash
# macOS
brew install --cask 1password-cli

# Linux/Windows
# Download from https://developer.1password.com/docs/cli/get-started/
```

#### "Not authenticated"

**Bitwarden:**
```bash
bw login
bw unlock
export BW_SESSION="..."
```

**1Password:**
```bash
op signin
```

#### "Google Drive: credentials file not found"

1. Download OAuth2 credentials from Google Cloud Console
2. Save to `~/.credstash/gdrive-credentials.json`
3. Run `credstash backup` and complete OAuth2 flow

#### "USB drive not available"

Ensure:
1. USB drive is plugged in
2. USB drive is mounted
3. Mount path in config matches actual mount point
4. You have write permissions

#### "Failed to decrypt"

This usually means:
1. Wrong encryption password
2. Corrupted backup file
3. Backup was created with a different password

### Debug Mode

Run with verbose flag for detailed output:

```bash
credstash backup --verbose
```

### Getting Help

If you encounter issues:

1. Run `credstash config validate` to check your setup
2. Check logs for error messages
3. Verify password manager CLI is working directly
4. Test storage backends individually

## Architecture

### Project Structure

```
credstash/
├── cmd/                      # CLI commands
│   ├── root.go              # Root command
│   ├── init.go              # Initialize configuration
│   ├── backup.go            # Backup command
│   ├── list.go              # List backups
│   └── config.go            # Config management
├── internal/
│   ├── config/              # Configuration management
│   │   └── config.go
│   ├── managers/            # Password manager integrations
│   │   ├── manager.go       # Interface
│   │   ├── bitwarden.go     # Bitwarden implementation
│   │   └── onepassword.go   # 1Password implementation
│   ├── storage/             # Storage backends
│   │   ├── storage.go       # Interface
│   │   ├── googledrive.go   # Google Drive implementation
│   │   └── usb.go           # USB implementation
│   ├── crypto/              # Encryption utilities
│   │   └── encryption.go
│   └── logger/              # Logging utilities
│       └── logger.go
├── pkg/
│   └── utils/               # Shared utilities
│       └── utils.go
└── main.go                  # Entry point
```

### Technology Stack

- **Language**: Go 1.21+
- **CLI Framework**: Cobra
- **Configuration**: Viper
- **Encryption**: AES-256-GCM (Go standard library)
- **Google Drive**: Google Drive API v3
- **OAuth2**: golang.org/x/oauth2

### Backup File Format

Encrypted backup files follow this structure:

```
[Header: 16 bytes]
  - Magic: "PWBK" (4 bytes)
  - Version: 1 (2 bytes)
  - Algorithm: 1 for AES-256-GCM (2 bytes)
  - Reserved: (8 bytes)
[Salt: 32 bytes]
[Nonce: 12 bytes]
[Encrypted Data: variable]
[Auth Tag: 16 bytes (included in GCM ciphertext)]
```

## Future Enhancements

Features planned for future releases (documented but not implemented):

- **Restore Functionality**: Restore backups to password managers
- **Additional Password Managers**: LastPass, Dashlane, KeePass
- **Additional Storage Backends**: Dropbox, Amazon S3, Azure Blob Storage
- **Scheduled Backups**: Cron job integration
- **Backup Verification**: Checksum verification
- **Key Rotation**: Automatic encryption key rotation
- **Web UI**: Web interface for configuration and management
- **Backup Compression**: Advanced compression algorithms
- **Backup Deduplication**: Save space by deduplicating data
- **Cloud-to-Cloud Backup**: Direct backup without local storage

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Bitwarden for their excellent open-source password manager
- 1Password for their robust CLI tool
- Google for the Drive API
- The Go community for excellent libraries

## Support

- **Issues**: [GitHub Issues](https://github.com/harshalranjhani/credstash/issues)
- **Documentation**: This README and inline code documentation
- **Security Issues**: Please report security issues privately to the maintainer

---

**⚠️ Security Notice**: This tool handles sensitive data. Always:
- Keep your encryption passwords secure
- Use strong passwords
- Keep your system and password manager CLIs up to date
- Regularly test your backups
- Store backups in secure locations

**Made with ❤️ for password security**
