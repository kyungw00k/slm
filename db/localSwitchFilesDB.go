package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/giwty/switch-library-manager/fileio"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/giwty/switch-library-manager/switchfs"
	"github.com/karrick/godirwalk"
	"go.uber.org/zap"
)

var (
	versionRegex = regexp.MustCompile(`\[[vV]?(?P<version>[0-9]{1,10})]`)
	titleIdRegex = regexp.MustCompile(`\[(?P<titleId>[A-Z,a-z0-9]{16})]`)
)

const (
	DB_TABLE_FILE_SCAN_METADATA = "deep-scan"
	DB_TABLE_LOCAL_LIBRARY      = "local-library"

	REASON_UNSUPPORTED_TYPE = iota
	REASON_DUPLICATE
	REASON_OLD_UPDATE
	REASON_UNRECOGNISED
	REASON_MALFORMED_FILE
)

type LocalSwitchDBManager struct {
	db *PersistentDB
}

func NewLocalSwitchDBManager(baseFolder string) (*LocalSwitchDBManager, error) {
	db, err := NewPersistentDB(baseFolder)
	if err != nil {
		return nil, err
	}
	return &LocalSwitchDBManager{db: db}, nil
}

func NewLocalSwitchDBManagerWithScanPaths(baseFolder string, scanPaths []string) (*LocalSwitchDBManager, error) {
	db, err := NewPersistentDBWithScanPaths(baseFolder, scanPaths)
	if err != nil {
		return nil, err
	}
	return &LocalSwitchDBManager{db: db}, nil
}

func (ldb *LocalSwitchDBManager) Close() {
	ldb.db.Close()
}

type ExtendedFileInfo struct {
	FileName   string
	BaseFolder string
	Size       int64
	IsDir      bool
}

type SwitchFileInfo struct {
	ExtendedInfo ExtendedFileInfo
	Metadata     *switchfs.ContentMetaAttributes
}

type SwitchGameFiles struct {
	File         SwitchFileInfo
	BaseExist    bool
	Updates      map[int]SwitchFileInfo
	Dlc          map[string]SwitchFileInfo
	MultiContent bool
	LatestUpdate int
	IsSplit      bool
}

type SkippedFile struct {
	ReasonCode     int
	ReasonText     string
	AdditionalInfo string
}

// ScanErrors collects errors that occurred during scanning
type ScanErrors struct {
	FolderErrors map[string]error // folder path -> error
	FileErrors   []FileError      // individual file errors
	mu           sync.Mutex
}

// FileError represents an error that occurred while processing a file
type FileError struct {
	FilePath string
	Error    error
	Stage    string // "scan", "process", "metadata"
}

// AddFolderError records an error for a specific folder
func (se *ScanErrors) AddFolderError(folder string, err error) {
	se.mu.Lock()
	defer se.mu.Unlock()
	if se.FolderErrors == nil {
		se.FolderErrors = make(map[string]error)
	}
	se.FolderErrors[folder] = err
}

// AddFileError records an error for a specific file
func (se *ScanErrors) AddFileError(filePath, stage string, err error) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.FileErrors = append(se.FileErrors, FileError{
		FilePath: filePath,
		Error:    err,
		Stage:    stage,
	})
}

// HasErrors returns true if any errors were collected
func (se *ScanErrors) HasErrors() bool {
	se.mu.Lock()
	defer se.mu.Unlock()
	return len(se.FolderErrors) > 0 || len(se.FileErrors) > 0
}

// Summary generates a human-readable summary of errors
func (se *ScanErrors) Summary() string {
	se.mu.Lock()
	defer se.mu.Unlock()

	if !se.HasErrors() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nScan completed with %d folder errors and %d file errors\n",
		len(se.FolderErrors), len(se.FileErrors)))

	if len(se.FolderErrors) > 0 {
		sb.WriteString("\nFolder errors:\n")
		for folder, err := range se.FolderErrors {
			sb.WriteString(fmt.Sprintf("  - %s: %v\n", folder, err))
		}
	}

	if len(se.FileErrors) > 0 && len(se.FileErrors) <= 10 {
		sb.WriteString("\nFile errors:\n")
		for _, fe := range se.FileErrors {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %v\n", fe.FilePath, fe.Stage, fe.Error))
		}
	} else if len(se.FileErrors) > 10 {
		sb.WriteString(fmt.Sprintf("\n%d file errors (showing first 10):\n", len(se.FileErrors)))
		for i := 0; i < 10; i++ {
			fe := se.FileErrors[i]
			sb.WriteString(fmt.Sprintf("  - %s (%s): %v\n", fe.FilePath, fe.Stage, fe.Error))
		}
	}

	return sb.String()
}

