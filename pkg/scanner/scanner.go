package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/pkg/config"
	"github.com/giwty/switch-library-manager/pkg/performance"
	"github.com/giwty/switch-library-manager/pkg/progress"
	"github.com/giwty/switch-library-manager/pkg/templates"
	"github.com/giwty/switch-library-manager/process"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

// progressUpdaterAdapter adapts the old ProgressUpdater interface
type progressUpdaterAdapter struct {
	updateFunc func(current, total int, message string)
}

func (p *progressUpdaterAdapter) UpdateProgress(current, total int, message string) {
	p.updateFunc(current, total, message)
}

// Options represents scan configuration
type Options struct {
	Folders           []string
	Recursive         bool
	Format            string
	Locale            string
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

	// Performance options
	MaxWorkers        int
	MemoryLimit       int
	EnableProfiling   bool
}

// Scanner handles the scanning process
type Scanner struct {
	config         *config.Manager
	settings       *settings.AppSettings
	logger         *zap.SugaredLogger
	options        *Options
	missingUpdates map[string]interface{}
	missingDLC     map[string]interface{}
}

// NewScanner creates a new scanner
func NewScanner(configMgr *config.Manager, opts *Options) (*Scanner, error) {
	settings, err := configMgr.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %v", err)
	}

	logger := zap.S()

	return &Scanner{
		config:   configMgr,
		settings: settings,
		logger:   logger,
		options:  opts,
	}, nil
}

// Run executes the scan process with progress tracking
func (s *Scanner) Run() error {
	stages := []progress.Stage{
		{Name: "Downloading databases", Description: "Download title and version databases"},
		{Name: "Scanning files", Description: "Scan game files in specified folders"},
		{Name: "Building library", Description: "Process and categorize found games"},
	}

	if !s.options.NoCheck {
		stages = append(stages, progress.Stage{Name: "Checking missing content", Description: "Check for missing updates and DLC"})
	}

	if s.options.Rename || s.options.CreateFolders || s.options.DeleteOldUpdates {
		stages = append(stages, progress.Stage{Name: "Organizing files", Description: "Rename and organize game files"})
		stages = append(stages, progress.Stage{Name: "Updating database", Description: "Update database with new file paths"})
	}

	mode := progress.ParseOutputMode(s.options.OutputMode)
	return progress.RunWithProgressMode(stages, mode, s.runScanProcess)
}

func (s *Scanner) runScanProcess(updater progress.ProgressUpdater) error {
	// Initialize missing content maps
	s.missingUpdates = make(map[string]interface{})
	s.missingDLC = make(map[string]interface{})

	// Stage 1: Download databases
	updater.UpdateProgress(0, 0, 0, "Downloading version and title databases...", "Starting database downloads")

	titlesDB, err := s.downloadDatabases(updater)
	if err != nil {
		return fmt.Errorf("failed to download databases: %v", err)
	}
	updater.CompleteStage(0)

	// Stage 2: Scan files
	updater.UpdateProgress(1, 0, 0, "Scanning game files...", "Initializing file scanner")

	localDB, localDbManager, err := s.scanFiles(updater)
	if err != nil {
		return fmt.Errorf("failed to scan files: %v", err)
	}
	defer localDbManager.Close()
	updater.CompleteStage(1)

	// Stage 3: Build library
	updater.UpdateProgress(2, 0, 0, "Building game library...", "Processing game metadata")

	gameLibrary, err := s.buildLibrary(localDB, titlesDB, updater)
	if err != nil {
		return fmt.Errorf("failed to build library: %v", err)
	}
	updater.CompleteStage(2)

	currentStage := 3

	// Stage 4: Check missing content (if enabled)
	if !s.options.NoCheck {
		updater.UpdateProgress(currentStage, 0, 0, "Checking for missing content...", "Analyzing game library")

		if err := s.checkMissingContent(gameLibrary, titlesDB, updater); err != nil {
			return fmt.Errorf("failed to check missing content: %v", err)
		}
		updater.CompleteStage(currentStage)
		currentStage++
	}

	// Stage 5 & 6: Organize files (if enabled)
	if s.options.Rename || s.options.CreateFolders || s.options.DeleteOldUpdates {
		updater.UpdateProgress(currentStage, 0, 0, "Organizing files...", "Starting file organization")

		if err := s.organizeFiles(localDB, titlesDB, updater); err != nil {
			return fmt.Errorf("failed to organize files: %v", err)
		}
		updater.CompleteStage(currentStage)
		currentStage++

		// Update database with new paths
		updater.UpdateProgress(currentStage, 0, 0, "Updating database...", "Syncing file path changes to database")

		if err := s.updateDatabasePaths(localDB, updater); err != nil {
			return fmt.Errorf("failed to update database: %v", err)
		}
		updater.CompleteStage(currentStage)
	}

	// Save updated settings
	if err := s.config.Save(); err != nil {
		s.logger.Warnf("Failed to save settings: %v", err)
	}

	// Output results
	if err := s.OutputResults(localDB, titlesDB, s.missingUpdates, s.missingDLC); err != nil {
		return fmt.Errorf("failed to output results: %v", err)
	}

	return nil
}

