package cmd

import (
	"fmt"
	"strings"

	"github.com/giwty/switch-library-manager/pkg/config"
	"github.com/giwty/switch-library-manager/pkg/scanner"
	"github.com/giwty/switch-library-manager/pkg/templates"
	"github.com/spf13/cobra"
)

var (
	// Scan flags
	folder         string
	folders        string
	recursive      bool
	noRecursive    bool
	format         string
	locale         string
	showTable      bool

	// Check flags
	checkAll       bool
	checkUpdates   bool
	checkDLC       bool
	noCheck        bool
	ignoreDLC      string

	// Organize flags
	rename         bool
	createFolders  bool
	deleteOldUpdates bool
	template       string
	listTemplates  bool
	dryRun         bool

	// Performance flags
	maxWorkers     int
	memoryLimit    int
	enableProfiling bool
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan and process game library",
	Long: `Scan your Nintendo Switch game library and optionally organize files and check for missing content.

This command combines scanning, checking for missing updates/DLC, and organizing files into a single operation
to ensure database consistency.

Examples:
  # Basic scan
  slm scan -f /games

  # Scan multiple folders
  slm scan -F "/games1,/games2"

  # Scan and organize with dry-run
  slm scan -f /games --rename --create-folders --dry-run

  # Scan with specific checks
  slm scan -f /games --check-updates --no-check-dlc

  # Output as JSON
  slm scan -f /games --format json`,
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	// Required folder flags
	scanCmd.Flags().StringVarP(&folder, "folder", "f", "", "folder to scan")
	scanCmd.Flags().StringVarP(&folders, "folders", "F", "", "comma-separated list of folders to scan")
	scanCmd.MarkFlagsMutuallyExclusive("folder", "folders")

	// Scan options
	scanCmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "scan recursively (default: true)")
	scanCmd.Flags().BoolVar(&noRecursive, "no-recursive", false, "disable recursive scanning")
	scanCmd.Flags().StringVar(&format, "format", "table", "output format (table, json, csv) - only used with --show-table")
	scanCmd.Flags().StringVar(&locale, "locale", "", "locale for title names (e.g., KR.ko, US.en, JP.ja)")
	scanCmd.Flags().BoolVar(&showTable, "show-table", false, "show full game table after scan (default: summary only)")

	// Check flags (mutually exclusive group)
	scanCmd.Flags().BoolVar(&checkAll, "check-all", true, "check for all missing content (default)")
	scanCmd.Flags().BoolVar(&checkUpdates, "check-updates", false, "check for missing updates only")
	scanCmd.Flags().BoolVar(&checkDLC, "check-dlc", false, "check for missing DLC only")
	scanCmd.Flags().BoolVar(&noCheck, "no-check", false, "skip missing content checks")
	scanCmd.Flags().StringVar(&ignoreDLC, "ignore-dlc", "", "comma-separated list of DLC title IDs to ignore")
	scanCmd.MarkFlagsMutuallyExclusive("check-all", "check-updates", "check-dlc", "no-check")

	// Organize flags
	scanCmd.Flags().BoolVar(&rename, "rename", false, "rename files based on metadata")
	scanCmd.Flags().BoolVar(&createFolders, "create-folders", false, "create folders per game")
	scanCmd.Flags().BoolVar(&deleteOldUpdates, "delete-old-updates", false, "delete old update files")
	scanCmd.Flags().StringVar(&template, "template", "", "custom naming template")
	scanCmd.Flags().BoolVar(&listTemplates, "list-templates", false, "list available templates")
	scanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")

	// Performance flags
	scanCmd.Flags().IntVar(&maxWorkers, "max-workers", 0, "maximum concurrent workers (0 = auto)")
	scanCmd.Flags().IntVar(&memoryLimit, "memory-limit", 512, "memory limit in MB")
	scanCmd.Flags().BoolVar(&enableProfiling, "profile", false, "enable performance profiling")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Handle --list-templates flag
	if listTemplates {
		fmt.Println(templates.ListTemplates())
		return nil
	}

	// Validate required flags
	if folder == "" && folders == "" {
		return fmt.Errorf("either --folder or --folders must be specified")
	}

	// Parse folders
	var scanFolders []string
	if folder != "" {
		scanFolders = []string{folder}
	} else {
		scanFolders = strings.Split(folders, ",")
		for i, f := range scanFolders {
			scanFolders[i] = strings.TrimSpace(f)
		}
	}

	// Validate format
	if format != "table" && format != "json" && format != "csv" {
		return fmt.Errorf("invalid format: %s (must be table, json, or csv)", format)
	}

	// Handle no-recursive flag
	if cmd.Flags().Changed("no-recursive") {
		recursive = false
	}

	// Setup scan options
	opts := &ScanOptions{
		Folders:           scanFolders,
		Recursive:         recursive,
		Format:            format,
		Locale:            locale,
		ShowTable:         showTable,
		CheckAll:          checkAll,
		CheckUpdates:      checkUpdates,
		CheckDLC:          checkDLC,
		NoCheck:           noCheck,
		IgnoreDLC:         parseIgnoreDLC(ignoreDLC),
		Rename:            rename,
		CreateFolders:     createFolders,
		DeleteOldUpdates:  deleteOldUpdates,
		Template:          template,
		DryRun:            dryRun,
		OutputMode:        getOutputMode(),
		MaxWorkers:        maxWorkers,
		MemoryLimit:       memoryLimit,
		EnableProfiling:   enableProfiling,
	}

	// Run the scan
	return executeScan(opts)
}

// ScanOptions holds all scan configuration
type ScanOptions struct {
	Folders           []string
	Recursive         bool
	Format            string
	Locale            string
	ShowTable         bool
	CheckAll          bool
	CheckUpdates      bool
	CheckDLC          bool
	NoCheck           bool
	IgnoreDLC         []string
	Rename            bool
	CreateFolders     bool
	DeleteOldUpdates  bool
	Template          string
	DryRun            bool
	OutputMode        string
	MaxWorkers        int
	MemoryLimit       int
	EnableProfiling   bool
}

func parseIgnoreDLC(ignoreDLCStr string) []string {
	if ignoreDLCStr == "" {
		return nil
	}
	dlcIDs := strings.Split(ignoreDLCStr, ",")
	for i, id := range dlcIDs {
		dlcIDs[i] = strings.TrimSpace(id)
	}
	return dlcIDs
}

func executeScan(opts *ScanOptions) error {
	// Convert ScanOptions to scanner.Options
	scannerOpts := &scanner.Options{
		Folders:           opts.Folders,
		Recursive:         opts.Recursive,
		Format:            opts.Format,
		Locale:            opts.Locale,
		ShowTable:         opts.ShowTable,
		CheckAll:          opts.CheckAll,
		CheckUpdates:      opts.CheckUpdates,
		CheckDLC:          opts.CheckDLC,
		NoCheck:           opts.NoCheck,
		IgnoreDLC:         opts.IgnoreDLC,
		Rename:            opts.Rename,
		CreateFolders:     opts.CreateFolders,
		DeleteOldUpdates:  opts.DeleteOldUpdates,
		Template:          opts.Template,
		DryRun:            opts.DryRun,
		OutputMode:        opts.OutputMode,
		MaxWorkers:        opts.MaxWorkers,
		MemoryLimit:       opts.MemoryLimit,
		EnableProfiling:   opts.EnableProfiling,
	}

	// Create config manager
	configMgr := config.NewManager(getConfigDir())

	// Create scanner
	s, err := scanner.NewScanner(configMgr, scannerOpts)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %v", err)
	}

	// Run scan with progress tracking
	return s.Run()
}