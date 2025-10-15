package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	configDir := "/tmp/test-config"
	manager := NewManager(configDir)

	assert.Equal(t, configDir, manager.configDir)
	assert.Equal(t, filepath.Join(configDir, "config.json"), manager.configPath)
}

func TestManager_LoadDefaultConfig(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Load config (should create default)
	config, err := manager.Load()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify default values
	assert.False(t, config.GUI)
	assert.True(t, config.CheckForMissingUpdates)
	assert.True(t, config.CheckForMissingDLC)
	assert.True(t, config.ScanRecursively)
	assert.Equal(t, []string{"KR.ko", "JP.ja", "US.en"}, config.LocalePriority)

	// Verify config file was created
	configPath := filepath.Join(tempDir, "config.json")
	assert.FileExists(t, configPath)
}

func TestManager_SaveAndLoad(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Load default config
	config, err := manager.Load()
	require.NoError(t, err)

	// Modify config
	config.Prodkeys = "/test/path/prod.keys"
	config.Debug = true
	config.LocalePriority = []string{"US.en", "JP.ja"}

	// Save config
	err = manager.Save()
	require.NoError(t, err)

	// Create new manager and load config
	manager2 := NewManager(tempDir)
	config2, err := manager2.Load()
	require.NoError(t, err)

	// Verify loaded config matches saved config
	assert.Equal(t, "/test/path/prod.keys", config2.Prodkeys)
	assert.True(t, config2.Debug)
	assert.Equal(t, []string{"US.en", "JP.ja"}, config2.LocalePriority)
}

func TestManager_MigrateFromOldSettings(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Create old settings file
	oldSettingsPath := filepath.Join(tempDir, "old_settings.json")
	// Create old settings file content
	err := os.WriteFile(oldSettingsPath, []byte(`{
		"prod_keys": "/old/path/prod.keys",
		"gui": true,
		"debug": true,
		"check_for_missing_updates": false,
		"locale_priority": ["JP.ja", "US.en"]
	}`), 0644)
	require.NoError(t, err)

	// Migrate settings
	err = manager.MigrateFromOldSettings(oldSettingsPath)
	require.NoError(t, err)

	// Load migrated settings
	config, err := manager.Load()
	require.NoError(t, err)

	// Verify migration
	assert.Equal(t, "/old/path/prod.keys", config.Prodkeys)
	assert.False(t, config.GUI) // Should be forced to false
	assert.True(t, config.Debug)
	assert.False(t, config.CheckForMissingUpdates)
	assert.Equal(t, []string{"JP.ja", "US.en"}, config.LocalePriority)
}

func TestManager_GetDirectories(t *testing.T) {
	configDir := "/test/config"
	manager := NewManager(configDir)

	assert.Equal(t, configDir, manager.GetConfigDir())
	assert.Equal(t, filepath.Join(configDir, "cache"), manager.GetCacheDir())
	assert.Equal(t, filepath.Join(configDir, "cache", "titledb"), manager.GetTitleDBDir())
	assert.Equal(t, filepath.Join(configDir, "cache", "db"), manager.GetLocalDBDir())
	assert.Equal(t, filepath.Join(configDir, "logs"), manager.GetLogsDir())
}

func TestManager_LoadInvalidConfig(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Create invalid config file
	configPath := filepath.Join(tempDir, "config.json")
	err := os.WriteFile(configPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Try to load invalid config
	_, err = manager.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestManager_SaveWithoutLoad(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Try to save without loading first
	err := manager.Save()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no settings loaded")
}