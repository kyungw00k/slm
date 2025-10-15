package process

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/giwty/switch-library-manager/db"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
)

// TransactionalOrganizeResults extends OrganizeResults with transaction info
type TransactionalOrganizeResults struct {
	*OrganizeResults
	Transaction     *Transaction `json:"transaction,omitempty"`
	TransactionSize int          `json:"transaction_size"`
}

// OrganizeByFoldersTransactional organizes files with transaction support and rollback capability
func OrganizeByFoldersTransactional(baseFolder string,
	localDB *db.LocalSwitchFilesDB,
	titlesDB *db.SwitchTitlesDB,
	updateProgress db.ProgressUpdater) *TransactionalOrganizeResults {

	options := settings.ReadSettings(baseFolder).OrganizeOptions
	if !IsOptionsValid(options) {
		zap.S().Error("the organize options in settings.json are not valid, please check that the template contains file/folder name")
		return nil
	}

	results := &TransactionalOrganizeResults{
		OrganizeResults: &OrganizeResults{
			DryRunResults: []DryRunResult{},
			TotalFiles:    0,
			TotalFolders:  0,
		},
	}

	// Create transaction
	transaction := NewTransaction()
	results.Transaction = transaction

	// Phase 1: Plan all operations (similar to dry run)
	if err := planOrganizeOperations(baseFolder, localDB, titlesDB, options, transaction, updateProgress); err != nil {
		zap.S().Errorf("Failed to plan organize operations: %v", err)
		return results
	}

	results.TransactionSize = transaction.Size()

	// If this is a dry run, just return the planned operations
	if options.DryRun {
		// Convert transaction operations to dry run results
		for _, op := range transaction.GetOperations() {
			dryRunResult := DryRunResult{
				From:        op.From,
				To:          op.To,
				Type:        string(op.Type),
				GameName:    op.GameName,
				Action:      string(op.Type),
				IsDirectory: op.Type == OpCreate,
			}
			results.DryRunResults = append(results.DryRunResults, dryRunResult)

			if op.Type == OpCreate {
				results.TotalFolders++
			} else {
				results.TotalFiles++
			}
		}
		return results
	}

	// Phase 2: Execute transaction
	if updateProgress != nil {
		updateProgress.UpdateProgress(0, transaction.Size(), "Executing file organization transaction...")
	}

	if err := transaction.Execute(); err != nil {
		zap.S().Errorf("Transaction failed: %v", err)
		return results
	}

	// Phase 3: Delete old updates if requested
	if options.DeleteOldUpdateFiles {
		if updateProgress != nil {
			updateProgress.UpdateProgress(transaction.Size(), transaction.Size()+1, "Deleting old update files...")
		}
		DeleteOldUpdates(baseFolder, localDB, updateProgress)
	}

	// Phase 4: Delete empty folders if requested
	if options.DeleteEmptyFolders {
		if updateProgress != nil {
			updateProgress.UpdateProgress(transaction.Size()+1, transaction.Size()+2, "Deleting empty folders...")
		}
		if err := deleteEmptyFolders(baseFolder); err != nil {
			zap.S().Errorf("Failed to delete empty folders: %v", err)
		}
	}

	if updateProgress != nil {
		updateProgress.UpdateProgress(transaction.Size()+2, transaction.Size()+2, "Organization completed successfully")
	}

	// Convert successful operations to results
	for _, op := range transaction.GetOperations() {
		if op.Type == OpCreate {
			results.TotalFolders++
		} else {
			results.TotalFiles++
		}
	}

	return results
}

// planOrganizeOperations plans all file operations without executing them
func planOrganizeOperations(baseFolder string,
	localDB *db.LocalSwitchFilesDB,
	titlesDB *db.SwitchTitlesDB,
	options settings.OrganizeOptions,
	transaction *Transaction,
	updateProgress db.ProgressUpdater) error {

	i := 0
	tasksSize := len(localDB.TitlesMap)

	for titleId, gameFiles := range localDB.TitlesMap {
		i++
		if !gameFiles.BaseExist {
			continue
		}

		if updateProgress != nil {
			updateProgress.UpdateProgress(i, tasksSize, fmt.Sprintf("Planning operations for: %s", gameFiles.File.ExtendedInfo.FileName))
		}

		titleName := getTitleName(titlesDB.TitlesMap[titleId], gameFiles)

		templateData := map[string]string{}
		templateData[settings.TEMPLATE_TITLE_ID] = gameFiles.File.Metadata.TitleId
		templateData[settings.TEMPLATE_TITLE_NAME] = titleName
		templateData[settings.TEMPLATE_VERSION_TXT] = ""
		if title, ok := titlesDB.TitlesMap[titleId]; ok {
			templateData[settings.TEMPLATE_REGION] = title.Attributes.Region
		}
		templateData[settings.TEMPLATE_VERSION] = "0"

		if gameFiles.File.Metadata.Ncap != nil {
			templateData[settings.TEMPLATE_VERSION_TXT] = gameFiles.File.Metadata.Ncap.DisplayVersion
		}

		var destinationPath = gameFiles.File.ExtendedInfo.BaseFolder

		// Plan folder creation if needed
		if options.CreateFolderPerGame {
			folderToCreate := getFolderName(options, templateData)
			destinationPath = filepath.Join(baseFolder, folderToCreate)
			transaction.AddCreateOperation(destinationPath, titleName)
		}

		// Plan base game file operations
		if err := planGameFileOperations(gameFiles, destinationPath, options, templateData, titleName, transaction); err != nil {
			return fmt.Errorf("failed to plan operations for %s: %v", titleName, err)
		}

		// Plan update file operations
		if err := planUpdateFileOperations(gameFiles, baseFolder, options, templateData, titleName, transaction); err != nil {
			return fmt.Errorf("failed to plan update operations for %s: %v", titleName, err)
		}

		// Plan DLC file operations
		if err := planDLCFileOperations(gameFiles, baseFolder, titlesDB.TitlesMap[titleId], options, templateData, titleName, transaction); err != nil {
			return fmt.Errorf("failed to plan DLC operations for %s: %v", titleName, err)
		}
	}

	return nil
}

