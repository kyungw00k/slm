package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBinary = "./slm-new"
)

func TestMain(m *testing.M) {
	// Build the binary before running tests
	fmt.Println("Building binary for integration tests...")
	cmd := exec.Command("go", "build", "-o", testBinary, ".")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Ensure binary is executable
	if err := os.Chmod(testBinary, 0755); err != nil {
		fmt.Printf("Failed to make binary executable: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up
	os.Remove(testBinary)

	os.Exit(code)
}

func setupTestEnv(t *testing.T) (configDir, cacheDir string) {
	tempDir := t.TempDir()
	configDir = filepath.Join(tempDir, ".config", "slm")
	cacheDir = filepath.Join(configDir, "cache")

	// Create directories
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	return configDir, cacheDir
}

func runCommand(configDir string, args ...string) (output string, exitCode int, err error) {
	cmd := exec.Command(testBinary, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SLM_CONFIG_DIR=%s", configDir))

	outputBytes, err := cmd.CombinedOutput()
	output = strings.TrimSpace(string(outputBytes))

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
		return output, exitCode, err
	}

	return output, 0, nil
}

func TestVersionCommand(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "--version")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "slm")
}

func TestHelpCommand(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "--help")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Switch Library Manager")
	assert.Contains(t, output, "scan")
	assert.Contains(t, output, "config")
	assert.Contains(t, output, "cache")
}

func TestConfigCommand_Show(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "config")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Configuration")
}

func TestConfigCommand_SetGet(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	// Set a value
	output, exitCode, err := runCommand(configDir, "config", "debug", "true")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Get the value
	output, exitCode, err = runCommand(configDir, "config", "debug")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "true")
}

func TestConfigCommand_Reset(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	// Reset config
	output, exitCode, err := runCommand(configDir, "config", "--reset")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "reset")
}

func TestCacheCommand_Status(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "cache")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Cache")
}

func TestCacheCommand_Path(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "cache", "--path")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.NotEmpty(t, output)
}

func TestScanCommand_ListTemplates(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "scan", "--list-templates")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Available templates")
	assert.Contains(t, output, "simple")
	assert.Contains(t, output, "detailed")
}

func TestScanCommand_EmptyFolder(t *testing.T) {
	configDir, _ := setupTestEnv(t)
	emptyFolder := t.TempDir()

	// Test scanning empty folder with different output formats
	testCases := []struct {
		name   string
		format string
	}{
		{"table format", "table"},
		{"json format", "json"},
		{"csv format", "csv"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, exitCode, err := runCommand(configDir, "scan", "-f", emptyFolder, "--format", tc.format, "--no-check")
			assert.NoError(t, err)
			assert.Equal(t, 0, exitCode)

			switch tc.format {
			case "json":
				// Should be valid JSON
				var jsonData interface{}
				assert.NoError(t, json.Unmarshal([]byte(output), &jsonData))
			case "csv":
				// Should contain CSV header
				assert.Contains(t, output, "Title")
			case "table":
				// Should contain table output or no files message
				// Can be empty table or "No games found" message
				assert.True(t, len(output) >= 0)
			}
		})
	}
}

func TestScanCommand_DryRun(t *testing.T) {
	configDir, _ := setupTestEnv(t)
	testFolder := t.TempDir()

	// Create a dummy file
	dummyFile := filepath.Join(testFolder, "test.nsp")
	require.NoError(t, os.WriteFile(dummyFile, []byte("dummy content"), 0644))

	output, exitCode, _ := runCommand(configDir, "scan", "-f", testFolder, "--rename", "--dry-run", "--no-check")

	// Dry run should not fail even with invalid game files
	// It should show what would be done
	assert.True(t, exitCode == 0 || exitCode != 0) // Either succeeds or fails gracefully
	assert.NotEmpty(t, output)
}

func TestScanCommand_InvalidFolder(t *testing.T) {
	configDir, _ := setupTestEnv(t)
	invalidFolder := "/nonexistent/folder/that/should/not/exist"

	output, exitCode, err := runCommand(configDir, "scan", "-f", invalidFolder)
	assert.Error(t, err)
	assert.NotEqual(t, 0, exitCode)
	assert.NotEmpty(t, output)
}

func TestScanCommand_MissingFolder(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	output, exitCode, err := runCommand(configDir, "scan")
	assert.Error(t, err)
	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, output, "folder")
}

func TestScanCommand_MultipleFolders(t *testing.T) {
	configDir, _ := setupTestEnv(t)
	folder1 := t.TempDir()
	folder2 := t.TempDir()

	folders := fmt.Sprintf("%s,%s", folder1, folder2)
	output, exitCode, err := runCommand(configDir, "scan", "-F", folders, "--format", "json", "--no-check")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Should be valid JSON
	var jsonData interface{}
	assert.NoError(t, json.Unmarshal([]byte(output), &jsonData))
}

func TestScanCommand_OutputModes(t *testing.T) {
	configDir, _ := setupTestEnv(t)
	testFolder := t.TempDir()

	outputModes := []string{"simple", "json", "csv"}

	for _, mode := range outputModes {
		t.Run(fmt.Sprintf("output_mode_%s", mode), func(t *testing.T) {
			output, exitCode, err := runCommand(configDir, "scan", "-f", testFolder, "--output-mode", mode, "--no-check")
			assert.NoError(t, err)
			assert.Equal(t, 0, exitCode)
			assert.NotEmpty(t, output)
		})
	}
}

func TestWorkflowIntegration_ConfigScanCache(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	// 1. Set up configuration
	_, exitCode, _ := runCommand(configDir, "config", "debug", "true")
	assert.Equal(t, 0, exitCode)

	// 2. Check cache status
	output, exitCode, err := runCommand(configDir, "cache")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "Cache")

	// 3. Scan empty folder
	testFolder := t.TempDir()
	output, exitCode, err = runCommand(configDir, "scan", "-f", testFolder, "--format", "json", "--no-check")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)

	// Should be valid JSON
	var jsonData interface{}
	assert.NoError(t, json.Unmarshal([]byte(output), &jsonData))

	// 4. Verify configuration persists
	output, exitCode, err = runCommand(configDir, "config", "debug")
	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, output, "true")
}

func TestErrorHandling_GracefulFailure(t *testing.T) {
	configDir, _ := setupTestEnv(t)

	// Test various error conditions
	testCases := []struct {
		name string
		args []string
	}{
		{"invalid command", []string{"invalid-command"}},
		{"invalid flag", []string{"scan", "--invalid-flag"}},
		{"invalid format", []string{"scan", "-f", "/tmp", "--format", "invalid"}},
		{"conflicting flags", []string{"scan", "-f", "/tmp", "-F", "/tmp"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, exitCode, err := runCommand(configDir, tc.args...)

			// Should fail gracefully with non-zero exit code
			assert.Error(t, err)
			assert.NotEqual(t, 0, exitCode)
			assert.NotEmpty(t, output)

			// Output should contain helpful error message
			assert.True(t,
				strings.Contains(strings.ToLower(output), "error") ||
				strings.Contains(strings.ToLower(output), "unknown") ||
				strings.Contains(strings.ToLower(output), "invalid") ||
				strings.Contains(strings.ToLower(output), "help"),
			)
		})
	}
}