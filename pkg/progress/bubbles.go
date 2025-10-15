package progress

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Stage represents a processing stage
type Stage struct {
	Name        string
	Description string
	Total       int
	Current     int
	Status      StageStatus
}

type StageStatus int

const (
	StageNotStarted StageStatus = iota
	StageInProgress
	StageCompleted
	StageFailed
)

// Model represents the progress UI model
type Model struct {
	stages    []Stage
	current   int
	spinner   spinner.Model
	progress  progress.Model
	quitting  bool
	err       error
	message   string
	details   []string
	
	// Discovery phase feedback
	filesFound  int
	totalSize   int64
	recentFiles []string
}

// ProgressMsg represents progress updates
type ProgressMsg struct {
	Stage   int
	Current int
	Total   int
	Message string
	Details []string
}

// CompleteMsg indicates stage completion
type CompleteMsg struct {
	Stage int
}

// ErrorMsg indicates an error
type ErrorMsg struct {
	Error error
}

// QuitMsg indicates completion
type QuitMsg struct{}

// NewModel creates a new progress model
func NewModel(stages []Stage) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(progress.WithDefaultGradient())

	return Model{
		stages:      stages,
		current:     0,
		spinner:     s,
		progress:    p,
		details:     make([]string, 0, 5),
		recentFiles: make([]string, 0, 3),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case ProgressMsg:
		if msg.Stage >= 0 && msg.Stage < len(m.stages) {
			m.stages[msg.Stage].Current = msg.Current
			m.stages[msg.Stage].Total = msg.Total
			m.stages[msg.Stage].Status = StageInProgress
			m.message = msg.Message
			
			// Parse discovery phase info from message
			if msg.Current < 0 && msg.Total < 0 {
				// Discovery phase - extract filename from "scanning <filename>"
				if strings.HasPrefix(msg.Message, "scanning ") {
					fileName := strings.TrimPrefix(msg.Message, "scanning ")
					if fileName != "" {
						m.filesFound++
						// Add to recent files (keep last 3)
						m.recentFiles = append(m.recentFiles, fileName)
						if len(m.recentFiles) > 3 {
							m.recentFiles = m.recentFiles[len(m.recentFiles)-3:]
						}
					}
				}
			}
			
			if len(msg.Details) > 0 {
				m.details = append(m.details, msg.Details...)
				// Keep only last 3 details
				if len(m.details) > 3 {
					m.details = m.details[len(m.details)-3:]
				}
			}
		}

	case CompleteMsg:
		if msg.Stage >= 0 && msg.Stage < len(m.stages) {
			m.stages[msg.Stage].Status = StageCompleted
			m.current = msg.Stage + 1
			// Reset discovery info for next stage
			m.filesFound = 0
			m.totalSize = 0
			m.recentFiles = make([]string, 0, 3)
		}

	case ErrorMsg:
		m.err = msg.Error
		if m.current < len(m.stages) {
			m.stages[m.current].Status = StageFailed
		}
		return m, tea.Quit

	case QuitMsg:
		m.quitting = true
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("❌ Error: %v\n", m.err)
	}

	if m.quitting {
		if m.current >= len(m.stages) {
			return "✅ All operations completed successfully!\n"
		}
		return "🛑 Operation cancelled\n"
	}

	var b strings.Builder

	// Header
	b.WriteString("🎮 Switch Library Manager\n\n")

	// Stages
	for i, stage := range m.stages {
		switch stage.Status {
		case StageNotStarted:
			b.WriteString(fmt.Sprintf("⏸️  [%d/%d] %s\n", i+1, len(m.stages), stage.Name))
		case StageInProgress:
			if stage.Total > 0 {
				percentage := float64(stage.Current) / float64(stage.Total)
				progressBar := m.progress.ViewAs(percentage)
				b.WriteString(fmt.Sprintf("🔄 [%d/%d] %s %s %d/%d (%.1f%%)\n",
					i+1, len(m.stages), stage.Name, progressBar, stage.Current, stage.Total, percentage*100))
			} else {
				// Discovery phase - show spinner with file count
				spinnerView := m.spinner.View()
				b.WriteString(fmt.Sprintf("🔄 [%d/%d] %s %s",
					i+1, len(m.stages), stage.Name, spinnerView))
				
				// Show files found if available
				if m.filesFound > 0 {
					b.WriteString(fmt.Sprintf("  Found: %d files", m.filesFound))
				}
				b.WriteString("\n")
				
				// Show recent files discovered
				if len(m.recentFiles) > 0 && i == m.current {
					b.WriteString("      Recent:\n")
					for j, fileName := range m.recentFiles {
						if j == len(m.recentFiles)-1 {
							b.WriteString(fmt.Sprintf("      └─ %s\n", fileName))
						} else {
							b.WriteString(fmt.Sprintf("      ├─ %s\n", fileName))
						}
					}
				}
			}
		case StageCompleted:
			b.WriteString(fmt.Sprintf("✅ [%d/%d] %s\n", i+1, len(m.stages), stage.Name))
		case StageFailed:
			b.WriteString(fmt.Sprintf("❌ [%d/%d] %s\n", i+1, len(m.stages), stage.Name))
		}
	}

	// Current message
	if m.message != "" {
		b.WriteString(fmt.Sprintf("\n📋 %s\n", m.message))
	}

	// Details
	if len(m.details) > 0 {
		b.WriteString("\n📄 Recent activity:\n")
		for _, detail := range m.details {
			b.WriteString(fmt.Sprintf("   %s\n", detail))
		}
	}

	b.WriteString("\n⌨️  Press 'q' to quit")

	return b.String()
}

