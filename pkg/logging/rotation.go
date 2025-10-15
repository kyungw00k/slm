package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RotatingFileWriter implements log rotation
type RotatingFileWriter struct {
	baseFilename string
	maxSize      int64  // Max size in bytes
	maxBackups   int    // Max number of backup files
	maxAge       int    // Max age in days
	currentFile  *os.File
	currentSize  int64
}

// NewRotatingFileWriter creates a new rotating file writer
func NewRotatingFileWriter(filename string, maxSizeMB int, maxBackups int, maxAge int) *RotatingFileWriter {
	return &RotatingFileWriter{
		baseFilename: filename,
		maxSize:      int64(maxSizeMB) * 1024 * 1024, // Convert MB to bytes
		maxBackups:   maxBackups,
		maxAge:       maxAge,
	}
}

// Write implements io.Writer interface
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	// Ensure we have an open file
	if w.currentFile == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	// Check if rotation is needed
	if w.currentSize+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	// Write to current file
	n, err = w.currentFile.Write(p)
	if err == nil {
		w.currentSize += int64(n)
	}

	return n, err
}

// Close closes the current file
func (w *RotatingFileWriter) Close() error {
	if w.currentFile != nil {
		return w.currentFile.Close()
	}
	return nil
}

// openFile opens a new log file
func (w *RotatingFileWriter) openFile() error {
	// Ensure directory exists
	dir := filepath.Dir(w.baseFilename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Open file
	file, err := os.OpenFile(w.baseFilename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}

	w.currentFile = file
	w.currentSize = info.Size()

	return nil
}

// rotate rotates the log file
func (w *RotatingFileWriter) rotate() error {
	// Close current file
	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
		w.currentSize = 0
	}

	// Generate backup filename with timestamp
	now := time.Now()
	backupName := fmt.Sprintf("%s.%s", w.baseFilename, now.Format("2006-01-02_15-04-05"))

	// Rename current file to backup
	if err := os.Rename(w.baseFilename, backupName); err != nil {
		// If rename fails, just remove the file
		os.Remove(w.baseFilename)
	}

	// Clean old backups
	w.cleanOldBackups()

	// Open new file
	return w.openFile()
}

// cleanOldBackups removes old backup files based on maxBackups and maxAge
func (w *RotatingFileWriter) cleanOldBackups() {
	dir := filepath.Dir(w.baseFilename)
	basename := filepath.Base(w.baseFilename)

	// Find all backup files
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []backupInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if strings.HasPrefix(name, basename+".") {
			info, err := file.Info()
			if err != nil {
				continue
			}

			backups = append(backups, backupInfo{
				name:    name,
				path:    filepath.Join(dir, name),
				modTime: info.ModTime(),
			})
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.After(backups[j].modTime)
	})

	// Remove files based on maxAge
	if w.maxAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -w.maxAge)
		for _, backup := range backups {
			if backup.modTime.Before(cutoff) {
				os.Remove(backup.path)
			}
		}
	}

	// Remove files based on maxBackups
	if w.maxBackups > 0 && len(backups) > w.maxBackups {
		for i := w.maxBackups; i < len(backups); i++ {
			os.Remove(backups[i].path)
		}
	}
}

type backupInfo struct {
	name    string
	path    string
	modTime time.Time
}

// SetupLogger creates a logger with rotation support
func SetupLogger(logDir string, verbose, quiet bool) (*zap.Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Configure log level
	var level zapcore.Level
	if verbose {
		level = zapcore.DebugLevel
	} else if quiet {
		level = zapcore.WarnLevel
	} else {
		level = zapcore.InfoLevel
	}

	// Create rotating writer
	logFile := filepath.Join(logDir, "slm.log")
	rotatingWriter := NewRotatingFileWriter(logFile, 10, 5, 7) // 10MB, 5 backups, 7 days

	// Create encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create core
	core := zapcore.NewCore(encoder, zapcore.AddSync(rotatingWriter), level)

	// Create logger
	logger := zap.New(core, zap.AddCaller())

	return logger, nil
}

// MultiWriter writes to multiple writers
type MultiWriter struct {
	writers []io.Writer
}

// NewMultiWriter creates a writer that writes to multiple writers
func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write implements io.Writer interface
func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// SetupLoggerWithConsole creates a logger that writes to both file and console
func SetupLoggerWithConsole(logDir string, verbose, quiet bool) (*zap.Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Configure log level
	var level zapcore.Level
	if verbose {
		level = zapcore.DebugLevel
	} else if quiet {
		level = zapcore.WarnLevel
	} else {
		level = zapcore.InfoLevel
	}

	// Create rotating writer for file
	logFile := filepath.Join(logDir, "slm.log")
	rotatingWriter := NewRotatingFileWriter(logFile, 10, 5, 7) // 10MB, 5 backups, 7 days

	// Create encoder configs
	fileEncoderConfig := zap.NewProductionEncoderConfig()
	fileEncoderConfig.TimeKey = "timestamp"
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

	consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	// Create cores
	fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(rotatingWriter), level)

	var consoleCore zapcore.Core
	if !quiet {
		consoleCore = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), level)
	}

	// Combine cores
	var core zapcore.Core
	if consoleCore != nil {
		core = zapcore.NewTee(fileCore, consoleCore)
	} else {
		core = fileCore
	}

	// Create logger
	logger := zap.New(core, zap.AddCaller())

	return logger, nil
}