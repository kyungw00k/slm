package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/giwty/switch-library-manager/pkg/config"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/spf13/cobra"
)

var (
	showPath   bool
	migrate    bool
	resetConfig bool
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "Manage configuration settings",
	Long: `Manage SLM configuration settings.

Usage patterns:
  slm config                    # Show all configuration
  slm config <key>              # Get specific value
  slm config <key> <value>      # Set value
  slm config --path             # Show config file path
  slm config --migrate          # Migrate from settings.json
  slm config --reset            # Reset to default configuration

Examples:
  slm config
  slm config prod_keys
  slm config prod_keys "/path/to/prod.keys"
  slm config locale_priority "US.en,JP.ja,KR.ko"
  slm config --migrate
  slm config --reset`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().BoolVar(&showPath, "path", false, "show config file path")
	configCmd.Flags().BoolVar(&migrate, "migrate", false, "migrate from existing settings.json")
	configCmd.Flags().BoolVar(&resetConfig, "reset", false, "reset to default configuration")
}

func runConfig(cmd *cobra.Command, args []string) error {
	configMgr := config.NewManager(getConfigDir())

	// Handle --path flag
	if showPath {
		fmt.Println(filepath.Join(configMgr.GetConfigDir(), "config.json"))
		return nil
	}

	// Handle --migrate flag
	if migrate {
		return migrateSettings(configMgr)
	}

	// Handle --reset flag
	if resetConfig {
		return resetSettings(configMgr)
	}

	switch len(args) {
	case 0:
		// Show all configuration
		return showConfig(configMgr)
	case 1:
		// Get specific value
		return getConfigValue(configMgr, args[0])
	case 2:
		// Set value
		return setConfigValue(configMgr, args[0], args[1])
	default:
		return fmt.Errorf("too many arguments")
	}
}

func showConfig(configMgr *config.Manager) error {
	settings, err := configMgr.Load()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format config: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func getConfigValue(configMgr *config.Manager, key string) error {
	settings, err := configMgr.Load()
	if err != nil {
		return err
	}

	value, err := getValueByPath(settings, key)
	if err != nil {
		return fmt.Errorf("key not found: %s", key)
	}

	switch v := value.(type) {
	case string:
		fmt.Println(v)
	case []string:
		fmt.Println(strings.Join(v, ","))
	case bool:
		fmt.Println(v)
	default:
		data, _ := json.Marshal(v)
		fmt.Println(string(data))
	}

	return nil
}

func setConfigValue(configMgr *config.Manager, key, value string) error {
	settings, err := configMgr.Load()
	if err != nil {
		return err
	}

	// Parse value based on key type
	if err := setValueByPath(settings, key, value); err != nil {
		return fmt.Errorf("failed to set value: %v", err)
	}

	// Save config
	if err := configMgr.Save(); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}


func migrateSettings(configMgr *config.Manager) error {
	// Look for settings.json in current directory or executable directory
	settingsPath := ""
	candidates := []string{"settings.json", filepath.Join("..", "settings.json")}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			settingsPath = candidate
			break
		}
	}

	if settingsPath == "" {
		return fmt.Errorf("no settings.json found to migrate")
	}

	// Use config manager to migrate
	if err := configMgr.MigrateFromOldSettings(settingsPath); err != nil {
		return fmt.Errorf("failed to migrate settings: %v", err)
	}

	configPath := filepath.Join(configMgr.GetConfigDir(), "config.json")
	fmt.Printf("Successfully migrated settings from %s to %s\n", settingsPath, configPath)
	return nil
}

func resetSettings(configMgr *config.Manager) error {
	// Delete existing config file first
	configPath := filepath.Join(configMgr.GetConfigDir(), "config.json")
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("failed to remove existing config: %v", err)
		}
	}

	// Load will automatically create a new default config
	_, err := configMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to create default config: %v", err)
	}

	fmt.Printf("Reset configuration to defaults: %s\n", configPath)
	return nil
}

// Helper functions for nested key access
func getValueByPath(config *settings.AppSettings, key string) (interface{}, error) {
	switch key {
	case "prod_keys":
		return config.Prodkeys, nil
	case "folder":
		return config.Folder, nil
	case "scan_folders":
		return config.ScanFolders, nil
	case "debug":
		return config.Debug, nil
	case "check_for_missing_updates":
		return config.CheckForMissingUpdates, nil
	case "check_for_missing_dlc":
		return config.CheckForMissingDLC, nil
	case "scan_recursively":
		return config.ScanRecursively, nil
	case "locale_priority":
		return config.LocalePriority, nil
	case "ignore_dlc_title_ids":
		return config.IgnoreDLCTitleIds, nil
	case "organize_options.rename_files":
		return config.OrganizeOptions.RenameFiles, nil
	case "organize_options.create_folder_per_game":
		return config.OrganizeOptions.CreateFolderPerGame, nil
	case "organize_options.dry_run":
		return config.OrganizeOptions.DryRun, nil
	default:
		return nil, fmt.Errorf("unknown key: %s", key)
	}
}

func setValueByPath(config *settings.AppSettings, key, value string) error {
	switch key {
	case "prod_keys":
		config.Prodkeys = value
	case "folder":
		config.Folder = value
	case "scan_folders":
		config.ScanFolders = strings.Split(value, ",")
		for i := range config.ScanFolders {
			config.ScanFolders[i] = strings.TrimSpace(config.ScanFolders[i])
		}
	case "debug":
		config.Debug = value == "true"
	case "check_for_missing_updates":
		config.CheckForMissingUpdates = value == "true"
	case "check_for_missing_dlc":
		config.CheckForMissingDLC = value == "true"
	case "scan_recursively":
		config.ScanRecursively = value == "true"
	case "locale_priority":
		config.LocalePriority = strings.Split(value, ",")
		for i := range config.LocalePriority {
			config.LocalePriority[i] = strings.TrimSpace(config.LocalePriority[i])
		}
	case "ignore_dlc_title_ids":
		config.IgnoreDLCTitleIds = strings.Split(value, ",")
		for i := range config.IgnoreDLCTitleIds {
			config.IgnoreDLCTitleIds[i] = strings.TrimSpace(config.IgnoreDLCTitleIds[i])
		}
	case "organize_options.rename_files":
		config.OrganizeOptions.RenameFiles = value == "true"
	case "organize_options.create_folder_per_game":
		config.OrganizeOptions.CreateFolderPerGame = value == "true"
	case "organize_options.dry_run":
		config.OrganizeOptions.DryRun = value == "true"
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
	return nil
}