func (s *Scanner) downloadDatabases(updater progress.ProgressUpdater) (*db.SwitchTitlesDB, error) {
	// Ensure cache directories exist
	titleDBDir := s.config.GetTitleDBDir()
	if err := os.MkdirAll(titleDBDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create title DB directory: %v", err)
	}

	totalDownloads := 1 + len(s.settings.LocalePriority) // versions + title files
	currentDownload := 0

	// Download versions.json
	updater.UpdateProgress(0, currentDownload, totalDownloads, "Downloading versions database...", "Fetching versions.json")

	versionsPath := filepath.Join(s.config.GetCacheDir(), settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, versionsPath, s.settings.VersionsEtag)
	if err != nil {
		return nil, fmt.Errorf("failed to download versions: %v", err)
	}
	s.settings.VersionsEtag = versionsEtag
	currentDownload++

	// Download title files for each locale
	titleFiles := make(map[string]*os.File)
	for _, locale := range s.settings.LocalePriority {
		updater.UpdateProgress(0, currentDownload, totalDownloads,
			fmt.Sprintf("Downloading %s title database...", locale),
			fmt.Sprintf("Fetching %s.json", locale))

		url := s.settings.GetTitleDBURL(locale)
		titlePath := filepath.Join(titleDBDir, locale+".json")

		etag := ""
		if s.settings.TitlesETags != nil {
			etag = s.settings.TitlesETags[locale]
		}

		titleFile, newEtag, err := db.LoadAndUpdateFile(url, titlePath, etag)
		if err != nil {
			s.logger.Warnf("Failed to download %s titles: %v", locale, err)
		} else {
			titleFiles[locale] = titleFile
			if s.settings.TitlesETags == nil {
				s.settings.TitlesETags = make(map[string]string)
			}
			s.settings.TitlesETags[locale] = newEtag
		}
		currentDownload++
	}

	if len(titleFiles) == 0 {
		return nil, fmt.Errorf("no title files could be downloaded")
	}

	// Use primary title file according to locale priority
	var primaryTitleFile *os.File
	var primaryLocale string
	for _, locale := range s.settings.LocalePriority {
		if titleFile, exists := titleFiles[locale]; exists {
			primaryTitleFile = titleFile
			primaryLocale = locale
			break
		}
	}

	if primaryTitleFile == nil {
		return nil, fmt.Errorf("no primary title file available")
	}

	updater.UpdateProgress(0, totalDownloads, totalDownloads,
		fmt.Sprintf("Creating title database using %s locale...", primaryLocale),
		"Building internal title database")

	// Create title database
	titlesDB, err := db.CreateSwitchTitleDB(primaryTitleFile, versionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create title database: %v", err)
	}

	// Close files
	for _, file := range titleFiles {
		file.Close()
	}
	versionsFile.Close()

	return titlesDB, nil
}

