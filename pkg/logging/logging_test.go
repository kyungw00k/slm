package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupLogger(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := SetupLogger(tempDir, false, false)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test that logger works
	logger.Info("test message")
	logger.Error("test error")

	// Verify log file was created
	logFile := filepath.Join(tempDir, "slm.log")
	assert.FileExists(t, logFile)
}

func TestSetupLogger_Verbose(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := SetupLogger(tempDir, true, false)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test that logger works
	logger.Debug("debug message")
	logger.Info("info message")

	// Verify log file was created
	logFile := filepath.Join(tempDir, "slm.log")
	assert.FileExists(t, logFile)
}

func TestSetupLogger_Quiet(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := SetupLogger(tempDir, false, true)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test that logger works
	logger.Warn("warning message")
	logger.Error("error message")

	// Verify log file was created
	logFile := filepath.Join(tempDir, "slm.log")
	assert.FileExists(t, logFile)
}

func TestSetupLoggerWithConsole(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := SetupLoggerWithConsole(tempDir, false, false)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Test that logger works
	logger.Info("test message")
	logger.Error("test error")

	// Verify log file was created
	logFile := filepath.Join(tempDir, "slm.log")
	assert.FileExists(t, logFile)
}

func TestNewRotatingFileWriter(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	writer := NewRotatingFileWriter(logFile, 1, 3, 7) // 1MB, 3 backups, 7 days
	require.NotNil(t, writer)

	// Test writing
	data := []byte("test log message\n")
	n, err := writer.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Verify file was created
	assert.FileExists(t, logFile)

	// Clean up
	writer.Close()
}

func TestRotatingFileWriter_Rotation(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create writer with very small max size to trigger rotation
	writer := NewRotatingFileWriter(logFile, 0, 3, 7) // 0MB to force immediate rotation
	require.NotNil(t, writer)

	// Write some data
	data := []byte("test log message that is longer than max size\n")
	_, err := writer.Write(data)
	assert.NoError(t, err)

	// Write more data to trigger rotation
	_, err = writer.Write(data)
	assert.NoError(t, err)

	// Clean up
	writer.Close()

	// Verify original file exists
	assert.FileExists(t, logFile)
}

func TestMultiWriter(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "test1.log")
	file2 := filepath.Join(tempDir, "test2.log")

	// Create file writers
	f1, err := os.Create(file1)
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Create(file2)
	require.NoError(t, err)
	defer f2.Close()

	// Create multi writer
	multiWriter := NewMultiWriter(f1, f2)
	require.NotNil(t, multiWriter)

	// Write data
	data := []byte("test message")
	n, err := multiWriter.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)

	// Close files to flush
	f1.Close()
	f2.Close()

	// Verify both files have content
	content1, err := os.ReadFile(file1)
	require.NoError(t, err)
	assert.Equal(t, data, content1)

	content2, err := os.ReadFile(file2)
	require.NoError(t, err)
	assert.Equal(t, data, content2)
}

func TestSetupLogger_InvalidDirectory(t *testing.T) {
	// Try to create logger in a directory we can't write to
	invalidDir := "/nonexistent/directory/that/should/not/exist"

	logger, err := SetupLogger(invalidDir, false, false)
	assert.Error(t, err)
	assert.Nil(t, logger)
}

func TestRotatingFileWriter_CloseNilFile(t *testing.T) {
	writer := &RotatingFileWriter{}
	err := writer.Close()
	assert.NoError(t, err) // Should not error when closing nil file
}