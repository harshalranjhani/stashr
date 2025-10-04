package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/config"
	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/internal/storage"
	"github.com/harshalranjhani/stashr/pkg/utils"
)

var listDestination string

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups",
	Long: `List all backups from configured storage destinations.

Shows backup files with their timestamp, size, and location.`,
	Run: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listDestination, "destination", "d", "all", "Destination to list from (gdrive, usb, local, all)")
}

func runList(cmd *cobra.Command, args []string) {
	logger.Header("📋 Backup List")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.PrintError(err)
		return
	}

	// Get storage backends
	storageBackends := getStorageBackendsForList(cfg)
	if len(storageBackends) == 0 {
		logger.Failure("No storage backends enabled or selected")
		return
	}

	// List backups from each backend
	allBackups := make(map[string][]storage.BackupFile)
	totalBackups := 0

	for _, backend := range storageBackends {
		logger.Separator()
		logger.Progress("Listing backups from %s...", backend.Name())

		available, err := backend.IsAvailable()
		if err != nil {
			logger.Warning("⚠ %s: %v", backend.Name(), err)
			continue
		}
		if !available {
			logger.Warning("⚠ %s is not available", backend.Name())
			continue
		}

		backups, err := backend.List()
		if err != nil {
			logger.Warning("⚠ Failed to list backups: %v", err)
			continue
		}

		allBackups[backend.Name()] = backups
		totalBackups += len(backups)
		logger.Success("✓ Found %d backup(s)", len(backups))
	}

	// Display backups
	logger.Separator()
	if totalBackups == 0 {
		logger.Info("No backups found")
		return
	}

	logger.Info("Total backups: %d", totalBackups)
	logger.Separator()

	for backendName, backups := range allBackups {
		if len(backups) == 0 {
			continue
		}

		fmt.Printf("\n%s:\n", backendName)
		fmt.Println(string(make([]rune, len(backendName)+1)))

		// Sort backups by modification time (newest first)
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].ModifiedTime.After(backups[j].ModifiedTime)
		})

		// Display backups in table format
		fmt.Printf("%-50s %-20s %-12s %-20s\n", "Name", "Modified", "Size", "Age")
		fmt.Println(string(make([]rune, 102)))

		for _, backup := range backups {
			age := formatAge(time.Since(backup.ModifiedTime))
			modTime := backup.ModifiedTime.Format("2006-01-02 15:04:05")
			size := utils.FormatBytes(backup.Size)

			fmt.Printf("%-50s %-20s %-12s %-20s\n",
				truncate(backup.Name, 50),
				modTime,
				size,
				age,
			)
		}
	}

	logger.Separator()
}

func getStorageBackendsForList(cfg *config.Config) []storage.Storage {
	var backends []storage.Storage

	if listDestination == "all" || listDestination == "gdrive" {
		if cfg.Storage.GoogleDrive.Enabled {
			backends = append(backends, storage.NewGoogleDrive(
				cfg.Storage.GoogleDrive.CredentialsPath,
				cfg.Storage.GoogleDrive.FolderID,
			))
		}
	}

	if listDestination == "all" || listDestination == "usb" {
		if cfg.Storage.USB.Enabled {
			backends = append(backends, storage.NewUSB(
				cfg.Storage.USB.MountPath,
				cfg.Storage.USB.BackupDir,
			))
		}
	}

	if listDestination == "all" || listDestination == "local" {
		if cfg.Storage.Local.Enabled {
			backends = append(backends, storage.NewLocal(
				cfg.Storage.Local.BackupPath,
			))
		}
	}

	return backends
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	if days < 7 {
		return fmt.Sprintf("%d days ago", days)
	}
	weeks := days / 7
	if weeks == 1 {
		return "1 week ago"
	}
	if weeks < 4 {
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	months := days / 30
	if months == 1 {
		return "1 month ago"
	}
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	years := days / 365
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