type LocalSwitchFilesDB struct {
	TitlesMap   map[string]*SwitchGameFiles
	Skipped     map[ExtendedFileInfo]SkippedFile
	NumFiles    int
	ScanErrors  *ScanErrors // Errors encountered during scanning
}

// FileJob represents a file to be processed by a worker
type FileJob struct {
	File  ExtendedFileInfo
	Index int
}

// FileResult represents the result of processing a file
type FileResult struct {
	Index        int
	File         ExtendedFileInfo
	ContentMap   map[string]*switchfs.ContentMetaAttributes
	Error        error
	Skipped      *SkippedFile
}

func (ldb *LocalSwitchDBManager) CreateLocalSwitchFilesDB(folders []string,
	progress ProgressUpdater, recursive bool, ignoreCache bool, numWorkers int) (*LocalSwitchFilesDB, error) {

	// Default to number of CPUs if not specified
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	titles := map[string]*SwitchGameFiles{}
	skipped := map[ExtendedFileInfo]SkippedFile{}
	files := []ExtendedFileInfo{}

	if !ignoreCache {
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "files", &files)
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "skipped", &skipped)
		ldb.db.GetEntry(DB_TABLE_LOCAL_LIBRARY, "titles", &titles)
	}

	// Always scan if ignoreCache is true or if no titles found
	var scanErrors *ScanErrors
	if ignoreCache || len(titles) == 0 {
		// Reset collections for fresh scan when ignoring cache
		if ignoreCache {
			titles = map[string]*SwitchGameFiles{}
			skipped = map[ExtendedFileInfo]SkippedFile{}
			files = []ExtendedFileInfo{}
		}

		// Use parallel folder scanning with streaming pipeline
		fileCount, errors := ldb.scanFoldersParallelStreaming(folders, recursive, progress, titles, skipped, numWorkers)
		scanErrors = errors

		// Collect all files for database storage
		for _, gameFiles := range titles {
			if gameFiles.BaseExist {
				files = append(files, gameFiles.File.ExtendedInfo)
			}
			for _, update := range gameFiles.Updates {
				files = append(files, update.ExtendedInfo)
			}
			for _, dlc := range gameFiles.Dlc {
				files = append(files, dlc.ExtendedInfo)
			}
		}
		for skipFile := range skipped {
			files = append(files, skipFile)
		}

		// Update progress after all processing complete
		if progress != nil {
			progress.UpdateProgress(fileCount, fileCount, fmt.Sprintf("Complete: processed %d files across %d folders", fileCount, len(folders)))
		}

		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "files", files)
		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "skipped", skipped)
		ldb.db.AddEntry(DB_TABLE_LOCAL_LIBRARY, "titles", titles)
	}

	if progress != nil {
		progress.UpdateProgress(len(files), len(files), "Complete")
	}

	return &LocalSwitchFilesDB{
		TitlesMap:  titles,
		Skipped:    skipped,
		NumFiles:   len(files),
		ScanErrors: scanErrors,
	}, nil
}

