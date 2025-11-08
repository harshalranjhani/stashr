package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/database"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

var (
	emergencyOutput string
)

// emergencyCmd represents the emergency command
var emergencyCmd = &cobra.Command{
	Use:   "emergency-kit",
	Short: "Generate emergency access kit",
	Long: `Generate an emergency access kit PDF with recovery instructions.

This PDF contains:
- Configuration summary (sensitive data redacted)
- Storage locations and access instructions
- Restoration step-by-step guide
- Recent backup information
- Emergency recovery procedures

The kit does NOT contain:
- Encryption passwords
- API keys or credentials
- Actual backup data

Keep this document in a safe place for emergency recovery situations.`,
	Run: runEmergency,
}

func init() {
	rootCmd.AddCommand(emergencyCmd)

	emergencyCmd.Flags().StringVarP(&emergencyOutput, "output", "o", "", "Output path for PDF (default: emergency-kit-YYYYMMDD.pdf)")
}

func runEmergency(cmd *cobra.Command, args []string) {
	logger.Header("🚨 Emergency Access Kit Generator")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Determine output path
	if emergencyOutput == "" {
		timestamp := time.Now().Format("20060102")
		emergencyOutput = fmt.Sprintf("emergency-kit-%s.pdf", timestamp)
	}

	// Make it absolute
	if !filepath.IsAbs(emergencyOutput) {
		cwd, _ := os.Getwd()
		emergencyOutput = filepath.Join(cwd, emergencyOutput)
	}

	logger.Progress("Generating emergency access kit...")

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Title
	pdf.SetFont("Arial", "B", 24)
	pdf.SetTextColor(200, 0, 0)
	pdf.Cell(0, 15, "EMERGENCY ACCESS KIT")
	pdf.Ln(10)

	// Subtitle
	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 8, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(15)

	// Warning box
	pdf.SetFillColor(255, 245, 230)
	pdf.SetDrawColor(255, 165, 0)
	pdf.Rect(20, pdf.GetY(), 170, 25, "FD")
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(200, 100, 0)
	pdf.Cell(0, 5, "WARNING: Keep this document secure!")
	pdf.Ln(5)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.Cell(0, 5, "This document contains information about your backup configuration.")
	pdf.Ln(5)
	pdf.Cell(0, 5, "Do not share with unauthorized persons.")
	pdf.Ln(15)

	// Configuration Summary
	addSection(pdf, "1. Configuration Summary")
	pdf.SetFont("Arial", "", 10)

	// Password Managers
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(0, 6, "Password Managers:")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 10)

	if cfg.PasswordManagers.Bitwarden.Enabled {
		pdf.Cell(0, 5, fmt.Sprintf("  - Bitwarden: Enabled (Email: %s)", redactEmail(cfg.PasswordManagers.Bitwarden.Email)))
		pdf.Ln(5)
	}
	if cfg.PasswordManagers.OnePassword.Enabled {
		pdf.Cell(0, 5, fmt.Sprintf("  - 1Password: Enabled (Account: %s)", redactDomain(cfg.PasswordManagers.OnePassword.Account)))
		pdf.Ln(5)
	}
	pdf.Ln(5)

	// Storage Backends
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(0, 6, "Storage Backends:")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 10)

	if cfg.Storage.Local.Enabled {
		pdf.Cell(0, 5, fmt.Sprintf("  - Local: %s", cfg.Storage.Local.BackupPath))
		pdf.Ln(5)
	}
	if cfg.Storage.USB.Enabled {
		pdf.Cell(0, 5, fmt.Sprintf("  - USB: %s/%s", cfg.Storage.USB.MountPath, cfg.Storage.USB.BackupDir))
		pdf.Ln(5)
	}
	if cfg.Storage.GoogleDrive.Enabled {
		pdf.Cell(0, 5, "  - Google Drive: Enabled")
		pdf.Ln(5)
	}
	pdf.Ln(5)

	// Backup Settings
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(0, 6, "Backup Settings:")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("  - Encryption: %v (%s)", cfg.Backup.Encryption.Enabled, cfg.Backup.Encryption.Algorithm))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("  - Compression: %v", cfg.Backup.Compression))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("  - Retention: Keep last %d backups", cfg.Backup.Retention.KeepLast))
	pdf.Ln(10)

	// Recent Backups
	addSection(pdf, "2. Recent Backups")
	pdf.SetFont("Arial", "", 10)

	backups, err := database.ListBackups("", "", nil)
	if err == nil && len(backups) > 0 {
		// Show last 5 backups
		count := 5
		if len(backups) < count {
			count = len(backups)
		}

		for i := 0; i < count; i++ {
			backup := backups[i]
			pdf.SetFont("Arial", "B", 9)
			pdf.Cell(0, 5, fmt.Sprintf("Backup %d:", i+1))
			pdf.Ln(5)
			pdf.SetFont("Arial", "", 9)
			pdf.Cell(0, 4, fmt.Sprintf("  File: %s", truncatePDF(backup.Filename, 60)))
			pdf.Ln(4)
			pdf.Cell(0, 4, fmt.Sprintf("  Manager: %s", backup.Manager))
			pdf.Ln(4)
			pdf.Cell(0, 4, fmt.Sprintf("  Storage: %s", backup.StorageType))
			pdf.Ln(4)
			pdf.Cell(0, 4, fmt.Sprintf("  Size: %s", utils.FormatBytes(backup.Size)))
			pdf.Ln(4)
			pdf.Cell(0, 4, fmt.Sprintf("  Date: %s", backup.CreatedAt.Format("2006-01-02 15:04:05")))
			pdf.Ln(6)
		}
	} else {
		pdf.Cell(0, 5, "No recent backups found in database.")
		pdf.Ln(10)
	}

	// Restoration Guide
	pdf.AddPage()
	addSection(pdf, "3. Emergency Restoration Guide")
	pdf.SetFont("Arial", "", 10)

	steps := []string{
		"1. Ensure you have stashr CLI installed:",
		"   brew install harshalranjhani/tap/stashr",
		"   (or download from GitHub releases)",
		"",
		"2. Locate your backup files:",
		"   - Check local storage path (see section 1)",
		"   - Check USB drive if available",
		"   - Check Google Drive if configured",
		"",
		"3. List available backups:",
		"   stashr list",
		"",
		"4. Restore the backup you need:",
		"   stashr restore --file <backup-filename>",
		"   (You will be prompted for encryption password)",
		"",
		"5. Import restored data:",
		"   For Bitwarden:",
		"     - Open Bitwarden web vault or desktop app",
		"     - Go to Tools -> Import Data",
		"     - Select 'Bitwarden (json)' format",
		"     - Upload the decrypted JSON file",
		"",
		"   For 1Password:",
		"     - Use 1Password CLI to import",
		"     - Or contact 1Password support for assistance",
		"",
		"6. Delete decrypted file after import:",
		"   rm <decrypted-file>",
	}

	for _, step := range steps {
		if step == "" {
			pdf.Ln(3)
		} else {
			pdf.Cell(0, 4, step)
			pdf.Ln(4)
		}
	}

	// Important Notes
	pdf.AddPage()
	addSection(pdf, "4. Important Notes")
	pdf.SetFont("Arial", "", 10)

	notes := []string{
		"Encryption Password:",
		"  - You MUST remember your encryption password",
		"  - It is NOT stored anywhere by stashr",
		"  - Without it, backups cannot be decrypted",
		"  - Consider storing it in a secure password manager",
		"",
		"Google Drive Access:",
		"  - Requires credentials file from Google Cloud Console",
		"  - Location: " + cfg.Storage.GoogleDrive.CredentialsPath,
		"  - You may need to re-authenticate",
		"",
		"USB Drive:",
		"  - Must be mounted at the configured path",
		"  - Backup directory: " + cfg.Storage.USB.BackupDir,
		"",
		"Security Recommendations:",
		"  - Keep this document in a secure location",
		"  - Update it after significant configuration changes",
		"  - Test restoration periodically",
		"  - Maintain multiple backup destinations",
		"",
		"Getting Help:",
		"  - GitHub: https://github.com/harshalranjhani/stashr",
		"  - Issues: https://github.com/harshalranjhani/stashr/issues",
	}

	for _, note := range notes {
		if note == "" {
			pdf.Ln(3)
		} else {
			pdf.Cell(0, 4, note)
			pdf.Ln(4)
		}
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.Cell(0, 4, "Generated by stashr - Password Manager Backup Tool")
	pdf.Ln(4)
	pdf.Cell(0, 4, fmt.Sprintf("Document ID: %s", time.Now().Format("20060102-150405")))

	// Save PDF
	if err := pdf.OutputFileAndClose(emergencyOutput); err != nil {
		logger.Failure("Failed to generate PDF: %v", err)
		return
	}

	logger.Success("✓ Emergency access kit generated: %s", emergencyOutput)
	logger.Separator()
	logger.Warning("⚠️  IMPORTANT:")
	logger.Info("  - Store this document in a secure location")
	logger.Info("  - Do not share with unauthorized persons")
	logger.Info("  - Update periodically after configuration changes")
	logger.Info("  - Test your restoration process regularly")
}

func addSection(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(0, 0, 0)
	pdf.Cell(0, 10, title)
	pdf.Ln(8)
}

func redactEmail(email string) string {
	if email == "" {
		return "[not configured]"
	}
	// Keep first char and domain
	at := 0
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at > 0 {
		return email[:1] + "***" + email[at:]
	}
	return "***"
}

func redactDomain(domain string) string {
	if domain == "" {
		return "[not configured]"
	}
	return domain
}

func truncatePDF(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