// GetError returns any error that occurred during execution
func (m Model) GetError() error {
	return m.err
}

// ProgressUpdater is an interface for progress updates
type ProgressUpdater interface {
	UpdateProgress(stage int, current, total int, message string, details ...string) error
	CompleteStage(stage int) error
	ErrorStage(err error) error
	Finish() error
}

// TeaProgressUpdater implements ProgressUpdater for Bubble Tea
type TeaProgressUpdater struct {
	program *tea.Program
}

// NewTeaProgressUpdater creates a new Tea progress updater
func NewTeaProgressUpdater(model Model) *TeaProgressUpdater {
	program := tea.NewProgram(model)
	return &TeaProgressUpdater{program: program}
}

func (t *TeaProgressUpdater) UpdateProgress(stage int, current, total int, message string, details ...string) error {
	t.program.Send(ProgressMsg{
		Stage:   stage,
		Current: current,
		Total:   total,
		Message: message,
		Details: details,
	})
	return nil
}

func (t *TeaProgressUpdater) CompleteStage(stage int) error {
	t.program.Send(CompleteMsg{Stage: stage})
	return nil
}

func (t *TeaProgressUpdater) ErrorStage(err error) error {
	t.program.Send(ErrorMsg{Error: err})
	return nil
}

func (t *TeaProgressUpdater) Finish() error {
	t.program.Send(QuitMsg{})
	return nil
}

// RunWithProgress runs a function with progress tracking
func RunWithProgress(stages []Stage, fn func(ProgressUpdater) error) error {
	// Check if we have a TTY, if not use simple progress
	if !isTTY() {
		return runWithSimpleProgress(stages, fn)
	}

	model := NewModel(stages)
	updater := NewTeaProgressUpdater(model)

	// Run the function in a goroutine
	go func() {
		err := fn(updater)
		if err != nil {
			updater.ErrorStage(err)
		} else {
			updater.Finish()
		}
	}()

	// Start the UI (only once)
	finalModel, err := updater.program.Run()
	if err != nil {
		return err
	}

	// Extract any error from the model
	if m, ok := finalModel.(Model); ok {
		if m.err != nil {
			return m.err
		}
	}

	return nil
}

// SimpleProgressUpdater implements ProgressUpdater for non-TTY environments
type SimpleProgressUpdater struct {
	currentStage int
	stages       []Stage
}

func (s *SimpleProgressUpdater) UpdateProgress(stage int, current, total int, message string, details ...string) error {
	if stage != s.currentStage {
		if s.currentStage < len(s.stages) && s.currentStage >= 0 {
			fmt.Printf("✅ [%d/%d] %s\n", s.currentStage+1, len(s.stages), s.stages[s.currentStage].Name)
		}
		s.currentStage = stage
		if stage < len(s.stages) {
			fmt.Printf("🔄 [%d/%d] %s\n", stage+1, len(s.stages), s.stages[stage].Name)
		}
	}

	if message != "" {
		if total > 0 {
			percentage := float64(current) / float64(total) * 100
			fmt.Printf("   %s (%d/%d - %.1f%%)\n", message, current, total, percentage)
		} else {
			fmt.Printf("   %s\n", message)
		}
	}
	return nil
}

func (s *SimpleProgressUpdater) CompleteStage(stage int) error {
	if stage < len(s.stages) {
		fmt.Printf("✅ [%d/%d] %s\n", stage+1, len(s.stages), s.stages[stage].Name)
	}
	return nil
}

func (s *SimpleProgressUpdater) ErrorStage(err error) error {
	fmt.Printf("❌ Error: %v\n", err)
	return nil
}

func (s *SimpleProgressUpdater) Finish() error {
	fmt.Println("✅ All operations completed successfully!")
	return nil
}

func runWithSimpleProgress(stages []Stage, fn func(ProgressUpdater) error) error {
	updater := &SimpleProgressUpdater{
		currentStage: -1,
		stages:       stages,
	}

	fmt.Println("🎮 Switch Library Manager")
	fmt.Println("")

	if err := fn(updater); err != nil {
		updater.ErrorStage(err)
		return err
	}

	updater.Finish()
	return nil
}

func isTTY() bool {
	// Check if stdout is a TTY
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return true
	}
	return false
}

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}