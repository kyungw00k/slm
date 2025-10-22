package scanner

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// mapToKoreanTitle maps common English game titles to Korean
func mapToKoreanTitle(englishTitle string) string {
	// Clean up common suffixes
	englishTitle = strings.TrimSuffix(englishTitle, " Base")
	englishTitle = strings.TrimSuffix(englishTitle, " – Nintendo Switch 2 Edition")
	englishTitle = strings.TrimSpace(englishTitle)

	// Korean title mappings for popular games
	koreanTitles := map[string]string{
		"The Legend of Zelda Tears of the Kingdom": "젤다의 전설 티어스 오브 더 킹덤",
		"The Legend of Zelda Breath of the Wild":   "젤다의 전설 브레스 오브 더 와일드",
		"Super Mario Odyssey":                      "슈퍼 마리오 오디세이",
		"Super Mario Bros Wonder":                  "슈퍼 마리오브라더스 원더",
		"Pokemon Scarlet":                          "포켓몬스터 스칼렛",
		"Pokemon Violet":                           "포켓몬스터 바이올렛",
		"Animal Crossing New Horizons":             "모여봐요 동물의 숲",
		"Little Nightmares II":                     "리틀 나이트메어 2",
	}

	if korean, exists := koreanTitles[englishTitle]; exists {
		return korean
	}

	return englishTitle // Return original if no mapping found
}

