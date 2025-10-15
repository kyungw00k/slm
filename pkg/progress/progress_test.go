package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineOutputMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		expected OutputMode
	}{
		{
			name:     "specific mode simple",
			mode:     OutputModeSimple,
			expected: OutputModeSimple,
		},
		{
			name:     "specific mode rich",
			mode:     OutputModeRich,
			expected: OutputModeRich,
		},
		{
			name:     "specific mode json",
			mode:     OutputModeJSON,
			expected: OutputModeJSON,
		},
		{
			name:     "auto mode",
			mode:     OutputModeAuto,
			expected: OutputModeSimple, // Non-TTY environment in tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineOutputMode(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseOutputMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected OutputMode
	}{
		{
			name:     "valid auto",
			mode:     "auto",
			expected: OutputModeAuto,
		},
		{
			name:     "valid simple",
			mode:     "simple",
			expected: OutputModeSimple,
		},
		{
			name:     "valid rich",
			mode:     "rich",
			expected: OutputModeRich,
		},
		{
			name:     "valid json",
			mode:     "json",
			expected: OutputModeJSON,
		},
		{
			name:     "valid csv",
			mode:     "csv",
			expected: OutputModeCSV,
		},
		{
			name:     "invalid mode",
			mode:     "invalid",
			expected: OutputModeAuto,
		},
		{
			name:     "empty string",
			mode:     "",
			expected: OutputModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOutputMode(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTTY(t *testing.T) {
	// Test that isTTY function exists and returns a boolean
	result := isTTY()
	assert.IsType(t, false, result)
}

func TestMinimalProgressUpdater(t *testing.T) {
	updater := &MinimalProgressUpdater{}

	// Test all methods exist and return nil
	err := updater.UpdateProgress(0, 1, 10, "test message")
	assert.NoError(t, err)

	err = updater.CompleteStage(0)
	assert.NoError(t, err)

	err = updater.ErrorStage(assert.AnError)
	assert.NoError(t, err)

	err = updater.Finish()
	assert.NoError(t, err)
}