// planGameFileOperations plans operations for base game files
func planGameFileOperations(gameFiles *db.SwitchGameFiles, destinationPath string, options settings.OrganizeOptions, templateData map[string]string, titleName string, transaction *Transaction) error {
	if gameFiles.IsSplit {
		// Handle split files
		files, err := ioutil.ReadDir(gameFiles.File.ExtendedInfo.BaseFolder)
		if err != nil {
			return err
		}

		for _, file := range files {
			if _, err := strconv.Atoi(file.Name()[len(file.Name())-1:]); err == nil {
				from := filepath.Join(gameFiles.File.ExtendedInfo.BaseFolder, file.Name())
				to := filepath.Join(destinationPath, file.Name())
				transaction.AddMoveOperation(from, to, titleName)
			}
		}
	} else {
		// Handle regular files
		from := filepath.Join(gameFiles.File.ExtendedInfo.BaseFolder, gameFiles.File.ExtendedInfo.FileName)
		to := filepath.Join(destinationPath, getFileName(options, gameFiles.File.ExtendedInfo.FileName, templateData))
		transaction.AddMoveOperation(from, to, titleName)
	}

	return nil
}

// planUpdateFileOperations plans operations for update files
func planUpdateFileOperations(gameFiles *db.SwitchGameFiles, baseFolder string, options settings.OrganizeOptions, templateData map[string]string, titleName string, transaction *Transaction) error {
	for version, updateInfo := range gameFiles.Updates {
		updateTemplateData := make(map[string]string)
		for k, v := range templateData {
			updateTemplateData[k] = v
		}

		if updateInfo.Metadata != nil {
			updateTemplateData[settings.TEMPLATE_TITLE_ID] = updateInfo.Metadata.TitleId
		}
		updateTemplateData[settings.TEMPLATE_VERSION] = strconv.Itoa(version)
		updateTemplateData[settings.TEMPLATE_TYPE] = "UPD"
		if updateInfo.Metadata.Ncap != nil {
			updateTemplateData[settings.TEMPLATE_VERSION_TXT] = updateInfo.Metadata.Ncap.DisplayVersion
		} else {
			updateTemplateData[settings.TEMPLATE_VERSION_TXT] = ""
		}

		from := filepath.Join(updateInfo.ExtendedInfo.BaseFolder, updateInfo.ExtendedInfo.FileName)
		var to string

		if options.CreateFolderPerGame {
			folderToCreate := getFolderName(options, updateTemplateData)
			destinationPath := filepath.Join(baseFolder, folderToCreate)
			transaction.AddCreateOperation(destinationPath, titleName)
			to = filepath.Join(destinationPath, getFileName(options, updateInfo.ExtendedInfo.FileName, updateTemplateData))
		} else {
			to = filepath.Join(updateInfo.ExtendedInfo.BaseFolder, getFileName(options, updateInfo.ExtendedInfo.FileName, updateTemplateData))
		}

		transaction.AddMoveOperation(from, to, titleName)
	}

	return nil
}

// planDLCFileOperations plans operations for DLC files
func planDLCFileOperations(gameFiles *db.SwitchGameFiles, baseFolder string, switchTitle *db.SwitchTitle, options settings.OrganizeOptions, templateData map[string]string, titleName string, transaction *Transaction) error {
	for dlcId, dlcFile := range gameFiles.Dlc {
		dlcTemplateData := make(map[string]string)
		for k, v := range templateData {
			dlcTemplateData[k] = v
		}

		if dlcFile.Metadata != nil {
			dlcTemplateData[settings.TEMPLATE_VERSION] = strconv.Itoa(dlcFile.Metadata.Version)
		}
		dlcTemplateData[settings.TEMPLATE_TYPE] = "DLC"
		dlcTemplateData[settings.TEMPLATE_TITLE_ID] = dlcId
		dlcTemplateData[settings.TEMPLATE_DLC_NAME] = getDlcName(switchTitle, dlcFile)

		from := filepath.Join(dlcFile.ExtendedInfo.BaseFolder, dlcFile.ExtendedInfo.FileName)
		var to string

		if options.CreateFolderPerGame {
			folderToCreate := getFolderName(options, dlcTemplateData)
			destinationPath := filepath.Join(baseFolder, folderToCreate)
			transaction.AddCreateOperation(destinationPath, titleName)
			to = filepath.Join(destinationPath, getFileName(options, dlcFile.ExtendedInfo.FileName, dlcTemplateData))
		} else {
			to = filepath.Join(dlcFile.ExtendedInfo.BaseFolder, getFileName(options, dlcFile.ExtendedInfo.FileName, dlcTemplateData))
		}

		transaction.AddMoveOperation(from, to, titleName)
	}

	return nil
}