// OutputFormat represents different output formats
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// GameInfo represents game information for output
type GameInfo struct {
	TitleID     string `json:"title_id"`
	Name        string `json:"name"`
	Version     int    `json:"version"`
	HasBase     bool   `json:"has_base"`
	UpdateCount int    `json:"update_count"`
	DLCCount    int    `json:"dlc_count"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
}

// MissingContentInfo represents missing content information
type MissingContentInfo struct {
	TitleID          string   `json:"title_id"`
	Name             string   `json:"name"`
	LocalVersion     int      `json:"local_version"`
	LatestVersion    int      `json:"latest_version"`
	LatestUpdateDate string   `json:"latest_update_date,omitempty"`
	MissingUpdates   []int    `json:"missing_updates,omitempty"`
	MissingDLC       []string `json:"missing_dlc,omitempty"`
}

// ScanResult represents the complete scan result
type ScanResult struct {
	Games          []GameInfo           `json:"games"`
	MissingContent []MissingContentInfo `json:"missing_content,omitempty"`
	Summary        ScanSummary          `json:"summary"`
}

// ScanSummary represents scan statistics
type ScanSummary struct {
	TotalGames       int `json:"total_games"`
	GamesWithBase    int `json:"games_with_base"`
	GamesWithUpdates int `json:"games_with_updates"`
	GamesWithDLC     int `json:"games_with_dlc"`
	MissingUpdates   int `json:"missing_updates"`
	MissingDLC       int `json:"missing_dlc"`
}

// OutputResults formats and outputs scan results
func (s *Scanner) OutputResults(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, missingUpdates map[string]interface{}, missingDLC map[string]interface{}) error {
	result := s.buildScanResult(localDB, titlesDB, missingUpdates, missingDLC)

	// If ShowTable is false, output summary only
	if !s.options.ShowTable {
		return s.outputSummary(result, localDB)
	}

	// Otherwise output full table/json/csv
	switch OutputFormat(s.options.Format) {
	case FormatTable:
		return s.outputTable(result)
	case FormatJSON:
		return s.outputJSON(result)
	case FormatCSV:
		return s.outputCSV(result)
	default:
		return fmt.Errorf("unsupported output format: %s", s.options.Format)
	}
}

func (s *Scanner) buildScanResult(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, missingUpdates, missingDLC map[string]interface{}) *ScanResult {
	result := &ScanResult{
		Games:          make([]GameInfo, 0),
		MissingContent: make([]MissingContentInfo, 0),
	}

	// Build game information
	for titleID, gameFiles := range localDB.TitlesMap {
		if !gameFiles.BaseExist {
			continue
		}

		// Get game name - try multiple sources in priority order
		gameName := titleID // fallback

		// 1. Try to get from NACP metadata using configured locale priority
		if gameFiles.File.Metadata.Ncap != nil {
			// Get locale priority from config (fallback to default if not available)
			localePriority := s.settings.LocalePriority
			if len(localePriority) == 0 {
				localePriority = []string{"KR.ko", "US.en", "JP.ja"}
			}
			if name := gameFiles.File.Metadata.Ncap.GetBestTitleName(localePriority); name != "" {
				gameName = name
			}
		}

		// 2. Try to get from title database
		if gameName == titleID && titlesDB != nil && titlesDB.TitlesMap != nil {
			if title, exists := titlesDB.TitlesMap[titleID]; exists && title.Attributes.Name != "" {
				gameName = title.Attributes.Name
			}
		}

		// 3. If nothing worked, try filename parsing
		if gameName == titleID {
			fileName := gameFiles.File.ExtendedInfo.FileName
			extracted := db.ParseTitleNameFromFileName(fileName)
			if extracted != "" {
				// Try to map to Korean titles for common games
				gameName = mapToKoreanTitle(extracted)
			}
		}

		gameInfo := GameInfo{
			TitleID:     titleID,
			Name:        gameName,
			HasBase:     gameFiles.BaseExist,
			UpdateCount: len(gameFiles.Updates),
			DLCCount:    len(gameFiles.Dlc),
			FilePath:    filepath.Join(gameFiles.File.ExtendedInfo.BaseFolder, gameFiles.File.ExtendedInfo.FileName),
			FileSize:    gameFiles.File.ExtendedInfo.Size,
		}

		// Get latest version
		if gameFiles.File.Metadata != nil {
			gameInfo.Version = int(gameFiles.File.Metadata.Version)
		}

		result.Games = append(result.Games, gameInfo)
		result.Summary.TotalGames++

		if gameFiles.BaseExist {
			result.Summary.GamesWithBase++
		}
		if len(gameFiles.Updates) > 0 {
			result.Summary.GamesWithUpdates++
		}
		if len(gameFiles.Dlc) > 0 {
			result.Summary.GamesWithDLC++
		}
	}

	// Build missing content information
	for titleID, _ := range missingUpdates {
		if title, exists := titlesDB.TitlesMap[titleID]; exists {
			missingInfo := MissingContentInfo{
				TitleID: titleID,
				Name:    title.Attributes.Name,
			}

			// Get local version
			if gameFiles, exists := localDB.TitlesMap[titleID]; exists && gameFiles.BaseExist && gameFiles.File.Metadata != nil {
				missingInfo.LocalVersion = int(gameFiles.File.Metadata.Version)
			}

			// Get latest version and update date from titles DB
			if len(title.Updates) > 0 {
				for version, updateDate := range title.Updates {
					if version > missingInfo.LatestVersion {
						missingInfo.LatestVersion = version
						missingInfo.LatestUpdateDate = updateDate
					}
				}
			}

			result.MissingContent = append(result.MissingContent, missingInfo)
			result.Summary.MissingUpdates++
		}
	}

	for titleID, _ := range missingDLC {
		// Add to existing missing content entry or create new one
		found := false
		for i := range result.MissingContent {
			if result.MissingContent[i].TitleID == titleID {
				// Add DLC info to existing entry
				found = true
				break
			}
		}

		if !found {
			if title, exists := titlesDB.TitlesMap[titleID]; exists {
				missingInfo := MissingContentInfo{
					TitleID: titleID,
					Name:    title.Attributes.Name,
				}
				result.MissingContent = append(result.MissingContent, missingInfo)
			}
		}
		result.Summary.MissingDLC++
	}

	return result
}

// truncateString truncates a string to maxLen characters (considering UTF-8 runes)
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// truncatePath truncates a file path for display
func truncatePath(path string, maxLen int) string {
	if utf8.RuneCountInString(path) <= maxLen {
		return path
	}

	// Get filename
	fileName := filepath.Base(path)

	// If filename alone is too long, just show end of filename
	if utf8.RuneCountInString(fileName) > maxLen-4 {
		runes := []rune(fileName)
		return "..." + string(runes[len(runes)-(maxLen-3):])
	}

	// Show .../ + filename
	return ".../" + fileName
}

func (s *Scanner) outputTable(result *ScanResult) error {
	// Games table with old console.go style (80 column width)
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)

	// Configure column widths for 80 column display
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMax: 3, Align: text.AlignRight},  // #
		{Number: 2, WidthMax: 20},                         // Title
		{Number: 3, WidthMax: 12},                         // TitleId
		{Number: 4, WidthMax: 8},                          // Version
		{Number: 5, WidthMax: 7, Align: text.AlignCenter}, // Updates
		{Number: 6, WidthMax: 5, Align: text.AlignCenter}, // DLC
		{Number: 7, WidthMax: 24},                         // File Path
	})

	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Version", "Updates", "DLC", "File Path"})

	for i, game := range result.Games {
		baseStatus := "✗"
		if game.HasBase {
			baseStatus = "✓"
		}

		t.AppendRow([]interface{}{
			i,
			truncateString(game.Name, 20),
			truncateString(game.TitleID, 12),
			fmt.Sprintf("v%d %s", game.Version, baseStatus),
			strconv.Itoa(game.UpdateCount),
			strconv.Itoa(game.DLCCount),
			truncatePath(game.FilePath, 24),
		})
	}

	t.AppendFooter(table.Row{"", "", "", "", "", "Total", len(result.Games)})
	t.Render()

	// Missing updates table
	if len(result.MissingContent) > 0 {
		hasUpdates := false
		hasDLC := false

		for _, missing := range result.MissingContent {
			if missing.LatestVersion > missing.LocalVersion {
				hasUpdates = true
			}
			if len(missing.MissingDLC) > 0 {
				hasDLC = true
			}
		}

		if hasUpdates {
			fmt.Print("\nFound available updates:\n\n")
			mt := table.NewWriter()
			mt.SetOutputMirror(os.Stdout)
			mt.SetStyle(table.StyleLight)

			// Configure column widths
			mt.SetColumnConfigs([]table.ColumnConfig{
				{Number: 1, WidthMax: 3, Align: text.AlignRight},   // #
				{Number: 2, WidthMax: 20},                          // Title
				{Number: 3, WidthMax: 12},                          // TitleId
				{Number: 4, WidthMax: 10, Align: text.AlignCenter}, // Local ver
				{Number: 5, WidthMax: 10, Align: text.AlignCenter}, // Latest ver
				{Number: 6, WidthMax: 12},                          // Date
			})

			mt.AppendHeader(table.Row{"#", "Title", "TitleId", "Local ver", "Latest ver", "Update Date"})

			i := 0
			for _, missing := range result.MissingContent {
				if missing.LatestVersion > missing.LocalVersion {
					mt.AppendRow([]interface{}{
						i,
						truncateString(missing.Name, 20),
						truncateString(missing.TitleID, 12),
						missing.LocalVersion,
						missing.LatestVersion,
						missing.LatestUpdateDate,
					})
					i++
				}
			}

			mt.AppendFooter(table.Row{"", "", "", "", "Total", i})
			mt.Render()
		}

		if hasDLC {
			fmt.Print("\nFound missing DLCs:\n\n")
			dt := table.NewWriter()
			dt.SetOutputMirror(os.Stdout)
			dt.SetStyle(table.StyleLight)

			// Configure column widths
			dt.SetColumnConfigs([]table.ColumnConfig{
				{Number: 1, WidthMax: 3, Align: text.AlignRight}, // #
				{Number: 2, WidthMax: 20},                        // Title
				{Number: 3, WidthMax: 12},                        // TitleId
				{Number: 4, WidthMax: 40},                        // Missing DLCs
			})

			dt.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs"})

			i := 0
			for _, missing := range result.MissingContent {
				if len(missing.MissingDLC) > 0 {
					dt.AppendRow([]interface{}{
						i,
						truncateString(missing.Name, 20),
						truncateString(missing.TitleID, 12),
						strings.Join(missing.MissingDLC, "\n"),
					})
					i++
				}
			}

			dt.AppendFooter(table.Row{"", "", "", "", "Total", i})
			dt.Render()
		}
	}

	// Summary in old style format
	fmt.Printf("\n")
	if result.Summary.TotalGames > 0 {
		fmt.Printf("Library status: %d games", result.Summary.TotalGames)
		if result.Summary.GamesWithUpdates > 0 {
			fmt.Printf(" (%d with updates)", result.Summary.GamesWithUpdates)
		}
		if result.Summary.GamesWithDLC > 0 {
			fmt.Printf(" (%d with DLC)", result.Summary.GamesWithDLC)
		}
		fmt.Printf("\n")
	}

	if result.Summary.MissingUpdates > 0 || result.Summary.MissingDLC > 0 {
		fmt.Printf("Missing content: %d updates, %d DLC\n", result.Summary.MissingUpdates, result.Summary.MissingDLC)
	} else if result.Summary.TotalGames > 0 {
		fmt.Printf("All NSP's are up to date!\n")
	}

	return nil
}

func (s *Scanner) outputJSON(result *ScanResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func (s *Scanner) outputCSV(result *ScanResult) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{
		"Title ID", "Name", "Version", "Has Base", "Update Count", "DLC Count", "File Path", "File Size",
	}); err != nil {
		return err
	}

	// Write game data
	for _, game := range result.Games {
		record := []string{
			game.TitleID,
			game.Name,
			strconv.Itoa(game.Version),
			strconv.FormatBool(game.HasBase),
			strconv.Itoa(game.UpdateCount),
			strconv.Itoa(game.DLCCount),
			game.FilePath,
			strconv.FormatInt(game.FileSize, 10),
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// outputSummary outputs a concise summary of the scan results
func (s *Scanner) outputSummary(result *ScanResult, localDB *db.LocalSwitchFilesDB) error {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("SCAN SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Count different types of games
	baseGames := 0
	updatesCount := 0
	dlcCount := 0

	for _, game := range result.Games {
		if game.HasBase {
			baseGames++
		}
		updatesCount += game.UpdateCount
		dlcCount += game.DLCCount
	}

	// Display basic statistics
	fmt.Printf("Total games:     %d\n", result.Summary.TotalGames)
	fmt.Printf("  - Base games:  %d\n", baseGames)
	fmt.Printf("  - Updates:     %d\n", updatesCount)
	fmt.Printf("  - DLC:         %d\n", dlcCount)
	fmt.Println()

	// Display missing content if any
	if result.Summary.MissingUpdates > 0 || result.Summary.MissingDLC > 0 {
		fmt.Println("Missing content:")
		if result.Summary.MissingUpdates > 0 {
			fmt.Printf("  - Updates:     %d games\n", result.Summary.MissingUpdates)
		}
		if result.Summary.MissingDLC > 0 {
			fmt.Printf("  - DLC:         %d games\n", result.Summary.MissingDLC)
		}
		fmt.Println()
	} else if result.Summary.TotalGames > 0 {
		fmt.Println("All games are up to date!")
		fmt.Println()
	}

	// Display scan statistics
	fmt.Printf("Scanned files:   %d\n", localDB.NumFiles)
	if len(localDB.Skipped) > 0 {
		fmt.Printf("Skipped files:   %d\n", len(localDB.Skipped))
	}

	// Display errors if any
	if localDB.ScanErrors != nil && localDB.ScanErrors.HasErrors() {
		fmt.Println()
		fmt.Printf("Errors encountered: %d folder errors, %d file errors\n",
			len(localDB.ScanErrors.FolderErrors),
			len(localDB.ScanErrors.FileErrors))
	}

	// Display database path
	dbPath := filepath.Join(s.config.GetLocalDBDir(), "scan_*.db")
	fmt.Println()
	fmt.Printf("Database path:   %s\n", dbPath)
	fmt.Println()

	// Hint about list command
	if result.Summary.TotalGames > 0 {
		fmt.Println(strings.Repeat("-", 80))
		fmt.Println("To view detailed game list, use:")
		fmt.Println("  slm list                    # Show first 50 games")
		fmt.Println("  slm list --missing-updates  # Show games missing updates")
		fmt.Println("  slm list --title zelda      # Search for specific games")
		fmt.Println("  slm scan --show-table       # Re-scan and show full table")
		fmt.Println(strings.Repeat("-", 80))
	}

	return nil
}

// Helper functions to expose scanner internals for list command
func (s *Scanner) SetOptions(opts *Options) {
	s.options = opts
}

func (s *Scanner) SetSettings(appSettings *settings.AppSettings) {
	s.settings = appSettings
}
