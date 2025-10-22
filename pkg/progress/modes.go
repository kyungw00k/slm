package progress

import (
	"fmt"
	"os"
)

// OutputMode represents different progress output modes
type OutputMode string

const (
	OutputModeAuto   OutputMode = "auto"
	OutputModeSimple OutputMode = "simple"
	OutputModeRich   OutputMode = "rich"
	OutputModeJSON   OutputMode = "json"
	OutputModeCSV    OutputMode = "csv"
)

// ParseOutputMode parses output mode from string
func ParseOutputMode(mode string) OutputMode {
	switch mode {
	case "auto":
		return OutputModeAuto
	case "simple":
		return OutputModeSimple
	case "rich":
		return OutputModeRich
	case "json":
		return OutputModeJSON
	case "csv":
		return OutputModeCSV
	default:
		return OutputModeAuto
	}
}

// DetermineOutputMode resolves auto mode to actual mode
func DetermineOutputMode(mode OutputMode) OutputMode {
	if mode != OutputModeAuto {
		return mode
	}

	// Auto mode: use simple to avoid table rendering conflicts
	// Rich mode (Bubble Tea) can conflict with table output
	return OutputModeSimple
}

// RunWithProgressMode runs with the specified output mode
func RunWithProgressMode(stages []Stage, mode OutputMode, fn func(ProgressUpdater) error) error {
	actualMode := DetermineOutputMode(mode)

	switch actualMode {
	case OutputModeRich:
		return RunWithProgress(stages, fn)
	case OutputModeSimple:
		return runWithSimpleProgress(stages, fn)
	case OutputModeJSON, OutputModeCSV:
		// For structured output modes, use minimal progress
		return runWithMinimalProgress(stages, fn)
	default:
		return runWithSimpleProgress(stages, fn)
	}
}

// MinimalProgressUpdater for structured output modes
type MinimalProgressUpdater struct{}

func (m *MinimalProgressUpdater) UpdateProgress(stage int, current, total int, message string, details ...string) error {
	// Silent for structured output
	return nil
}

func (m *MinimalProgressUpdater) CompleteStage(stage int) error {
	// Silent for structured output
	return nil
}

func (m *MinimalProgressUpdater) ErrorStage(err error) error {
	// Only show errors to stderr
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	return nil
}

func (m *MinimalProgressUpdater) Finish() error {
	// Silent for structured output
	return nil
}

func runWithMinimalProgress(stages []Stage, fn func(ProgressUpdater) error) error {
	updater := &MinimalProgressUpdater{}
	return fn(updater)
}