// calculateBufferSize determines optimal channel buffer size
func calculateBufferSize(numWorkers int, folderCount int) int {
	// Base buffer: 2x workers
	base := numWorkers * 2

	// Increase for multiple folders (more concurrent producers)
	if folderCount > 1 {
		base = numWorkers * min(folderCount, 4)
	}

	// Cap at reasonable limit to prevent excessive memory use
	const maxBuffer = 1000
	return min(base, maxBuffer)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// scanFoldersParallelStreaming scans multiple folders in parallel with streaming processing
func (ldb *LocalSwitchDBManager) scanFoldersParallelStreaming(
	folders []string,
	recursive bool,
	progress ProgressUpdater,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile,
	numWorkers int) (int, *ScanErrors) {

	if len(folders) == 0 {
		return 0, &ScanErrors{}
	}

	// Initialize error collector
	scanErrors := &ScanErrors{
		FolderErrors: make(map[string]error),
		FileErrors:   make([]FileError, 0),
	}

	// Calculate optimal buffer sizes
	bufferSize := calculateBufferSize(numWorkers, len(folders))

	// Channels for streaming pipeline
	filesChan := make(chan ExtendedFileInfo, bufferSize)

	// WaitGroup for folder scanning goroutines
	var scanWg sync.WaitGroup

	// Thread-safe counters
	var totalFilesFound int64
	var processedFiles int64
	var counterMu sync.Mutex

	// Thread-safe maps
	var mapMu sync.Mutex

	// Start folder scanning goroutines
	for i, folder := range folders {
		scanWg.Add(1)
		go func(folderIdx int, folderPath string) {
			defer scanWg.Done()

			// Update progress for this folder
			if progress != nil {
				folderName := filepath.Base(folderPath)
				if len(folderName) > 50 {
					runes := []rune(folderName)
					if len(runes) > 50 {
						folderName = string(runes[:47]) + "..."
					}
				}
				progress.UpdateProgress(folderIdx, len(folders),
					fmt.Sprintf("scanning folder %d/%d: %s", folderIdx+1, len(folders), folderName))
			}

			// Scan folder and collect errors
			err := ldb.scanFolderStreaming(folderPath, recursive, filesChan, progress, folderIdx, len(folders), &totalFilesFound, &counterMu, scanErrors)
			if err != nil {
				scanErrors.AddFolderError(folderPath, err)
				zap.S().Errorf("Error scanning folder %s: %v", folderPath, err)
			}
		}(i, folder)
	}

	// Close filesChan when all scanning is done
	go func() {
		scanWg.Wait()
		close(filesChan)
	}()

	// Start worker pool for processing files
	jobs := make(chan FileJob, bufferSize)
	results := make(chan FileResult, bufferSize)

	var workerWg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		workerWg.Add(1)
		go ldb.fileWorker(w, jobs, results, &workerWg)
	}

	// Result collector goroutine
	done := make(chan bool)
	go func() {
		for result := range results {
			counterMu.Lock()
			processedFiles++
			current := processedFiles
			counterMu.Unlock()

			// Update progress
			if progress != nil && (current%10 == 0 || current == 1) {
				progress.UpdateProgress(int(current), 0, "processing:"+result.File.FileName)
			}

			// Handle skipped file
			if result.Skipped != nil {
				mapMu.Lock()
				skipped[result.File] = *result.Skipped
				mapMu.Unlock()
				continue
			}

			// Handle error
			if result.Error != nil {
				// Record the error
				filePath := filepath.Join(result.File.BaseFolder, result.File.FileName)
				scanErrors.AddFileError(filePath, "metadata", result.Error)

				mapMu.Lock()
				if _, ok := skipped[result.File]; !ok {
					skipped[result.File] = SkippedFile{
						ReasonText: "unable to determine title-Id / version - " + result.Error.Error(),
						ReasonCode: REASON_UNRECOGNISED,
					}
				}
				mapMu.Unlock()
				continue
			}

			// Process content metadata with thread safety
			if result.ContentMap != nil {
				mapMu.Lock()
				ldb.processContentMetadata(result.File, result.ContentMap, titles, skipped)
				mapMu.Unlock()
			}
		}
		done <- true
	}()

	// Stream files from scanner to workers
	go func() {
		jobIdx := 0
		for file := range filesChan {
			jobs <- FileJob{File: file, Index: jobIdx}
			jobIdx++
		}
		close(jobs)
	}()

	// Wait for workers to finish
	workerWg.Wait()
	close(results)

	// Wait for result collector
	<-done

	return int(processedFiles), scanErrors
}