func (s *Scanner) scanFiles(updater progress.ProgressUpdater) (*db.LocalSwitchFilesDB, *db.LocalSwitchDBManager, error) {
	// Initialize keys
	keys, err := settings.InitSwitchKeys(s.config.GetConfigDir())
	if err != nil || keys == nil || keys.GetKey("header_key") == "" {
		s.logger.Warn("Keys file not found, deep scan disabled - library will be based on file tags")
	}

	// Ensure local DB directory exists
	localDBDir := s.config.GetLocalDBDir()
	if err := os.MkdirAll(localDBDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create local DB directory: %v", err)
	}

	// Create database manager
	localDbManager, err := db.NewLocalSwitchDBManagerWithScanPaths(localDBDir, s.options.Folders)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create local DB manager: %v", err)
	}

	// Create progress updater that adapts to the old interface
	progressUpdaterAdapter := &progressUpdaterAdapter{
		updateFunc: func(current, total int, message string) {
			details := make([]string, 0)
			if current >= 0 && total > 0 {
				updater.UpdateProgress(1, current, total,
					fmt.Sprintf("Processing file %d of %d", current+1, total), details...)
			} else {
				updater.UpdateProgress(1, 0, 0, "Discovering files...", details...)
			}
		},
	}

	// Determine number of workers adaptively
	var numWorkers int
	if s.options.MaxWorkers > 0 {
		// User specified worker count - use it
		numWorkers = s.options.MaxWorkers
		s.logger.Infof("Using user-specified worker count: %d", numWorkers)
	} else {
		// Auto-detect optimal worker count based on environment
		env, err := performance.DetectEnvironment(s.options.Folders[0])
		if err != nil {
			// Fallback to CPU count on error
			numWorkers = runtime.NumCPU()
			s.logger.Warnf("Failed to detect environment, using CPU count: %d", numWorkers)
		} else {
			sysRes, _ := performance.GetSystemResources()
			numWorkers = performance.DetermineOptimalWorkers(env, sysRes)
			s.logger.Infof("Auto-detected environment: %s (%s), optimal workers: %d (CPU cores: %d)", 
				env.MountType, env.FileSystem, numWorkers, sysRes.CPUCores)
		}
	}

	// Scan files with worker pool
	localDB, err := localDbManager.CreateLocalSwitchFilesDB(s.options.Folders, progressUpdaterAdapter, s.options.Recursive, true, numWorkers)
	if err != nil {
		localDbManager.Close()
		return nil, nil, fmt.Errorf("failed to scan files: %v", err)
	}

	return localDB, localDbManager, nil
}

func (s *Scanner) buildLibrary(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, updater progress.ProgressUpdater) (*db.LocalSwitchFilesDB, error) {
	// The library is already built during scanning process
	// This stage could be used for additional processing if needed

	totalGames := len(localDB.TitlesMap)
	updater.UpdateProgress(2, totalGames, totalGames,
		fmt.Sprintf("Found %d games in library", totalGames),
		fmt.Sprintf("Library contains %d unique titles", totalGames))

	return localDB, nil
}

func (s *Scanner) checkMissingContent(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, updater progress.ProgressUpdater) error {
	// Apply check options from settings and command line
	checkUpdates := s.settings.CheckForMissingUpdates
	checkDLC := s.settings.CheckForMissingDLC

	if s.options.CheckUpdates {
		checkUpdates = true
		checkDLC = false
	} else if s.options.CheckDLC {
		checkUpdates = false
		checkDLC = true
	} else if s.options.NoCheck {
		return nil // Skip checking entirely
	}

	// Use existing missing content processors
	var missingUpdates, missingDLC map[string]interface{}

	if checkUpdates {
		missingUpdatesResult := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap)
		missingUpdates = make(map[string]interface{})
		for k, v := range missingUpdatesResult {
			missingUpdates[k] = v
		}

		if len(missingUpdates) > 0 {
			updater.UpdateProgress(3, 0, 0,
				fmt.Sprintf("Found %d missing updates", len(missingUpdates)),
				"Missing updates detected")
		}
	}

	if checkDLC {
		// Convert ignore list to map
		ignoreDLCMap := make(map[string]struct{})
		for _, id := range s.settings.IgnoreDLCTitleIds {
			ignoreDLCMap[id] = struct{}{}
		}

		missingDLCResult := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap, ignoreDLCMap)
		missingDLC = make(map[string]interface{})
		for k, v := range missingDLCResult {
			missingDLC[k] = v
		}

		if len(missingDLC) > 0 {
			updater.UpdateProgress(3, 0, 0,
				fmt.Sprintf("Found %d missing DLC", len(missingDLC)),
				"Missing DLC detected")
		}
	}

	// Store results for output formatting
	s.missingUpdates = missingUpdates
	s.missingDLC = missingDLC

	return nil
}

