package progress

import (
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
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

// WorkerStats represents worker pool statistics
type WorkerStats struct {
	TotalWorkers  int
	ActiveWorkers int
	FilesPerSec   float64
	ETA           time.Duration
}

// ProgressMsg represents progress updates
type ProgressMsg struct {
	Stage   int
	Current int
	Total   int
	Message string
	Details []string

	// Worker statistics (optional)
	WorkerStats *WorkerStats
	Environment string
}

// ProgressUpdater is an interface for progress updates
type ProgressUpdater interface {
	UpdateProgress(stage int, current, total int, message string, details ...string) error
	CompleteStage(stage int) error
	ErrorStage(err error) error
	Finish() error
}

// PrettyProgressUpdater implements ProgressUpdater using go-pretty
type PrettyProgressUpdater struct {
	pw           progress.Writer
	trackers     []*progress.Tracker
	stages       []Stage
	currentStage int
}

// NewPrettyProgressUpdater creates a new go-pretty based progress updater
func NewPrettyProgressUpdater(stages []Stage) *PrettyProgressUpdater {
	pw := progress.NewWriter()
	pw.SetAutoStop(false)
	pw.SetTrackerLength(25)
	pw.SetUpdateFrequency(time.Millisecond * 100)
	pw.Style().Colors = progress.StyleColorsExample
	pw.Style().Options.PercentFormat = "%4.1f%%"
	pw.Style().Visibility.TrackerOverall = true
	pw.Style().Visibility.ETA = true
	pw.Style().Visibility.Percentage = true

	// Create trackers for each stage
	trackers := make([]*progress.Tracker, len(stages))
	for i, stage := range stages {
		tracker := &progress.Tracker{
			Message: stage.Name,
			Total:   int64(stage.Total),
			Units:   progress.UnitsDefault,
		}
		trackers[i] = tracker
		pw.AppendTracker(tracker)
	}

	// Start rendering
	go pw.Render()

	return &PrettyProgressUpdater{
		pw:           pw,
		trackers:     trackers,
		stages:       stages,
		currentStage: -1,
	}
}

func (p *PrettyProgressUpdater) UpdateProgress(stage int, current, total int, message string, details ...string) error {
	if stage < 0 || stage >= len(p.trackers) {
		return nil
	}

	tracker := p.trackers[stage]

	// Update total if changed
	if total > 0 && int64(total) != tracker.Total {
		tracker.UpdateTotal(int64(total))
	}

	// Update current progress
	if current >= 0 {
		tracker.SetValue(int64(current))
	}

	// Update message
	if message != "" {
		displayMsg := p.stages[stage].Name
		if message != "" {
			displayMsg = fmt.Sprintf("%s - %s", displayMsg, message)
		}
		tracker.UpdateMessage(displayMsg)
	}

	p.currentStage = stage
	p.stages[stage].Status = StageInProgress
	p.stages[stage].Current = current
	p.stages[stage].Total = total

	return nil
}

func (p *PrettyProgressUpdater) CompleteStage(stage int) error {
	if stage >= 0 && stage < len(p.trackers) {
		p.trackers[stage].MarkAsDone()
		p.stages[stage].Status = StageCompleted
	}
	return nil
}

func (p *PrettyProgressUpdater) ErrorStage(err error) error {
	if p.currentStage >= 0 && p.currentStage < len(p.trackers) {
		p.trackers[p.currentStage].MarkAsErrored()
		p.stages[p.currentStage].Status = StageFailed
	}
	fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
	return nil
}

func (p *PrettyProgressUpdater) Finish() error {
	// Wait for all trackers to finish rendering
	for p.pw.IsRenderInProgress() {
		time.Sleep(time.Millisecond * 10)
	}
	p.pw.Stop()
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
			fmt.Printf("\r[%d/%d] %s (completed)\n", s.currentStage+1, len(s.stages), s.stages[s.currentStage].Name)
		}
		s.currentStage = stage
		if stage < len(s.stages) {
			fmt.Printf("[%d/%d] %s\n", stage+1, len(s.stages), s.stages[stage].Name)
		}
	}

	// Show progress updates
	if total > 0 && current > 0 && current%10 == 0 {
		// Show percentage for stages with known totals (every 10 items)
		percentage := float64(current) / float64(total) * 100
		fmt.Fprintf(os.Stderr, "\r[%d/%d] %s %d/%d (%.0f%%)", stage+1, len(s.stages), s.stages[stage].Name, current, total, percentage)
		os.Stderr.Sync() // Force flush
	} else if current > 0 && total <= 0 {
		// Show count for discovery stages (unknown total) - every item
		fmt.Fprintf(os.Stderr, "\r[%d/%d] %s %d files", stage+1, len(s.stages), s.stages[stage].Name, current)
		os.Stderr.Sync() // Force flush
	}
	return nil
}

func (s *SimpleProgressUpdater) CompleteStage(stage int) error {
	if stage < len(s.stages) {
		fmt.Printf("\r[%d/%d] %s (completed)\n", stage+1, len(s.stages), s.stages[stage].Name)
	}
	return nil
}

func (s *SimpleProgressUpdater) ErrorStage(err error) error {
	fmt.Printf("\nError: %v\n", err)
	return nil
}

func (s *SimpleProgressUpdater) Finish() error {
	fmt.Println("\nCompleted")
	return nil
}

// RunWithProgress runs a function with progress tracking
func RunWithProgress(stages []Stage, fn func(ProgressUpdater) error) error {
	// Check if we have a TTY, if not use simple progress
	if !isTTY() {
		return runWithSimpleProgress(stages, fn)
	}

	updater := NewPrettyProgressUpdater(stages)

	// Run the function
	err := fn(updater)

	// Finish progress display
	updater.Finish()

	if err != nil {
		return err
	}

	return nil
}

func runWithSimpleProgress(stages []Stage, fn func(ProgressUpdater) error) error {
	updater := &SimpleProgressUpdater{
		currentStage: -1,
		stages:       stages,
	}

	fmt.Println("Switch Library Manager")
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

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
