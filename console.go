package main

import (
	"flag"
	"fmt"
	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/process"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/jedib0t/go-pretty/table"
	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	nspFolder   = flag.String("f", "", "path to NSP folder")
	recursive   = flag.Bool("r", true, "recursively scan sub folders")
	mode        = flag.String("m", "", "**deprecated**")
	progressBar *progressbar.ProgressBar
)

type Console struct {
	baseFolder  string
	sugarLogger *zap.SugaredLogger
}

func CreateConsole(baseFolder string, sugarLogger *zap.SugaredLogger) *Console {
	return &Console{baseFolder: baseFolder, sugarLogger: sugarLogger}
}

func (c *Console) Start() {
	flag.Parse()

	if mode != nil && *mode != "" {
		fmt.Println("note : the mode option ('-m') is deprecated, please use the settings.json to control options.")
	}

	settingsObj := settings.ReadSettings(c.baseFolder)

	// Download versions.json first (shared across all languages)
	fmt.Printf("Downloading versions.json and multiple language title files\n")
	progressBar = progressbar.New(1 + len(settingsObj.LocalePriority))

	filename := filepath.Join(c.baseFolder, settings.VERSIONS_JSON_FILENAME)
	versionsFile, versionsEtag, err := db.LoadAndUpdateFile(settings.VERSIONS_JSON_URL, filename, settingsObj.VersionsEtag)
	if err != nil {
		fmt.Printf("version json file doesn't exist\n")
		return
	}
	settingsObj.VersionsEtag = versionsEtag
	progressBar.Add(1)

	// Download multiple language title files
	titleFiles := make(map[string]*os.File)
	for _, locale := range settingsObj.LocalePriority {
		url := settingsObj.GetTitleDBURL(locale)
		filename = filepath.Join(c.baseFolder, locale+".json")

		etag := ""
		if settingsObj.TitlesETags != nil {
			etag = settingsObj.TitlesETags[locale]
		}

		titleFile, newEtag, err := db.LoadAndUpdateFile(url, filename, etag)
			if err != nil {
				fmt.Printf("Warning: Failed to download %s titles: %v\n", locale, err)
			} else {
				titleFiles[locale] = titleFile
				if settingsObj.TitlesETags == nil {
					settingsObj.TitlesETags = make(map[string]string)
				}
				settingsObj.TitlesETags[locale] = newEtag
				fmt.Printf("Downloaded %s titles\n", locale)
			}
		}
		progressBar.Add(1)
	}

	// If no title files were downloaded, exit with error
	if len(titleFiles) == 0 {
		fmt.Printf("No title files could be downloaded\n")
		return
	}

	progressBar.Finish()
	newUpdate, err := settings.CheckForUpdates()

	if newUpdate {
		fmt.Printf("\n=== New version available, download from Github ===\n")
	}

	//3. update the config file with new etag
	settings.SaveSettings(settingsObj, c.baseFolder)

	//4. create switch title db
	// Use title file according to locale priority
	var primaryTitleFile *os.File
	for _, locale := range settingsObj.LocalePriority {
		if titleFile, exists := titleFiles[locale]; exists {
			primaryTitleFile = titleFile
			fmt.Printf("Using %s titledb as primary source\n", locale)
			break
		}
	}

	if primaryTitleFile == nil {
		fmt.Printf("No title files could be downloaded\n")
		return
	}

	titlesDB, err := db.CreateSwitchTitleDB(primaryTitleFile, versionsFile)

	//5. read local files
	folderToScan := settingsObj.Folder
	if nspFolder != nil && *nspFolder != "" {
		folderToScan = *nspFolder
	}

	if folderToScan == "" {
		fmt.Printf("\n\nNo folder to scan was defined, please edit settings.json with the folder path\n")
		return
	}
	fmt.Printf("\n\nScanning folder [%v]", folderToScan)
	progressBar = progressbar.New(2000)
	keys, _ := settings.InitSwitchKeys(c.baseFolder)
	if keys == nil || keys.GetKey("header_key") == "" {
		fmt.Printf("\n!!NOTE!!: keys file was not found, deep scan is disabled, library will be based on file tags.\n %v", err)
	}

	recursiveMode := settingsObj.ScanRecursively
	if recursive != nil && *recursive != true {
		recursiveMode = *recursive
	}

	scanFolders := settingsObj.ScanFolders
	scanFolders = append(scanFolders, folderToScan)

	localDbManager, err := db.NewLocalSwitchDBManagerWithScanPaths(c.baseFolder, scanFolders)
	if err != nil {
		fmt.Printf("failed to create local files db :%v\n", err)
		return
	}
	defer localDbManager.Close()

	localDB, err := localDbManager.CreateLocalSwitchFilesDB(scanFolders, c, recursiveMode, true)
	if err != nil {
		fmt.Printf("\nfailed to process local folder\n %v", err)
		return
	}
	progressBar.Finish()

	p := (float32(len(localDB.TitlesMap)) / float32(len(titlesDB.TitlesMap))) * 100

	fmt.Printf("Local library completion status: %.2f%% (have %d titles, out of %d titles)\n", p, len(localDB.TitlesMap), len(titlesDB.TitlesMap))

	c.processIssues(localDB)

	if settingsObj.OrganizeOptions.DeleteOldUpdateFiles {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nDeleting old updates\n")
		process.DeleteOldUpdates(c.baseFolder, localDB, c)
		progressBar.Finish()
	}

	if settingsObj.OrganizeOptions.RenameFiles || settingsObj.OrganizeOptions.CreateFolderPerGame {
		progressBar = progressbar.New(2000)
		fmt.Printf("\nStarting library organization\n")
		results := process.OrganizeByFolders(folderToScan, localDB, titlesDB, c)

		// If dry run mode, show results
		if results != nil && settingsObj.OrganizeOptions.DryRun {
			c.displayDryRunResults(results)
			return
		}

		progressBar.Finish()
	}

	if settingsObj.CheckForMissingUpdates {
		fmt.Printf("\nChecking for missing updates\n")
		c.processMissingUpdates(localDB, titlesDB)
	}

	if settingsObj.CheckForMissingDLC {
		fmt.Printf("\nChecking for missing DLC\n")
		c.processMissingDLC(localDB, titlesDB)
	}

	fmt.Printf("Completed")
}

