package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/harshalranjhani/stashr/internal/logger"
	"github.com/harshalranjhani/stashr/internal/version"
)

var (
	verbose bool
	cfgFile string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "stashr",
	Short: "Password Manager Backup Tool",
	Long: `
     _            _         
 ___| |_ __ _ ___| |__  _ _ 
/ __| __/ _' / __| '_ \| '_|
\__ \ || (_| \__ \ | | | |  
|___/\__\__,_|___/_| |_|_|  

🔐 Password Manager Backup Tool

A secure CLI tool for backing up password manager vaults to multiple destinations.
Supports Bitwarden, 1Password, Google Drive, USB, and local storage.`,
	Version: version.GetFullVersion(),
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.stashr/config.yaml)")
}

func initConfig() {
	// This will be called before each command execution
}
