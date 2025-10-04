package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/credstash/internal/logger"
)

var (
	verbose bool
	cfgFile string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "credstash",
	Short: "Password Manager Backup Tool",
	Long: `🔐 Password Manager Backup Tool

A secure CLI tool for backing up password manager vaults to multiple destinations.
Supports Bitwarden, 1Password, Google Drive, USB, and local storage.`,
	Version: "1.0.0",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set verbose mode
		if verbose {
			logger.SetVerbose(true)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.PrintError(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.credstash/config.yaml)")
}

func initConfig() {
	// This will be called before each command execution
}