func (c *Console) processIssues(localDB *db.LocalSwitchFilesDB) {
	if len(localDB.Skipped) != 0 {
		fmt.Print("\nSkipped files:\n\n")
	} else {
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Skipped file", "Reason"})
	i := 0
	for k, v := range localDB.Skipped {
		t.AppendRow([]interface{}{i, path.Join(k.BaseFolder, k.FileName), v})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(localDB.Skipped)})
	t.Render()
}

func (c *Console) processMissingUpdates(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {
	incompleteTitles := process.ScanForMissingUpdates(localDB.TitlesMap, titlesDB.TitlesMap)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound available updates:\n\n")
	} else {
		fmt.Print("\nAll NSP's are up to date!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Local version", "Latest Version", "Update Date"})
	i := 0
	for _, v := range incompleteTitles {
		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, v.LocalUpdate, v.LatestUpdate, v.LatestUpdateDate})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()
}

func (c *Console) processMissingDLC(localDB *db.LocalSwitchFilesDB, titlesDB *db.SwitchTitlesDB) {
	settingsObj := settings.ReadSettings(c.baseFolder)
	ignoreIds := map[string]struct{}{}
	for _, id := range settingsObj.IgnoreDLCTitleIds {
		ignoreIds[strings.ToLower(id)] = struct{}{}
	}
	incompleteTitles := process.ScanForMissingDLC(localDB.TitlesMap, titlesDB.TitlesMap, ignoreIds)
	if len(incompleteTitles) != 0 {
		fmt.Print("\nFound missing DLCS:\n\n")
	} else {
		fmt.Print("\nYou have all the DLCS!\n\n")
		return
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.AppendHeader(table.Row{"#", "Title", "TitleId", "Missing DLCs (titleId - Name)"})
	i := 0
	for _, v := range incompleteTitles {
		t.AppendRow([]interface{}{i, v.Attributes.Name, v.Attributes.Id, strings.Join(v.MissingDLC, "\n")})
		i++
	}
	t.AppendFooter(table.Row{"", "", "", "", "Total", len(incompleteTitles)})
	t.Render()
}

func (c *Console) UpdateProgress(curr int, total int, message string) {
	progressBar.ChangeMax(total)
	progressBar.Set(curr)

}

func (c *Console) displayDryRunResults(results *process.OrganizeResults) {
	fmt.Printf("\n🔍 DRY RUN MODE - No files will be moved\n")
	fmt.Printf("==================================================\n\n")

	if len(results.DryRunResults) == 0 {
		fmt.Printf("✅ No changes needed - all files are already organized correctly!\n\n")
		return
	}

	fmt.Printf("📊 Summary: %d files, %d folders would be affected\n\n", results.TotalFiles, results.TotalFolders)

	// Group results by game
	gameResults := make(map[string][]process.DryRunResult)
	for _, result := range results.DryRunResults {
		if _, exists := gameResults[result.GameName]; !exists {
			gameResults[result.GameName] = []process.DryRunResult{}
		}
		gameResults[result.GameName] = append(gameResults[result.GameName], result)
	}

	// Display results by game
	for gameName, gameFiles := range gameResults {
		fmt.Printf("🎮 %s\n", gameName)
		fmt.Printf("   ────────────────────────────────────────\n")

		for _, result := range gameFiles {
			switch result.Action {
			case "create":
				fmt.Printf("   📁 CREATE: %s\n", result.To)
			case "move":
				fromName := filepath.Base(result.From)
				toName := filepath.Base(result.To)
				if fromName != toName {
					fmt.Printf("   📄 RENAME: %s → %s\n", fromName, toName)
				} else {
					fmt.Printf("   📂 MOVE:   %s → %s\n", filepath.Dir(result.From), filepath.Dir(result.To))
				}
			}
		}
		fmt.Printf("\n")
	}

	fmt.Printf("💡 To apply these changes, set \"dry_run\": false in organize_options\n\n")
}
