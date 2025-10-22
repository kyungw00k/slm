package cmd

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/pkg/config"
	"github.com/giwty/switch-library-manager/pkg/scanner"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/spf13/cobra"
)

var (
	// Filter flags
	missingUpdates bool
	missingDLC     bool
	titleFilter    string
	titleIDFilter  string
	limit          int

	// Sorting flags
	sortBy string

	// Pagination flags
	page    int
	perPage int

	// Output flags
	listFormat string

	// Database selection
	listScanPath string
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List games from the database",
	Long: `List games from the local database created by the scan command.

This command provides various filtering, sorting, and pagination options to view your game library.

Examples:
  # List first 50 games
  slm list

  # List games missing updates
  slm list --missing-updates

  # Search for Zelda games
  slm list --title "zelda"

  # Show second page with 100 games per page
  slm list --page 2 --per-page 100

  # Export to JSON
  slm list --format json

  # List specific title by ID
  slm list --title-id 0100f2c0115b6000

  # Limit to top 20 games
  slm list --limit 20`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Filter flags
	listCmd.Flags().BoolVar(&missingUpdates, "missing-updates", false, "show only games missing updates")
	listCmd.Flags().BoolVar(&missingDLC, "missing-dlc", false, "show only games missing DLC")
	listCmd.Flags().StringVar(&titleFilter, "title", "", "filter by game title (case-insensitive)")
	listCmd.Flags().StringVar(&titleIDFilter, "title-id", "", "filter by title ID")
	listCmd.Flags().IntVar(&limit, "limit", 0, "limit number of results (0 = no limit)")

	// Sorting flags
	listCmd.Flags().StringVar(&sortBy, "sort", "title", "sort by (title, title-id, missing)")

	// Pagination flags
	listCmd.Flags().IntVar(&page, "page", 1, "page number (1-based)")
	listCmd.Flags().IntVar(&perPage, "per-page", 50, "results per page")

	// Output flags
	listCmd.Flags().StringVar(&listFormat, "format", "table", "output format (table, json, csv)")

	// Database selection flags
	listCmd.Flags().StringVar(&listScanPath, "scan-path", "", "scan path to select specific database (default: use most recent DB)")
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate format
	if listFormat != "table" && listFormat != "json" && listFormat != "csv" {
		return fmt.Errorf("invalid format: %s (must be table, json, or csv)", listFormat)
	}

	// Validate sort option
	if sortBy != "title" && sortBy != "title-id" && sortBy != "missing" {
		return fmt.Errorf("invalid sort option: %s (must be title, title-id, or missing)", sortBy)
	}

	// Validate pagination
	if page < 1 {
		return fmt.Errorf("page must be >= 1")
	}
	if perPage < 1 {
		return fmt.Errorf("per-page must be >= 1")
	}

	// Find database based on scan path or use most recent
	configMgr := config.NewManager(getConfigDir())
	dbPath, err := findDB(configMgr, listScanPath)
	if err != nil {
		return fmt.Errorf("no scan database found: %v\nPlease run 'slm scan -f <folder>' first to create the database", err)
	}

	// Open database using the full path
	ldb, err := db.NewLocalSwitchDBManagerWithPath(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer ldb.Close()

	// Load title database for enriched information
	titlesDB, err := loadTitlesDB(configMgr)
	if err != nil {
		// Non-fatal: we can list without title DB
		fmt.Printf("Warning: could not load titles database: %v\n", err)
	}

	// Build list options
	opts := db.ListOptions{
		MissingUpdates: missingUpdates,
		MissingDLC:     missingDLC,
		TitleFilter:    titleFilter,
		TitleIDFilter:  titleIDFilter,
		Limit:          limit,
		SortBy:         sortBy,
		Page:           page,
		PerPage:        perPage,
	}

	// Query games
	games, totalCount, err := ldb.ListGames(opts, titlesDB)
	if err != nil {
		return fmt.Errorf("failed to list games: %v", err)
	}

	// Check if no results
	if len(games) == 0 {
		if titleFilter != "" || titleIDFilter != "" || missingUpdates || missingDLC {
			fmt.Println("No games found matching the filters")
		} else {
			fmt.Println("No games found in database")
		}
		return nil
	}

	// Output results
	return outputListResults(games, totalCount, opts, titlesDB, configMgr)
}

// findDB finds a database either by scan path or returns the most recent one
func findDB(configMgr *config.Manager, scanPath string) (string, error) {
	dbDir := configMgr.GetLocalDBDir()

	// If scan path is specified, generate the DB name for that path
	if scanPath != "" {
		// Clean the path to match how scan command does it
		cleanPath := filepath.Clean(scanPath)

		// Generate MD5 hash (same logic as persistentDB.go)
		hash := md5.Sum([]byte(cleanPath))
		hashString := fmt.Sprintf("%x", hash)
		dbName := fmt.Sprintf("slm_%s.db", hashString[:8])
		dbPath := filepath.Join(dbDir, dbName)

		// Check if this DB exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return "", fmt.Errorf("database not found for scan path %s (expected: %s)", scanPath, dbPath)
		}

		return dbPath, nil
	}

	// Find all slm_*.db files
	matches, err := filepath.Glob(filepath.Join(dbDir, "slm_*.db"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no database files found in %s", dbDir)
	}

	// Find the most recently modified database
	var mostRecentDB string
	var mostRecentTime os.FileInfo

	for _, dbPath := range matches {
		info, err := os.Stat(dbPath)
		if err != nil {
			continue
		}

		if mostRecentTime == nil || info.ModTime().After(mostRecentTime.ModTime()) {
			mostRecentDB = dbPath
			mostRecentTime = info
		}
	}

	if mostRecentDB == "" {
		return "", fmt.Errorf("could not determine most recent database")
	}

	return mostRecentDB, nil
}

// loadTitlesDB loads the titles database with multi-language support
func loadTitlesDB(configMgr *config.Manager) (*db.SwitchTitlesDB, error) {
	titleDBDir := configMgr.GetTitleDBDir()
	localePriority := []string{"KR.ko", "JP.ja", "US.en"}

	// Load all available title files in priority order
	var titleFiles []*os.File
	for _, locale := range localePriority {
		titlePath := filepath.Join(titleDBDir, fmt.Sprintf("%s.json", locale))
		if file, err := os.Open(titlePath); err == nil {
			titleFiles = append(titleFiles, file)
		}
	}

	if len(titleFiles) == 0 {
		return nil, fmt.Errorf("no title database files found")
	}
	defer func() {
		for _, file := range titleFiles {
			file.Close()
		}
	}()

	// Load versions file
	versionsPath := filepath.Join(configMgr.GetCacheDir(), settings.VERSIONS_JSON_FILENAME)
	versionsFile, err := os.Open(versionsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open versions file: %v", err)
	}
	defer versionsFile.Close()

	// Convert to []io.Reader for the function call
	var titleReaders []io.Reader
	for _, file := range titleFiles {
		titleReaders = append(titleReaders, file)
	}

	// Create title database with multi-language support
	titlesDB, err := db.CreateSwitchTitleDBMultiLang(titleReaders, versionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create title database: %v", err)
	}

	return titlesDB, nil
}

// outputListResults outputs the list results in the specified format
func outputListResults(games []*db.SwitchGameFiles, totalCount int, opts db.ListOptions, titlesDB *db.SwitchTitlesDB, configMgr *config.Manager) error {
	// Build local DB structure
	localDB := &db.LocalSwitchFilesDB{
		TitlesMap: make(map[string]*db.SwitchGameFiles),
	}

	for _, game := range games {
		if game.BaseExist {
			titleID := game.File.Metadata.TitleId
			if len(titleID) >= 4 {
				idPrefix := titleID[:len(titleID)-4]
				localDB.TitlesMap[idPrefix] = game
			}
		}
	}

	// Convert format to scanner options
	scannerOpts := &scanner.Options{
		Format:    listFormat,
		ShowTable: true, // Always show full table/output for list command
	}

	// Create scanner with config manager
	s, err := scanner.NewScanner(configMgr, scannerOpts)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %v", err)
	}

	// Output using existing scanner output logic
	err = s.OutputResults(localDB, titlesDB, nil, nil)
	if err != nil {
		return err
	}

	// Print pagination info for table format
	if listFormat == "table" && opts.Limit == 0 {
		totalPages := (totalCount + opts.PerPage - 1) / opts.PerPage
		start := (opts.Page-1)*opts.PerPage + 1
		end := start + len(games) - 1

		fmt.Printf("\nShowing %d-%d of %d games (Page %d/%d)\n", start, end, totalCount, opts.Page, totalPages)

		if opts.Page < totalPages {
			fmt.Printf("Use --page %d to see the next page\n", opts.Page+1)
		}
	}

	return nil
}
