package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/database"
	"github.com/harshalranjhani/stashr/internal/logger"
)

var (
	tagFilename string
	tagValue    string
	noteValue   string
)

// tagCmd represents the tag command
var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage backup tags and annotations",
	Long: `Manage tags and notes for your backups.

Tags help you organize and find backups easily. You can add multiple tags
to a backup and search for backups by tag.

Examples:
  # Add a tag to a backup
  stashr tag add --file backup.enc --tag important

  # Remove a tag from a backup
  stashr tag remove --file backup.enc --tag important

  # List all backups with a specific tag
  stashr tag list --tag important

  # Add a note to a backup
  stashr note add --file backup.enc --note "Before password reset"

  # View notes for a backup
  stashr note show --file backup.enc`,
}

// tagAddCmd represents the tag add command
var tagAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a tag to a backup",
	Run:   runTagAdd,
}

// tagRemoveCmd represents the tag remove command
var tagRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a tag from a backup",
	Run:   runTagRemove,
}

// tagListCmd represents the tag list command
var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups by tag",
	Run:   runTagList,
}

// tagShowAllCmd represents the tag show-all command
var tagShowAllCmd = &cobra.Command{
	Use:   "show-all",
	Short: "Show all unique tags",
	Run:   runTagShowAll,
}

// noteCmd represents the note command
var noteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage backup notes",
	Long:  `Add or view notes for your backups.`,
}

// noteAddCmd represents the note add command
var noteAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update note for a backup",
	Run:   runNoteAdd,
}

// noteShowCmd represents the note show command
var noteShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show note for a backup",
	Run:   runNoteShow,
}

func init() {
	rootCmd.AddCommand(tagCmd)
	rootCmd.AddCommand(noteCmd)

	// Tag subcommands
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagShowAllCmd)

	// Note subcommands
	noteCmd.AddCommand(noteAddCmd)
	noteCmd.AddCommand(noteShowCmd)

	// Tag add flags
	tagAddCmd.Flags().StringVarP(&tagFilename, "file", "f", "", "Backup filename (required)")
	tagAddCmd.Flags().StringVarP(&tagValue, "tag", "t", "", "Tag to add (required)")
	tagAddCmd.MarkFlagRequired("file")
	tagAddCmd.MarkFlagRequired("tag")

	// Tag remove flags
	tagRemoveCmd.Flags().StringVarP(&tagFilename, "file", "f", "", "Backup filename (required)")
	tagRemoveCmd.Flags().StringVarP(&tagValue, "tag", "t", "", "Tag to remove (required)")
	tagRemoveCmd.MarkFlagRequired("file")
	tagRemoveCmd.MarkFlagRequired("tag")

	// Tag list flags
	tagListCmd.Flags().StringVarP(&tagValue, "tag", "t", "", "Tag to filter by (required)")
	tagListCmd.MarkFlagRequired("tag")

	// Note add flags
	noteAddCmd.Flags().StringVarP(&tagFilename, "file", "f", "", "Backup filename (required)")
	noteAddCmd.Flags().StringVarP(&noteValue, "note", "n", "", "Note text (required)")
	noteAddCmd.MarkFlagRequired("file")
	noteAddCmd.MarkFlagRequired("note")

	// Note show flags
	noteShowCmd.Flags().StringVarP(&tagFilename, "file", "f", "", "Backup filename (required)")
	noteShowCmd.MarkFlagRequired("file")
}

func runTagAdd(cmd *cobra.Command, args []string) {
	logger.Header("🏷️  Add Tag")

	// Check if backup exists in database
	backup, err := database.GetBackup(tagFilename)
	if err != nil {
		logger.PrintError(err)
		return
	}
	if backup == nil {
		logger.Failure("Backup not found in database: %s", tagFilename)
		logger.Info("Note: Only backups created after database feature was added are tracked.")
		logger.Info("You can run a new backup to start tracking.")
		return
	}

	// Add tag
	if err := database.AddTag(tagFilename, tagValue); err != nil {
		logger.PrintError(err)
		return
	}

	logger.Success("✓ Tag '%s' added to backup: %s", tagValue, tagFilename)
}

func runTagRemove(cmd *cobra.Command, args []string) {
	logger.Header("🏷️  Remove Tag")

	// Remove tag
	if err := database.RemoveTag(tagFilename, tagValue); err != nil {
		logger.PrintError(err)
		return
	}

	logger.Success("✓ Tag '%s' removed from backup: %s", tagValue, tagFilename)
}

func runTagList(cmd *cobra.Command, args []string) {
	logger.Header("🏷️  List Backups by Tag")

	// Get backups with this tag
	backups, err := database.GetBackupsByTag(tagValue)
	if err != nil {
		logger.PrintError(err)
		return
	}

	if len(backups) == 0 {
		logger.Info("No backups found with tag: %s", tagValue)
		return
	}

	logger.Success("Found %d backup(s) with tag '%s':", len(backups), tagValue)
	logger.Separator()

	for _, filename := range backups {
		fmt.Printf("  • %s\n", filename)
	}
}

func runTagShowAll(cmd *cobra.Command, args []string) {
	logger.Header("🏷️  All Tags")

	// Get all tags
	tags, err := database.ListAllTags()
	if err != nil {
		logger.PrintError(err)
		return
	}

	if len(tags) == 0 {
		logger.Info("No tags found")
		return
	}

	logger.Success("Found %d unique tag(s):", len(tags))
	logger.Separator()

	for _, tag := range tags {
		// Get count of backups with this tag
		backups, _ := database.GetBackupsByTag(tag)
		fmt.Printf("  • %-20s (%d backup%s)\n", tag, len(backups), pluralize(len(backups)))
	}
}

func runNoteAdd(cmd *cobra.Command, args []string) {
	logger.Header("📝 Add Note")

	// Check if backup exists in database
	backup, err := database.GetBackup(tagFilename)
	if err != nil {
		logger.PrintError(err)
		return
	}
	if backup == nil {
		logger.Failure("Backup not found in database: %s", tagFilename)
		logger.Info("Note: Only backups created after database feature was added are tracked.")
		logger.Info("You can run a new backup to start tracking.")
		return
	}

	// Add or update note
	if err := database.UpdateBackupNotes(tagFilename, noteValue); err != nil {
		logger.PrintError(err)
		return
	}

	logger.Success("✓ Note added to backup: %s", tagFilename)
}

func runNoteShow(cmd *cobra.Command, args []string) {
	logger.Header("📝 Show Note")

	// Get backup
	backup, err := database.GetBackup(tagFilename)
	if err != nil {
		logger.PrintError(err)
		return
	}
	if backup == nil {
		logger.Failure("Backup not found in database: %s", tagFilename)
		return
	}

	// Display backup info
	logger.Info("Backup: %s", backup.Filename)
	logger.Info("Manager: %s", backup.Manager)
	logger.Info("Storage: %s", backup.StorageType)
	logger.Separator()

	// Display tags if any
	if len(backup.Tags) > 0 {
		logger.Info("Tags: %v", backup.Tags)
	}

	// Display note
	if backup.Notes != nil && *backup.Notes != "" {
		logger.Info("Note:")
		fmt.Printf("\n%s\n\n", *backup.Notes)
	} else {
		logger.Info("No note found for this backup")
	}
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