// scanFolderStreaming scans a folder and streams files to a channel
func (ldb *LocalSwitchDBManager) scanFolderStreaming(
	folder string,
	recursive bool,
	filesChan chan<- ExtendedFileInfo,
	progress ProgressUpdater,
	folderIdx int,
	totalFolders int,
	totalFilesFound *int64,
	counterMu *sync.Mutex,
	scanErrors *ScanErrors) error {

	localFilesFound := 0

	err := godirwalk.Walk(folder, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			// Skip the root folder itself
			if osPathname == folder {
				return nil
			}

			// Handle directories
			if de.IsDir() {
				if !recursive && osPathname != folder {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files
			name := de.Name()
			if len(name) > 0 && name[0:1] == "." {
				return nil
			}

			// Check if this file is in a subdirectory when not recursive
			base := filepath.Dir(osPathname) + string(os.PathSeparator)
			if strings.TrimSuffix(base, string(os.PathSeparator)) != strings.TrimSuffix(folder, string(os.PathSeparator)) &&
				!recursive {
				return nil
			}

			// Early filtering: Only process supported file types
			fileName := strings.ToLower(name)
			isSplit := false

			// Check for split files
			if len(fileName) >= 2 {
				if partNum, err := strconv.Atoi(fileName[len(fileName)-2:]); err == nil {
					if partNum == 0 {
						isSplit = true
					} else {
						// Skip non-zero split parts
						return nil
					}
				}
			}

			// Only process files with supported extensions
			if !isSplit &&
				!strings.HasSuffix(fileName, ".xci") &&
				!strings.HasSuffix(fileName, ".nsp") &&
				!strings.HasSuffix(fileName, ".nsz") &&
				!strings.HasSuffix(fileName, ".xcz") {
				return nil
			}

			// Get file info for size
			info, err := os.Stat(osPathname)
			if err != nil {
				zap.S().Warnf("Failed to stat file %s: %v", osPathname, err)
				return nil
			}

			// Stream file to channel immediately
			filesChan <- ExtendedFileInfo{
				FileName:   name,
				BaseFolder: base,
				Size:       info.Size(),
				IsDir:      false,
			}

			localFilesFound++

			// Update total counter atomically
			if totalFilesFound != nil && counterMu != nil {
				counterMu.Lock()
				*totalFilesFound++
				counterMu.Unlock()
			}

			// Update progress periodically
			if progress != nil && (localFilesFound%10 == 0 || localFilesFound == 1) {
				displayName := truncateUTF8(name, 40)
				progress.UpdateProgress(localFilesFound, 0,
					fmt.Sprintf("[folder %d/%d] %s", folderIdx+1, totalFolders, displayName))
			}

			return nil
		},
		Unsorted:      true,
		ScratchBuffer: make([]byte, 64*1024),
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			// Record file stat errors
			if scanErrors != nil {
				scanErrors.AddFileError(osPathname, "scan", err)
			}
			zap.S().Errorf("Error scanning %s: %v", osPathname, err)
			return godirwalk.SkipNode
		},
	})

	return err
}

