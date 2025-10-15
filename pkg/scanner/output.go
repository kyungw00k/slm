package scanner

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/giwty/switch-library-manager/db"
	"github.com/jedib0t/go-pretty/table"
)

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
	TitleID        string   `json:"title_id"`
	Name           string   `json:"name"`
	LocalVersion   int      `json:"local_version"`
	LatestVersion  int      `json:"latest_version"`
	MissingUpdates []int    `json:"missing_updates,omitempty"`
	MissingDLC     []string `json:"missing_dlc,omitempty"`
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

		// Get game name from titles DB
		gameName := titleID
		if title, exists := titlesDB.TitlesMap[titleID]; exists {
			gameName = title.Attributes.Name
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

			// Get latest version from titles DB
			if len(title.Updates) > 0 {
				for version := range title.Updates {
					if version > missingInfo.LatestVersion {
						missingInfo.LatestVersion = version
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

func (s *Scanner) outputTable(result *ScanResult) error {
	// Games table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Title ID", "Name", "Version", "Updates", "DLC", "File Path"})

	for _, game := range result.Games {
		baseStatus := "✗"
		if game.HasBase {
			baseStatus = "✓"
		}

		t.AppendRow(table.Row{
			game.TitleID,
			game.Name,
			fmt.Sprintf("v%d %s", game.Version, baseStatus),
			strconv.Itoa(game.UpdateCount),
			strconv.Itoa(game.DLCCount),
			game.FilePath,
		})
	}

	t.SetTitle("Game Library")
	t.Render()

	// Missing content table
	if len(result.MissingContent) > 0 {
		fmt.Println()
		mt := table.NewWriter()
		mt.SetOutputMirror(os.Stdout)
		mt.AppendHeader(table.Row{"Title ID", "Name", "Local Version", "Latest Version"})

		for _, missing := range result.MissingContent {
			mt.AppendRow(table.Row{
				missing.TitleID,
				missing.Name,
				fmt.Sprintf("v%d", missing.LocalVersion),
				fmt.Sprintf("v%d", missing.LatestVersion),
			})
		}

		mt.SetTitle("Missing Content")
		mt.Render()
	}

	// Summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total Games: %d\n", result.Summary.TotalGames)
	fmt.Printf("  Games with Base: %d\n", result.Summary.GamesWithBase)
	fmt.Printf("  Games with Updates: %d\n", result.Summary.GamesWithUpdates)
	fmt.Printf("  Games with DLC: %d\n", result.Summary.GamesWithDLC)
	if result.Summary.MissingUpdates > 0 {
		fmt.Printf("  Missing Updates: %d\n", result.Summary.MissingUpdates)
	}
	if result.Summary.MissingDLC > 0 {
		fmt.Printf("  Missing DLC: %d\n", result.Summary.MissingDLC)
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