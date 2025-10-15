package errors

import (
	"fmt"
	"strings"
)

// SLMError represents an application-specific error
type SLMError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

// ErrorType represents different types of errors
type ErrorType string

const (
	// File operation errors
	ErrorTypeFileNotFound    ErrorType = "FILE_NOT_FOUND"
	ErrorTypeFilePermission  ErrorType = "FILE_PERMISSION"
	ErrorTypeFileCorrupted   ErrorType = "FILE_CORRUPTED"
	ErrorTypeInvalidFile     ErrorType = "INVALID_FILE"

	// Configuration errors
	ErrorTypeConfig         ErrorType = "CONFIG_ERROR"
	ErrorTypeInvalidConfig  ErrorType = "INVALID_CONFIG"
	ErrorTypeMissingConfig  ErrorType = "MISSING_CONFIG"

	// Network errors
	ErrorTypeNetwork        ErrorType = "NETWORK_ERROR"
	ErrorTypeTimeout        ErrorType = "TIMEOUT"
	ErrorTypeDownload       ErrorType = "DOWNLOAD_ERROR"

	// Database errors
	ErrorTypeDatabase       ErrorType = "DATABASE_ERROR"
	ErrorTypeDatabaseCorrupt ErrorType = "DATABASE_CORRUPT"
	ErrorTypeDatabaseLock   ErrorType = "DATABASE_LOCK"

	// Validation errors
	ErrorTypeValidation     ErrorType = "VALIDATION_ERROR"
	ErrorTypeInvalidInput   ErrorType = "INVALID_INPUT"
	ErrorTypeMissingInput   ErrorType = "MISSING_INPUT"

	// Transaction errors
	ErrorTypeTransaction    ErrorType = "TRANSACTION_ERROR"
	ErrorTypeRollback       ErrorType = "ROLLBACK_ERROR"

	// Template errors
	ErrorTypeTemplate       ErrorType = "TEMPLATE_ERROR"
	ErrorTypeInvalidTemplate ErrorType = "INVALID_TEMPLATE"

	// Generic errors
	ErrorTypeUnknown        ErrorType = "UNKNOWN_ERROR"
	ErrorTypeInternal       ErrorType = "INTERNAL_ERROR"
)

// Error implements the error interface
func (e *SLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *SLMError) Unwrap() error {
	return e.Cause
}

// IsType checks if the error is of a specific type
func (e *SLMError) IsType(errorType ErrorType) bool {
	return e.Type == errorType
}

// WithContext adds context to the error
func (e *SLMError) WithContext(key string, value interface{}) *SLMError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// GetContext retrieves context value
func (e *SLMError) GetContext(key string) (interface{}, bool) {
	if e.Context == nil {
		return nil, false
	}
	value, exists := e.Context[key]
	return value, exists
}