// truncateUTF8 truncates a string to maxLen runes, adding "..." if truncated
func truncateUTF8(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func scanFolder(folder string, recursive bool, files *[]ExtendedFileInfo, progress ProgressUpdater) error {
	filesFound := 0

	// Use godirwalk for better performance on network filesystems
	err := godirwalk.Walk(folder, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			// Skip the root folder itself
			if osPathname == folder {
				return nil
			}

			// Handle directories
			if de.IsDir() {
				// If not recursive, skip subdirectories
				if !recursive && osPathname != folder {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files (starting with .)
			name := de.Name()
			if len(name) > 0 && name[0:1] == "." {
				return nil
			}

			// Check if this file is in a subdirectory when not recursive
			base := filepath.Dir(osPathname) + string(os.PathSeparator)
			if strings.TrimSuffix(base, string(os.PathSeparator)) != strings.TrimSuffix(folder, string(os.PathSeparator)) &&
				!recursive {
				return nil
			}

			// Early filtering: Only add supported file types to the list
			fileName := strings.ToLower(name)
			isSplit := false

			// Check for split files (files ending with numbers like .00, .01, etc.)
			if len(fileName) >= 2 {
				if partNum, err := strconv.Atoi(fileName[len(fileName)-2:]); err == nil {
					if partNum == 0 {
						isSplit = true
					} else {
						// Skip non-zero split parts (they'll be processed with part 0)
						return nil
					}
				}
			}

			// Only add files with supported extensions
			if !isSplit &&
				!strings.HasSuffix(fileName, ".xci") &&
				!strings.HasSuffix(fileName, ".nsp") &&
				!strings.HasSuffix(fileName, ".nsz") &&
				!strings.HasSuffix(fileName, ".xcz") {
				// Skip unsupported file types immediately (don't add to memory)
				return nil
			}

			// Get file info for size
			info, err := os.Stat(osPathname)
			if err != nil {
				zap.S().Warnf("Failed to stat file %s: %v", osPathname, err)
				return nil
			}

			// Add file to list (only supported types reach here)
			*files = append(*files, ExtendedFileInfo{
				FileName:   name,
				BaseFolder: base,
				Size:       info.Size(),
				IsDir:      false,
			})
			filesFound++

			// Update progress every 10 files to minimize overhead on network filesystems
			if progress != nil && (filesFound%10 == 0 || filesFound == 1) {
				displayName := name
				// Truncate long filenames to 40 characters
				if len(displayName) > 40 {
					// Handle UTF-8 properly by converting to rune slice
					runes := []rune(displayName)
					if len(runes) > 40 {
						displayName = string(runes[:37]) + "..."
					}
				}
				progress.UpdateProgress(filesFound, 0, fmt.Sprintf("scanning: %s", displayName))
			}

			return nil
		},
		Unsorted:      true,              // Don't sort entries (faster)
		ScratchBuffer: make([]byte, 64*1024), // 64KB buffer for better network performance
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			zap.S().Errorf("Error scanning %s: %v", osPathname, err)
			return godirwalk.SkipNode
		},
	})

	// Final update when scanning completes
	if progress != nil && filesFound > 0 {
		progress.UpdateProgress(filesFound, filesFound, fmt.Sprintf("scan complete: %d files found", filesFound))
	}

	return err
}

func (ldb *LocalSwitchDBManager) ClearScanData() error {
	return ldb.db.ClearTable(DB_TABLE_FILE_SCAN_METADATA)
}