func (s *Scanner) organizeFiles(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB, updater progress.ProgressUpdater) error {
	// Configure organize options based on command line flags
	organizeOpts := s.settings.OrganizeOptions
	organizeOpts.RenameFiles = s.options.Rename
	organizeOpts.CreateFolderPerGame = s.options.CreateFolders
	organizeOpts.DeleteOldUpdateFiles = s.options.DeleteOldUpdates
	organizeOpts.DryRun = s.options.DryRun

	// Handle template options
	if s.options.Template != "" {
		template, err := templates.ParseInlineTemplate(s.options.Template)
		if err != nil {
			return fmt.Errorf("failed to parse template: %v", err)
		}

		// Validate template
		if err := templates.ValidateTemplate(template.Folder, template.File); err != nil {
			return fmt.Errorf("invalid template: %v", err)
		}

		// Apply template to organize options
		templates.ApplyTemplateToOptions(template, &organizeOpts)
	}

	// Create progress updater for organize process
	organizeProgressUpdater := &progressUpdaterAdapter{
		updateFunc: func(current, total int, message string) {
			if current >= 0 && total > 0 {
				displayMessage := "Organizing files..."
				if organizeOpts.DryRun {
					displayMessage = "Previewing file organization..."
				}
				updater.UpdateProgress(4, current, total, displayMessage)
			}
		},
	}

	// Run transactional organize process for each folder
	for i, folder := range s.options.Folders {
		folderMsg := fmt.Sprintf("Processing folder %d/%d: %s", i+1, len(s.options.Folders), folder)
		updater.UpdateProgress(4, i, len(s.options.Folders), folderMsg)

		// Update settings with current organize options
		s.settings.OrganizeOptions = organizeOpts

		// Run transactional organize process with rollback capability
		results := process.OrganizeByFoldersTransactional(folder, localDB, titlesDB, organizeProgressUpdater)
		if results == nil {
			return fmt.Errorf("failed to organize folder %s", folder)
		}

		// Report results
		if organizeOpts.DryRun {
			updater.UpdateProgress(4, i+1, len(s.options.Folders),
				fmt.Sprintf("Would organize %d files, %d folders in %s", results.TotalFiles, results.TotalFolders, folder))
		} else {
			if results.Transaction != nil && results.Transaction.IsCommitted() {
				updater.UpdateProgress(4, i+1, len(s.options.Folders),
					fmt.Sprintf("Successfully organized %d files, %d folders in %s", results.TotalFiles, results.TotalFolders, folder))
			} else {
				return fmt.Errorf("transaction failed for folder %s", folder)
			}
		}
	}

	return nil
}

func (s *Scanner) updateDatabasePaths(localDB *db.LocalSwitchFilesDB, updater progress.ProgressUpdater) error {
	// If organize was run and not in dry-run mode, we need to update the database
	// with the new file paths after organization

	if s.options.DryRun {
		updater.UpdateProgress(5, 1, 1, "Dry run mode - no database updates needed")
		return nil
	}

	// Re-scan the organized folders to get updated file paths
	updater.UpdateProgress(5, 0, 3, "Re-scanning folders for path updates...")

	// Create a temporary database manager for re-scanning
	localDBDir := s.config.GetLocalDBDir()
	tempDbManager, err := db.NewLocalSwitchDBManagerWithScanPaths(localDBDir, s.options.Folders)
	if err != nil {
		return fmt.Errorf("failed to create temp DB manager: %v", err)
	}
	defer tempDbManager.Close()

	updater.UpdateProgress(5, 1, 3, "Scanning updated file paths...")

	// Create a simple progress updater for the re-scan
	reScanProgressUpdater := &progressUpdaterAdapter{
		updateFunc: func(current, total int, message string) {
			// Update progress for the re-scan operation
			if total > 0 {
				progress := float64(current) / float64(total)
				updater.UpdateProgress(5, 1+int(progress), 3,
					fmt.Sprintf("Re-scanning files: %d/%d", current, total))
			}
		},
	}

	// Perform the re-scan to get updated paths
	// Use same number of workers as initial scan
	numWorkers := s.options.MaxWorkers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	newLocalDB, err := tempDbManager.CreateLocalSwitchFilesDB(s.options.Folders, reScanProgressUpdater, s.options.Recursive, false, numWorkers)
	if err != nil {
		return fmt.Errorf("failed to re-scan for path updates: %v", err)
	}

	updater.UpdateProgress(5, 2, 3, "Synchronizing database entries...")

	// Sync the database with new paths
	if err := s.synchronizeDatabasePaths(localDB, newLocalDB, updater); err != nil {
		return fmt.Errorf("failed to synchronize database paths: %v", err)
	}

	updater.UpdateProgress(5, 3, 3, "Database paths synchronized successfully")
	return nil
}