// New creates a new SLM error
func New(errorType ErrorType, message string) *SLMError {
	return &SLMError{
		Type:    errorType,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with SLM error type
func Wrap(err error, errorType ErrorType, message string) *SLMError {
	return &SLMError{
		Type:    errorType,
		Message: message,
		Cause:   err,
		Context: make(map[string]interface{}),
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *SLMError {
	return Wrap(err, errorType, fmt.Sprintf(format, args...))
}

// IsType checks if an error is of a specific SLM error type
func IsType(err error, errorType ErrorType) bool {
	if slmErr, ok := err.(*SLMError); ok {
		return slmErr.IsType(errorType)
	}
	return false
}

// GetUserFriendlyMessage returns a user-friendly error message
func GetUserFriendlyMessage(err error) string {
	if err == nil {
		return ""
	}

	slmErr, ok := err.(*SLMError)
	if !ok {
		return err.Error()
	}

	switch slmErr.Type {
	case ErrorTypeFileNotFound:
		if path, exists := slmErr.GetContext("path"); exists {
			return fmt.Sprintf("File not found: %v", path)
		}
		return "Required file could not be found"

	case ErrorTypeFilePermission:
		if path, exists := slmErr.GetContext("path"); exists {
			return fmt.Sprintf("Permission denied accessing file: %v", path)
		}
		return "Permission denied accessing file"

	case ErrorTypeFileCorrupted:
		if path, exists := slmErr.GetContext("path"); exists {
			return fmt.Sprintf("File appears to be corrupted: %v", path)
		}
		return "File appears to be corrupted"

	case ErrorTypeInvalidFile:
		if path, exists := slmErr.GetContext("path"); exists {
			return fmt.Sprintf("Invalid file format: %v", path)
		}
		return "Invalid file format"

	case ErrorTypeConfig:
		return "Configuration error. Please check your settings with 'slm config'"

	case ErrorTypeInvalidConfig:
		return "Invalid configuration. Use 'slm config --reset' to restore defaults"

	case ErrorTypeMissingConfig:
		return "Missing configuration. Run 'slm config' to set up initial configuration"

	case ErrorTypeNetwork:
		return "Network error. Please check your internet connection"

	case ErrorTypeTimeout:
		return "Operation timed out. Please try again"

	case ErrorTypeDownload:
		return "Failed to download required files. Please check your internet connection"

	case ErrorTypeDatabase:
		return "Database error. Try cleaning cache with 'slm cache --clean'"

	case ErrorTypeDatabaseCorrupt:
		return "Database is corrupted. Please run 'slm cache --clean' to rebuild"

	case ErrorTypeDatabaseLock:
		return "Database is locked by another process. Please wait and try again"

	case ErrorTypeValidation:
		return fmt.Sprintf("Validation error: %s", slmErr.Message)

	case ErrorTypeInvalidInput:
		return fmt.Sprintf("Invalid input: %s", slmErr.Message)

	case ErrorTypeMissingInput:
		return fmt.Sprintf("Missing required input: %s", slmErr.Message)

	case ErrorTypeTransaction:
		return "File operation failed. All changes have been rolled back"

	case ErrorTypeRollback:
		return "Rollback failed. Some files may be in an inconsistent state"

	case ErrorTypeTemplate:
		return fmt.Sprintf("Template error: %s. Use 'slm scan --list-templates' to see available templates", slmErr.Message)

	case ErrorTypeInvalidTemplate:
		return "Invalid template format. Templates must contain {TITLE_NAME} or {TITLE_ID}"

	case ErrorTypeInternal:
		return "Internal error occurred. Please report this issue"

	default:
		return slmErr.Message
	}
}

// GetRecoveryActions returns suggested recovery actions for an error
func GetRecoveryActions(err error) []string {
	if err == nil {
		return nil
	}

	slmErr, ok := err.(*SLMError)
	if !ok {
		return []string{"Please check the error details and try again"}
	}

	switch slmErr.Type {
	case ErrorTypeFileNotFound:
		return []string{
			"Check if the file path is correct",
			"Ensure the file exists and is readable",
			"For game files, check if they are in the correct directory",
		}

	case ErrorTypeFilePermission:
		return []string{
			"Check file permissions",
			"Run with appropriate user privileges",
			"Ensure the directory is writable",
		}

	case ErrorTypeConfig:
		return []string{
			"Run 'slm config' to check current configuration",
			"Use 'slm config --reset' to restore default settings",
			"Check if prod.keys file is properly configured",
		}

	case ErrorTypeNetwork, ErrorTypeDownload:
		return []string{
			"Check your internet connection",
			"Try again in a few minutes",
			"Use 'slm cache --update' to force refresh databases",
		}

	case ErrorTypeDatabase, ErrorTypeDatabaseCorrupt:
		return []string{
			"Run 'slm cache --clean' to clear corrupted data",
			"Re-scan your game library",
			"Check available disk space",
		}

	case ErrorTypeValidation, ErrorTypeInvalidInput:
		return []string{
			"Check command syntax with --help",
			"Verify input parameters are correct",
			"Use quotes around paths with spaces",
		}

	case ErrorTypeTemplate:
		return []string{
			"Use 'slm scan --list-templates' to see available templates",
			"Check template syntax in documentation",
			"Try using a predefined template first",
		}

	case ErrorTypeTransaction:
		return []string{
			"Check available disk space",
			"Ensure destination directory is writable",
			"Try running operation in dry-run mode first",
		}

	default:
		return []string{
			"Check the error details above",
			"Try running the command again",
			"Report this issue if the problem persists",
		}
	}
}

// FormatError formats an error for display with recovery actions
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var result strings.Builder

	// Main error message
	result.WriteString("❌ Error: ")
	result.WriteString(GetUserFriendlyMessage(err))
	result.WriteString("\n")

	// Recovery actions
	actions := GetRecoveryActions(err)
	if len(actions) > 0 {
		result.WriteString("\n💡 Suggested actions:\n")
		for _, action := range actions {
			result.WriteString(fmt.Sprintf("   • %s\n", action))
		}
	}

	return result.String()
}