func (ldb *LocalSwitchDBManager) processLocalFiles(files []ExtendedFileInfo,
	progress ProgressUpdater,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile) {
	ind := 0
	total := len(files)
	for _, file := range files {
		ind += 1
		if progress != nil {
			progress.UpdateProgress(ind, total, "process:"+file.FileName)
		}

		//scan sub-folders if flag is present
		filePath := filepath.Join(file.BaseFolder, file.FileName)
		if file.IsDir {
			continue
		}

		fileName := strings.ToLower(file.FileName)
		isSplit := false

		if partNum, err := strconv.Atoi(fileName[len(fileName)-2:]); err == nil {
			if partNum == 0 {
				isSplit = true
			} else {
				continue
			}

		}

		//only handle NSZ and NSP files

		if !isSplit &&
			!strings.HasSuffix(fileName, "xci") &&
			!strings.HasSuffix(fileName, "nsp") &&
			!strings.HasSuffix(fileName, "nsz") &&
			!strings.HasSuffix(fileName, "xcz") {
			skipped[file] = SkippedFile{ReasonCode: REASON_UNSUPPORTED_TYPE, ReasonText: "file type is not supported"}
			continue
		}

		contentMap, err := ldb.getGameMetadata(file, filePath, skipped)

		if err != nil {
			if _, ok := skipped[file]; !ok {
				skipped[file] = SkippedFile{ReasonText: "unable to determine title-Id / version - " + err.Error(), ReasonCode: REASON_UNRECOGNISED}
			}
			continue
		}

		for _, metadata := range contentMap {

			idPrefix := metadata.TitleId[0 : len(metadata.TitleId)-4]

			multiContent := len(contentMap) > 1
			switchTitle := &SwitchGameFiles{
				MultiContent: multiContent,
				Updates:      map[int]SwitchFileInfo{},
				Dlc:          map[string]SwitchFileInfo{},
				BaseExist:    false,
				IsSplit:      isSplit,
				LatestUpdate: 0,
			}
			if t, ok := titles[idPrefix]; ok {
				switchTitle = t
			}
			titles[idPrefix] = switchTitle

			//process Updates
			if strings.HasSuffix(metadata.TitleId, "800") {
				metadata.Type = "Update"

				if update, ok := switchTitle.Updates[metadata.Version]; ok {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate update file (" + update.ExtendedInfo.FileName + ")"}
					zap.S().Warnf("-->Duplicate update file found [%v] and [%v]", update.ExtendedInfo.FileName, file.FileName)
					continue
				}
				switchTitle.Updates[metadata.Version] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				if metadata.Version > switchTitle.LatestUpdate {
					if switchTitle.LatestUpdate != 0 {
						skipped[switchTitle.Updates[switchTitle.LatestUpdate].ExtendedInfo] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally"}
					}
					switchTitle.LatestUpdate = metadata.Version
				} else {
					skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old update file, newer update exist locally"}
				}
				continue
			}

			//process base
			if strings.HasSuffix(metadata.TitleId, "000") {
				metadata.Type = "Base"
				if switchTitle.BaseExist {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate base file (" + switchTitle.File.ExtendedInfo.FileName + ")"}
					zap.S().Warnf("-->Duplicate base file found [%v] and [%v]", file.FileName, switchTitle.File.ExtendedInfo.FileName)
					continue
				}
				switchTitle.File = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
				switchTitle.BaseExist = true

				continue
			}

			if dlc, ok := switchTitle.Dlc[metadata.TitleId]; ok {
				if metadata.Version < dlc.Metadata.Version {
					skipped[file] = SkippedFile{ReasonCode: REASON_OLD_UPDATE, ReasonText: "old DLC file, newer version exist locally"}
					zap.S().Warnf("-->Old DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
					continue
				} else if metadata.Version == dlc.Metadata.Version {
					skipped[file] = SkippedFile{ReasonCode: REASON_DUPLICATE, ReasonText: "duplicate DLC file (" + dlc.ExtendedInfo.FileName + ")"}
					zap.S().Warnf("-->Duplicate DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
					continue
				}
			}
			//not an update, and not main TitleAttributes, so treat it as a DLC
			metadata.Type = "DLC"
			switchTitle.Dlc[metadata.TitleId] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
		}
	}

}

// processLocalFilesWithWorkers processes files in parallel using a worker pool
func (ldb *LocalSwitchDBManager) processLocalFilesWithWorkers(
	files []ExtendedFileInfo,
	progress ProgressUpdater,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile,
	numWorkers int) {

	total := len(files)
	if total == 0 {
		return
	}

	// Create channels
	jobs := make(chan FileJob, numWorkers*2)
	results := make(chan FileResult, numWorkers*2)

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go ldb.fileWorker(w, jobs, results, &wg)
	}

	// Start result collector
	done := make(chan bool)
	processedCount := 0
	go func() {
		for result := range results {
			processedCount++
			
			// Update progress
			if progress != nil {
				progress.UpdateProgress(processedCount, total, "process:"+result.File.FileName)
			}

			// Handle skipped file
			if result.Skipped != nil {
				skipped[result.File] = *result.Skipped
				continue
			}

			// Handle error
			if result.Error != nil {
				if _, ok := skipped[result.File]; !ok {
					skipped[result.File] = SkippedFile{
						ReasonText: "unable to determine title-Id / version - " + result.Error.Error(),
						ReasonCode: REASON_UNRECOGNISED,
					}
				}
				continue
			}

			// Process content metadata
			if result.ContentMap != nil {
				ldb.processContentMetadata(result.File, result.ContentMap, titles, skipped)
			}
		}
		done <- true
	}()

	// Send jobs
	for i, file := range files {
		jobs <- FileJob{File: file, Index: i}
	}
	close(jobs)

	// Wait for workers
	wg.Wait()
	close(results)

	// Wait for collector
	<-done
}

// fileWorker processes files from the job queue
func (ldb *LocalSwitchDBManager) fileWorker(id int, jobs <-chan FileJob, results chan<- FileResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		result := FileResult{
			Index: job.Index,
			File:  job.File,
		}

		file := job.File
		filePath := filepath.Join(file.BaseFolder, file.FileName)

		// Skip directories
		if file.IsDir {
			continue
		}

		fileName := strings.ToLower(file.FileName)
		isSplit := false

		// Check for split files
		if partNum, err := strconv.Atoi(fileName[len(fileName)-2:]); err == nil {
			if partNum == 0 {
				isSplit = true
			} else {
				continue
			}
		}

		// Only handle NSZ, NSP, XCI, XCZ files
		if !isSplit &&
			!strings.HasSuffix(fileName, "xci") &&
			!strings.HasSuffix(fileName, "nsp") &&
			!strings.HasSuffix(fileName, "nsz") &&
			!strings.HasSuffix(fileName, "xcz") {
			result.Skipped = &SkippedFile{
				ReasonCode: REASON_UNSUPPORTED_TYPE,
				ReasonText: "file type is not supported",
			}
			results <- result
			continue
		}

		// Get metadata (this is the I/O intensive part)
		tempSkipped := make(map[ExtendedFileInfo]SkippedFile)
		contentMap, err := ldb.getGameMetadata(file, filePath, tempSkipped)

		if err != nil {
			if skippedInfo, ok := tempSkipped[file]; ok {
				result.Skipped = &skippedInfo
			} else {
				result.Error = err
			}
			results <- result
			continue
		}

		result.ContentMap = contentMap
		results <- result
	}
}

