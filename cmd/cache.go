package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/pkg/config"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/spf13/cobra"
)

var (
	cleanCache  bool
	updateCache bool
	cachePath   bool
)

// cacheCmd represents the cache command
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage cache files",
	Long: `Manage SLM cache files including title databases and local databases.

Usage patterns:
  slm cache           # Show cache status
  slm cache --clean   # Clean all cache files
  slm cache --update  # Force update title databases
  slm cache --path    # Show cache directory path

Examples:
  slm cache
  slm cache --clean
  slm cache --update`,
	RunE: runCache,
}

func init() {
	rootCmd.AddCommand(cacheCmd)

	cacheCmd.Flags().BoolVar(&cleanCache, "clean", false, "clean cache files")
	cacheCmd.Flags().BoolVar(&updateCache, "update", false, "force update title databases")
	cacheCmd.Flags().BoolVar(&cachePath, "path", false, "show cache directory path")

	// Make flags mutually exclusive
	cacheCmd.MarkFlagsMutuallyExclusive("clean", "update", "path")
}

func runCache(cmd *cobra.Command, args []string) error {
	configDir := getConfigDir()
	cacheDir := filepath.Join(configDir, "cache")

	// Handle --path flag
	if cachePath {
		fmt.Println(cacheDir)
		return nil
	}

	// Handle --clean flag
	if cleanCache {
		return cleanCacheFiles(cacheDir)
	}

	// Handle --update flag
	if updateCache {
		return updateCacheFiles(cacheDir)
	}

	// Default: show cache status
	return showCacheStatus(cacheDir)
}

func showCacheStatus(cacheDir string) error {
	fmt.Printf("Cache Directory: %s\n", cacheDir)

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("Status: No cache directory found")
		return nil
	}

	totalSize := int64(0)
	fileCount := 0

	// Walk through cache directory
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan cache directory: %v", err)
	}

	fmt.Printf("Total Size: %s\n", formatSize(totalSize))
	fmt.Printf("Total Files: %d\n", fileCount)

	// Show title database info
	titleDBDir := filepath.Join(cacheDir, "titledb")
	if info, err := os.Stat(titleDBDir); err == nil && info.IsDir() {
		titleDBFiles, titleDBSize := countFiles(titleDBDir)
		fmt.Printf("\nTitle Databases:\n")
		fmt.Printf("  Files: %d\n", titleDBFiles)
		fmt.Printf("  Size: %s\n", formatSize(titleDBSize))

		// List individual title DB files
		entries, err := os.ReadDir(titleDBDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					info, err := entry.Info()
					if err == nil {
						fmt.Printf("    %s: %s\n", entry.Name(), formatSize(info.Size()))
					}
				}
			}
		}
	}

	// Show local database info
	dbDir := filepath.Join(cacheDir, "db")
	if info, err := os.Stat(dbDir); err == nil && info.IsDir() {
		dbFiles, dbSize := countFiles(dbDir)
		fmt.Printf("\nLocal Databases:\n")
		fmt.Printf("  Files: %d\n", dbFiles)
		fmt.Printf("  Size: %s\n", formatSize(dbSize))

		// List individual DB files with details
		entries, err := os.ReadDir(dbDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".db") {
					info, err := entry.Info()
					if err == nil {
						fmt.Printf("    %s: %s (modified: %s)\n",
							entry.Name(),
							formatSize(info.Size()),
							info.ModTime().Format("2006-01-02 15:04:05"))
					}
				}
			}
		}
	}

	// Show versions file info
	versionsFile := filepath.Join(cacheDir, "versions.json")
	if info, err := os.Stat(versionsFile); err == nil {
		fmt.Printf("\nVersions Database:\n")
		fmt.Printf("  Size: %s\n", formatSize(info.Size()))
		fmt.Printf("  Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func cleanCacheFiles(cacheDir string) error {
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("No cache directory found - nothing to clean")
		return nil
	}

	// Count files before deletion
	fileCount := 0
	totalSize := int64(0)
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
			totalSize += info.Size()
		}
		return nil
	})

	// Remove cache directory
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clean cache: %v", err)
	}

	fmt.Printf("Cache cleaned successfully\n")
	fmt.Printf("Removed %d files (%s)\n", fileCount, formatSize(totalSize))
	return nil
}

func updateCacheFiles(cacheDir string) error {
	fmt.Printf("Updating cache files in %s\n", cacheDir)

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %v", err)
	}

	// Create config manager to access settings
	configMgr := config.NewManager(getConfigDir())
	appSettings, err := configMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %v", err)
	}

	startTime := time.Now()
	fmt.Println("🔄 Downloading latest databases...")

	// Ensure titledb directory exists
	titleDBDir := filepath.Join(cacheDir, "titledb")
	if err := os.MkdirAll(titleDBDir, 0755); err != nil {
		return fmt.Errorf("failed to create titledb directory: %v", err)
	}

	updatedCount := 0

	// Download versions.json
	fmt.Print("  📋 Updating versions database... ")
	versionsPath := filepath.Join(cacheDir, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, versionsPath, appSettings.VersionsEtag)
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		if versionsEtag != appSettings.VersionsEtag {
			appSettings.VersionsEtag = versionsEtag
			updatedCount++
			fmt.Println("✅ Updated")
		} else {
			fmt.Println("✅ Already up to date")
		}
		versionsFile.Close()
	}

	// Download title databases for each locale
	if appSettings.TitlesETags == nil {
		appSettings.TitlesETags = make(map[string]string)
	}

	for _, locale := range appSettings.LocalePriority {
		fmt.Printf("  🌍 Updating %s title database... ", locale)

		url := appSettings.GetTitleDBURL(locale)
		titlePath := filepath.Join(titleDBDir, locale+".json")

		etag := ""
		if appSettings.TitlesETags != nil {
			etag = appSettings.TitlesETags[locale]
		}

		titleFile, newEtag, err := db.LoadAndUpdateFile(url, titlePath, etag)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			if newEtag != etag {
				appSettings.TitlesETags[locale] = newEtag
				updatedCount++
				fmt.Println("✅ Updated")
			} else {
				fmt.Println("✅ Already up to date")
			}
			titleFile.Close()
		}
	}

	// Save updated settings
	if err := configMgr.Save(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save settings: %v\n", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\n🎉 Cache update completed in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("📊 Updated %d databases\n", updatedCount)

	// Show updated cache status
	fmt.Println("\nUpdated cache status:")
	return showCacheStatus(cacheDir)
}

// Helper functions
func countFiles(dir string) (int, int64) {
	count := 0
	size := int64(0)

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
			size += info.Size()
		}
		return nil
	})

	return count, size
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}