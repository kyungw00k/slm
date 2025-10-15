package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrorTypeFileNotFound, "test file not found")

	assert.Equal(t, ErrorTypeFileNotFound, err.Type)
	assert.Equal(t, "test file not found", err.Message)
	assert.Nil(t, err.Cause)
	assert.NotNil(t, err.Context)
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, ErrorTypeNetwork, "network operation failed")

	assert.Equal(t, ErrorTypeNetwork, wrappedErr.Type)
	assert.Equal(t, "network operation failed", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.NotNil(t, wrappedErr.Context)
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrapf(originalErr, ErrorTypeValidation, "validation failed for field %s", "username")

	assert.Equal(t, ErrorTypeValidation, wrappedErr.Type)
	assert.Equal(t, "validation failed for field username", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
}

func TestSLMError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SLMError
		expected string
	}{
		{
			name: "error without cause",
			err: &SLMError{
				Type:    ErrorTypeFileNotFound,
				Message: "file not found",
			},
			expected: "FILE_NOT_FOUND: file not found",
		},
		{
			name: "error with cause",
			err: &SLMError{
				Type:    ErrorTypeNetwork,
				Message: "network error",
				Cause:   errors.New("connection refused"),
			},
			expected: "NETWORK_ERROR: network error (caused by: connection refused)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestSLMError_Unwrap(t *testing.T) {
	originalErr := errors.New("original")
	wrappedErr := Wrap(originalErr, ErrorTypeInternal, "internal error")

	unwrapped := wrappedErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestSLMError_IsType(t *testing.T) {
	err := New(ErrorTypeFileNotFound, "test")

	assert.True(t, err.IsType(ErrorTypeFileNotFound))
	assert.False(t, err.IsType(ErrorTypeNetwork))
}

func TestSLMError_WithContext(t *testing.T) {
	err := New(ErrorTypeFileNotFound, "test")
	err.WithContext("path", "/test/path")
	err.WithContext("size", 1024)

	path, exists := err.GetContext("path")
	assert.True(t, exists)
	assert.Equal(t, "/test/path", path)

	size, exists := err.GetContext("size")
	assert.True(t, exists)
	assert.Equal(t, 1024, size)

	nonexistent, exists := err.GetContext("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, nonexistent)
}

func TestIsType(t *testing.T) {
	slmErr := New(ErrorTypeFileNotFound, "test")
	regularErr := errors.New("regular error")

	assert.True(t, IsType(slmErr, ErrorTypeFileNotFound))
	assert.False(t, IsType(slmErr, ErrorTypeNetwork))
	assert.False(t, IsType(regularErr, ErrorTypeFileNotFound))
	assert.False(t, IsType(nil, ErrorTypeFileNotFound))
}

func TestGetUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: "regular error",
		},
		{
			name:     "file not found with context",
			err:      New(ErrorTypeFileNotFound, "test").WithContext("path", "/test/file"),
			expected: "File not found: /test/file",
		},
		{
			name:     "file not found without context",
			err:      New(ErrorTypeFileNotFound, "test"),
			expected: "Required file could not be found",
		},
		{
			name:     "config error",
			err:      New(ErrorTypeConfig, "test"),
			expected: "Configuration error. Please check your settings with 'slm config'",
		},
		{
			name:     "network error",
			err:      New(ErrorTypeNetwork, "test"),
			expected: "Network error. Please check your internet connection",
		},
		{
			name:     "validation error",
			err:      New(ErrorTypeValidation, "invalid username"),
			expected: "Validation error: invalid username",
		},
		{
			name:     "template error",
			err:      New(ErrorTypeTemplate, "invalid syntax"),
			expected: "Template error: invalid syntax. Use 'slm scan --list-templates' to see available templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserFriendlyMessage(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRecoveryActions(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectedCount int
		containsText  string
	}{
		{
			name:          "nil error",
			err:           nil,
			expectedCount: 0,
		},
		{
			name:          "file not found",
			err:           New(ErrorTypeFileNotFound, "test"),
			expectedCount: 3,
			containsText:  "Check if the file path is correct",
		},
		{
			name:          "config error",
			err:           New(ErrorTypeConfig, "test"),
			expectedCount: 3,
			containsText:  "Run 'slm config'",
		},
		{
			name:          "network error",
			err:           New(ErrorTypeNetwork, "test"),
			expectedCount: 3,
			containsText:  "Check your internet connection",
		},
		{
			name:          "template error",
			err:           New(ErrorTypeTemplate, "test"),
			expectedCount: 3,
			containsText:  "Use 'slm scan --list-templates'",
		},
		{
			name:          "unknown error type",
			err:           New(ErrorTypeUnknown, "test"),
			expectedCount: 3,
			containsText:  "Check the error details above",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := GetRecoveryActions(tt.err)
			assert.Len(t, actions, tt.expectedCount)

			if tt.containsText != "" && len(actions) > 0 {
				found := false
				for _, action := range actions {
					if assert.Contains(t, action, tt.containsText) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find text '%s' in actions: %v", tt.containsText, actions)
				}
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	err := New(ErrorTypeFileNotFound, "test file").WithContext("path", "/test/file")
	formatted := FormatError(err)

	assert.Contains(t, formatted, "❌ Error:")
	assert.Contains(t, formatted, "File not found: /test/file")
	assert.Contains(t, formatted, "💡 Suggested actions:")
	assert.Contains(t, formatted, "Check if the file path is correct")
}

func TestFormatError_NilError(t *testing.T) {
	formatted := FormatError(nil)
	assert.Empty(t, formatted)
}

func TestFormatError_RegularError(t *testing.T) {
	err := errors.New("regular error")
	formatted := FormatError(err)

	assert.Contains(t, formatted, "❌ Error:")
	assert.Contains(t, formatted, "regular error")
	assert.Contains(t, formatted, "💡 Suggested actions:")
}