// processContentMetadata processes the content metadata and updates titles/skipped maps
func (ldb *LocalSwitchDBManager) processContentMetadata(
	file ExtendedFileInfo,
	contentMap map[string]*switchfs.ContentMetaAttributes,
	titles map[string]*SwitchGameFiles,
	skipped map[ExtendedFileInfo]SkippedFile) {

	for _, metadata := range contentMap {
		idPrefix := metadata.TitleId[0 : len(metadata.TitleId)-4]

		multiContent := len(contentMap) > 1
		switchTitle := &SwitchGameFiles{
			MultiContent: multiContent,
			Updates:      map[int]SwitchFileInfo{},
			Dlc:          map[string]SwitchFileInfo{},
			BaseExist:    false,
			IsSplit:      false,
			LatestUpdate: 0,
		}
		if t, ok := titles[idPrefix]; ok {
			switchTitle = t
		}
		titles[idPrefix] = switchTitle

		// Process Updates
		if strings.HasSuffix(metadata.TitleId, "800") {
			metadata.Type = "Update"

			if update, ok := switchTitle.Updates[metadata.Version]; ok {
				skipped[file] = SkippedFile{
					ReasonCode: REASON_DUPLICATE,
					ReasonText: "duplicate update file (" + update.ExtendedInfo.FileName + ")",
				}
				zap.S().Warnf("-->Duplicate update file found [%v] and [%v]", update.ExtendedInfo.FileName, file.FileName)
				continue
			}
			switchTitle.Updates[metadata.Version] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
			if metadata.Version > switchTitle.LatestUpdate {
				if switchTitle.LatestUpdate != 0 {
					skipped[switchTitle.Updates[switchTitle.LatestUpdate].ExtendedInfo] = SkippedFile{
						ReasonCode: REASON_OLD_UPDATE,
						ReasonText: "old update file, newer update exist locally",
					}
				}
				switchTitle.LatestUpdate = metadata.Version
			} else {
				skipped[file] = SkippedFile{
					ReasonCode: REASON_OLD_UPDATE,
					ReasonText: "old update file, newer update exist locally",
				}
			}
			continue
		}

		// Process base
		if strings.HasSuffix(metadata.TitleId, "000") {
			metadata.Type = "Base"
			if switchTitle.BaseExist {
				skipped[file] = SkippedFile{
					ReasonCode: REASON_DUPLICATE,
					ReasonText: "duplicate base file (" + switchTitle.File.ExtendedInfo.FileName + ")",
				}
				zap.S().Warnf("-->Duplicate base file found [%v] and [%v]", file.FileName, switchTitle.File.ExtendedInfo.FileName)
				continue
			}
			switchTitle.File = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
			switchTitle.BaseExist = true
			continue
		}

		// Process DLC
		if dlc, ok := switchTitle.Dlc[metadata.TitleId]; ok {
			if metadata.Version < dlc.Metadata.Version {
				skipped[file] = SkippedFile{
					ReasonCode: REASON_OLD_UPDATE,
					ReasonText: "old DLC file, newer version exist locally",
				}
				zap.S().Warnf("-->Old DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
				continue
			} else if metadata.Version == dlc.Metadata.Version {
				skipped[file] = SkippedFile{
					ReasonCode: REASON_DUPLICATE,
					ReasonText: "duplicate DLC file (" + dlc.ExtendedInfo.FileName + ")",
				}
				zap.S().Warnf("-->Duplicate DLC file found [%v] and [%v]", file.FileName, dlc.ExtendedInfo.FileName)
				continue
			}
		}
		// Not an update, and not main title, so treat it as a DLC
		metadata.Type = "DLC"
		switchTitle.Dlc[metadata.TitleId] = SwitchFileInfo{ExtendedInfo: file, Metadata: metadata}
	}
}

func (ldb *LocalSwitchDBManager) getGameMetadata(file ExtendedFileInfo,
	filePath string,
	skipped map[ExtendedFileInfo]SkippedFile) (map[string]*switchfs.ContentMetaAttributes, error) {

	var metadata map[string]*switchfs.ContentMetaAttributes = nil
	keys, _ := settings.SwitchKeys()
	var err error
	fileKey := filePath + "|" + file.FileName + "|" + strconv.Itoa(int(file.Size))
	if keys != nil && keys.GetKey("header_key") != "" {
		err = ldb.db.GetEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, &metadata)

		if err != nil {
			zap.S().Warnf("%v", err)
		}

		if metadata != nil {
			return metadata, nil
		}

		fileName := strings.ToLower(file.FileName)
		if strings.HasSuffix(fileName, "nsp") ||
			strings.HasSuffix(fileName, "nsz") {
			metadata, err = switchfs.ReadNspMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "xci") ||
			strings.HasSuffix(fileName, "xcz") {
			metadata, err = switchfs.ReadXciMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read NSP [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read file [reason: %v]\n", file.FileName, err)
			}
		} else if strings.HasSuffix(fileName, "00") {
			metadata, err = fileio.ReadSplitFileMetadata(filePath)
			if err != nil {
				skipped[file] = SkippedFile{ReasonCode: REASON_MALFORMED_FILE, ReasonText: fmt.Sprintf("failed to read split files [reason: %v]", err)}
				zap.S().Errorf("[file:%v] failed to read NSP [reason: %v]\n", file.FileName, err)
			}
		}
	}

	if metadata != nil {
		err = ldb.db.AddEntry(DB_TABLE_FILE_SCAN_METADATA, fileKey, metadata)

		if err != nil {
			zap.S().Warnf("%v", err)
		}
		return metadata, nil
	}

	//fallback to parse data from filename

	//parse title id
	titleId, _ := parseTitleIdFromFileName(file.FileName)
	version, _ := parseVersionFromFileName(file.FileName)

	if titleId == nil || version == nil {
		return nil, errors.New("unable to determine titileId / version")
	}
	metadata = map[string]*switchfs.ContentMetaAttributes{}
	metadata[*titleId] = &switchfs.ContentMetaAttributes{TitleId: *titleId, Version: *version}

	return metadata, nil
}

func parseVersionFromFileName(fileName string) (*int, error) {
	res := versionRegex.FindStringSubmatch(fileName)
	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no version id found")
	}
	ver, err := strconv.Atoi(res[1])
	if err != nil {
		return nil, errors.New("failed to parse name - no version id found")
	}
	return &ver, nil
}

func parseTitleIdFromFileName(fileName string) (*string, error) {
	res := titleIdRegex.FindStringSubmatch(fileName)

	if len(res) != 2 {
		return nil, errors.New("failed to parse name - no title id found")
	}
	titleId := strings.ToLower(res[1])
	return &titleId, nil
}

func ParseTitleNameFromFileName(fileName string) string {
	ind := strings.Index(fileName, "[")
	if ind != -1 {
		return fileName[:ind]
	}
	return fileName
}
