package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/giwty/switch-library-manager/pkg/logging"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configDir  string
	verbose    bool
	quiet      bool
	outputMode string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "slm",
	Short: "Switch Library Manager - Manage your Nintendo Switch game library",
	Long: `Switch Library Manager (SLM) is a CLI tool for managing Nintendo Switch game libraries.
It can scan, organize, and check for missing updates and DLC in your game collection.

Features:
- Scan game libraries (NSP/NSZ/XCI files)
- Organize games into folders with proper naming
- Check for missing updates and DLC
- Multi-language title database support
- Dry-run mode for safe operations`,
	Version: settings.SLM_VERSION,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "config directory (default: $HOME/.config/slm)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output")
	rootCmd.PersistentFlags().StringVar(&outputMode, "output-mode", "auto", "output mode (auto, simple, rich, json, csv)")

	// Mark conflicting flags as mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	setupLogger()
}

func setupLogger() {
	// Setup log directory
	logDir := filepath.Join(getConfigDir(), "logs")

	// Create logger with rotation support
	logger, err := logging.SetupLogger(logDir, verbose, quiet)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// Replace global logger
	zap.ReplaceGlobals(logger)

	// Log startup information
	logger.Info("SLM started",
		zap.String("version", settings.SLM_VERSION),
		zap.String("config_dir", getConfigDir()),
		zap.String("output_mode", getOutputMode()),
		zap.Bool("verbose", verbose),
		zap.Bool("quiet", quiet))
}

// getConfigDir returns the configuration directory
func getConfigDir() string {
	if configDir != "" {
		return configDir
	}

	// Try environment variable
	if envDir := os.Getenv("SLM_CONFIG_DIR"); envDir != "" {
		return envDir
	}

	// Default to XDG Base Directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	return filepath.Join(homeDir, ".config", "slm")
}

// getOutputMode returns the current output mode
func getOutputMode() string {
	// Try environment variable first
	if envMode := os.Getenv("SLM_OUTPUT_MODE"); envMode != "" {
		return envMode
	}
	return outputMode
}