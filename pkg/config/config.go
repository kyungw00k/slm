package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giwty/switch-library-manager/settings"
)

// Manager handles configuration loading and saving
type Manager struct {
	configDir  string
	configPath string
	settings   *settings.AppSettings
}

// NewManager creates a new config manager
func NewManager(configDir string) *Manager {
	return &Manager{
		configDir:  configDir,
		configPath: filepath.Join(configDir, "config.json"),
	}
}

// Load loads configuration from file or creates default
func (m *Manager) Load() (*settings.AppSettings, error) {
	if m.settings != nil {
		return m.settings, nil
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %v", err)
	}

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// Create default config
		m.settings = m.createDefaultConfig()
		if err := m.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %v", err)
		}
		return m.settings, nil
	}

	// Load existing config
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config settings.AppSettings
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	m.settings = &config
	return m.settings, nil
}

// Save saves current configuration to file
func (m *Manager) Save() error {
	if m.settings == nil {
		return fmt.Errorf("no settings loaded")
	}

	data, err := json.MarshalIndent(m.settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// GetConfigDir returns the configuration directory
func (m *Manager) GetConfigDir() string {
	return m.configDir
}

// GetCacheDir returns the cache directory
func (m *Manager) GetCacheDir() string {
	return filepath.Join(m.configDir, "cache")
}

// GetTitleDBDir returns the title database directory
func (m *Manager) GetTitleDBDir() string {
	return filepath.Join(m.GetCacheDir(), "titledb")
}

// GetLocalDBDir returns the local database directory
func (m *Manager) GetLocalDBDir() string {
	return filepath.Join(m.GetCacheDir(), "db")
}

// GetLogsDir returns the logs directory
func (m *Manager) GetLogsDir() string {
	return filepath.Join(m.configDir, "logs")
}

// MigrateFromOldSettings migrates from existing settings.json
func (m *Manager) MigrateFromOldSettings(oldSettingsPath string) error {
	// Read old settings
	oldData, err := os.ReadFile(oldSettingsPath)
	if err != nil {
		return fmt.Errorf("failed to read old settings: %v", err)
	}

	var oldSettings settings.AppSettings
	if err := json.Unmarshal(oldData, &oldSettings); err != nil {
		return fmt.Errorf("failed to parse old settings: %v", err)
	}

	// Force GUI to false for CLI
	oldSettings.GUI = false

	m.settings = &oldSettings
	return m.Save()
}

func (m *Manager) createDefaultConfig() *settings.AppSettings {
	return &settings.AppSettings{
		VersionsEtag:           "",
		Prodkeys:               "",
		Folder:                 "",
		ScanFolders:            []string{},
		GUI:                    false, // Always false for new CLI
		Debug:                  false,
		CheckForMissingUpdates: true,
		CheckForMissingDLC:     true,
		ScanRecursively:        true,
		GuiPagingSize:          100,
		IgnoreDLCTitleIds:      []string{},
		LocalePriority:         []string{"KR.ko", "JP.ja", "US.en"},
		TitleDBUrls:            map[string]string{},
		TitlesETags:            map[string]string{
			"KR.ko": "",
			"US.en": "",
			"JP.ja": "",
		},
		OrganizeOptions: settings.OrganizeOptions{
			RenameFiles:         false,
			CreateFolderPerGame: false,
			FolderNameTemplate:  "{TITLE_NAME}",
			FileNameTemplate:    "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]",
			DeleteEmptyFolders:  false,
			SwitchSafeFileNames: true,
			DeleteOldUpdateFiles: false,
			DryRun:              false,
		},
	}
}