// synchronizeDatabasePaths updates the original database with new file paths
func (s *Scanner) synchronizeDatabasePaths(originalDB, newDB *db.LocalSwitchFilesDB, updater progress.ProgressUpdater) error {
	syncCount := 0

	// Create a map of file contents to new paths for efficient lookup
	contentToNewPath := make(map[string]string)
	for _, gameFiles := range newDB.TitlesMap {
		if gameFiles.BaseExist {
			// Use a combination of title ID and file size as a key
			if gameFiles.File.Metadata != nil {
				key := fmt.Sprintf("%s_%d", gameFiles.File.Metadata.TitleId, gameFiles.File.ExtendedInfo.Size)
				filePath := filepath.Join(gameFiles.File.ExtendedInfo.BaseFolder, gameFiles.File.ExtendedInfo.FileName)
				contentToNewPath[key] = filePath
			}
		}

		// Also map updates
		for version, updateFile := range gameFiles.Updates {
			if updateFile.Metadata != nil {
				key := fmt.Sprintf("%s_%d_%d", updateFile.Metadata.TitleId, updateFile.ExtendedInfo.Size, version)
				filePath := filepath.Join(updateFile.ExtendedInfo.BaseFolder, updateFile.ExtendedInfo.FileName)
				contentToNewPath[key] = filePath
			}
		}

		// And DLC
		for dlcId, dlcFile := range gameFiles.Dlc {
			if dlcFile.Metadata != nil {
				key := fmt.Sprintf("%s_%d_%s", dlcFile.Metadata.TitleId, dlcFile.ExtendedInfo.Size, dlcId)
				filePath := filepath.Join(dlcFile.ExtendedInfo.BaseFolder, dlcFile.ExtendedInfo.FileName)
				contentToNewPath[key] = filePath
			}
		}
	}

	// Update paths in the original database
	for titleId, gameFiles := range originalDB.TitlesMap {
		// Update base game path
		if gameFiles.BaseExist && gameFiles.File.Metadata != nil {
			key := fmt.Sprintf("%s_%d", gameFiles.File.Metadata.TitleId, gameFiles.File.ExtendedInfo.Size)
			if newPath, exists := contentToNewPath[key]; exists {
				currentPath := filepath.Join(gameFiles.File.ExtendedInfo.BaseFolder, gameFiles.File.ExtendedInfo.FileName)
				if currentPath != newPath {
					s.logger.Infof("Updating path for %s: %s -> %s", titleId, currentPath, newPath)
					// Update the ExtendedInfo fields
					gameFiles.File.ExtendedInfo.BaseFolder = filepath.Dir(newPath)
					gameFiles.File.ExtendedInfo.FileName = filepath.Base(newPath)
					syncCount++
				}
			}
		}

		// Update update file paths
		for version, updateFile := range gameFiles.Updates {
			if updateFile.Metadata != nil {
				key := fmt.Sprintf("%s_%d_%d", updateFile.Metadata.TitleId, updateFile.ExtendedInfo.Size, version)
				if newPath, exists := contentToNewPath[key]; exists {
					currentPath := filepath.Join(updateFile.ExtendedInfo.BaseFolder, updateFile.ExtendedInfo.FileName)
					if currentPath != newPath {
						s.logger.Infof("Updating update path for %s v%d: %s -> %s", titleId, version, currentPath, newPath)
						// Update the ExtendedInfo fields
						updateFile.ExtendedInfo.BaseFolder = filepath.Dir(newPath)
						updateFile.ExtendedInfo.FileName = filepath.Base(newPath)
						gameFiles.Updates[version] = updateFile
						syncCount++
					}
				}
			}
		}

		// Update DLC file paths
		for dlcId, dlcFile := range gameFiles.Dlc {
			if dlcFile.Metadata != nil {
				key := fmt.Sprintf("%s_%d_%s", dlcFile.Metadata.TitleId, dlcFile.ExtendedInfo.Size, dlcId)
				if newPath, exists := contentToNewPath[key]; exists {
					currentPath := filepath.Join(dlcFile.ExtendedInfo.BaseFolder, dlcFile.ExtendedInfo.FileName)
					if currentPath != newPath {
						s.logger.Infof("Updating DLC path for %s (%s): %s -> %s", titleId, dlcId, currentPath, newPath)
						// Update the ExtendedInfo fields
						dlcFile.ExtendedInfo.BaseFolder = filepath.Dir(newPath)
						dlcFile.ExtendedInfo.FileName = filepath.Base(newPath)
						gameFiles.Dlc[dlcId] = dlcFile
						syncCount++
					}
				}
			}
		}
	}

	s.logger.Infof("Synchronized %d file paths in database", syncCount)
	return nil
}