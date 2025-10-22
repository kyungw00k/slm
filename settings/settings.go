package settings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mcuadros/go-version"
	"go.uber.org/zap"
)

var (
	settingsInstance *AppSettings
)

const (
	SETTINGS_FILENAME      = "settings.json"
	TITLE_JSON_FILENAME    = "titles.json"
	VERSIONS_JSON_FILENAME = "versions.json"
	SLM_VERSION            = "1.4.0"
	TITLES_JSON_URL        = "https://raw.githubusercontent.com/blawar/titledb/master/KR.ko.json"
	//TITLES_JSON_URL    = "https://raw.githubusercontent.com/blawar/titledb/master/titles.US.en.json"
	VERSIONS_JSON_URL = "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
	//VERSIONS_JSON_URL = "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
	SLM_VERSION_URL = "https://raw.githubusercontent.com/giwty/switch-library-manager/master/slm.json"
)

const (
	TEMPLATE_TITLE_ID    = "TITLE_ID"
	TEMPLATE_TITLE_NAME  = "TITLE_NAME"
	TEMPLATE_DLC_NAME    = "DLC_NAME"
	TEMPLATE_VERSION     = "VERSION"
	TEMPLATE_REGION      = "REGION"
	TEMPLATE_VERSION_TXT = "VERSION_TXT"
	TEMPLATE_TYPE        = "TYPE"
)

type OrganizeOptions struct {
	CreateFolderPerGame  bool   `json:"create_folder_per_game"`
	RenameFiles          bool   `json:"rename_files"`
	DeleteEmptyFolders   bool   `json:"delete_empty_folders"`
	DeleteOldUpdateFiles bool   `json:"delete_old_update_files"`
	FolderNameTemplate   string `json:"folder_name_template"`
	SwitchSafeFileNames  bool   `json:"switch_safe_file_names"`
	FileNameTemplate     string `json:"file_name_template"`
	DryRun               bool   `json:"dry_run"`
}

type AppSettings struct {
	VersionsEtag           string            `json:"versions_etag"`
	Prodkeys               string            `json:"prod_keys"`
	Folder                 string            `json:"folder"`
	ScanFolders            []string          `json:"scan_folders"`
	GUI                    bool              `json:"gui"`
	Debug                  bool              `json:"debug"`
	CheckForMissingUpdates bool              `json:"check_for_missing_updates"`
	CheckForMissingDLC     bool              `json:"check_for_missing_dlc"`
	OrganizeOptions        OrganizeOptions   `json:"organize_options"`
	ScanRecursively        bool              `json:"scan_recursively"`
	GuiPagingSize          int               `json:"gui_page_size"`
	IgnoreDLCTitleIds      []string          `json:"ignore_dlc_title_ids"`
	LocalePriority         []string          `json:"locale_priority"`
	TitleDBUrls            map[string]string `json:"titledb_urls"`
	TitlesETags            map[string]string `json:"titles_etags"`
}

func ReadSettingsAsJSON(baseFolder string) string {
	if _, err := os.Stat(filepath.Join(baseFolder, SETTINGS_FILENAME)); err != nil {
		saveDefaultSettings(baseFolder)
	}
	file, _ := os.Open(filepath.Join(baseFolder, SETTINGS_FILENAME))
	bytes, _ := ioutil.ReadAll(file)
	return string(bytes)
}

func ReadSettings(baseFolder string) *AppSettings {
	if settingsInstance != nil {
		return settingsInstance
	}
	settingsInstance = &AppSettings{Debug: false, GuiPagingSize: 100, ScanFolders: []string{},
		OrganizeOptions: OrganizeOptions{SwitchSafeFileNames: true}, Prodkeys: "", IgnoreDLCTitleIds: []string{"01007F600B135007"},
		LocalePriority: []string{"KR.ko", "US.en", "JP.ja"},
		TitleDBUrls:    map[string]string{},
		TitlesETags: map[string]string{
			"KR.ko": "",
			"US.en": "",
			"JP.ja": "",
		}}
	if _, err := os.Stat(filepath.Join(baseFolder, SETTINGS_FILENAME)); err == nil {
		file, err := os.Open(filepath.Join(baseFolder, SETTINGS_FILENAME))
		if err != nil {
			zap.S().Warnf("Missing or corrupted config file, creating a new one")
			return saveDefaultSettings(baseFolder)
		} else {
			_ = json.NewDecoder(file).Decode(&settingsInstance)
			return settingsInstance
		}
	} else {
		return saveDefaultSettings(baseFolder)
	}
}

func saveDefaultSettings(baseFolder string) *AppSettings {
	settingsInstance = &AppSettings{
		VersionsEtag:           "W/\"2ef50d1cb6bd61:0\"",
		Folder:                 "",
		ScanFolders:            []string{},
		IgnoreDLCTitleIds:      []string{},
		GUI:                    true,
		GuiPagingSize:          100,
		CheckForMissingUpdates: true,
		CheckForMissingDLC:     true,
		ScanRecursively:        true,
		Debug:          false,
		LocalePriority: []string{"KR.ko", "US.en", "JP.ja"},
		TitleDBUrls:    map[string]string{},
		TitlesETags: map[string]string{
			"KR.ko": "",
			"US.en": "",
			"JP.ja": "",
		},
		OrganizeOptions: OrganizeOptions{
			RenameFiles:         false,
			CreateFolderPerGame: false,
			FolderNameTemplate:  fmt.Sprintf("{%v}", TEMPLATE_TITLE_NAME),
			FileNameTemplate: fmt.Sprintf("{%v} ({%v})[{%v}][v{%v}]", TEMPLATE_TITLE_NAME, TEMPLATE_DLC_NAME,
				TEMPLATE_TITLE_ID, TEMPLATE_VERSION),
			DeleteEmptyFolders:   false,
			SwitchSafeFileNames:  true,
			DeleteOldUpdateFiles: false,
			DryRun:               false,
		},
	}
	return SaveSettings(settingsInstance, baseFolder)
}

func SaveSettings(settings *AppSettings, baseFolder string) *AppSettings {
	file, _ := json.MarshalIndent(settings, "", " ")
	_ = ioutil.WriteFile(filepath.Join(baseFolder, SETTINGS_FILENAME), file, 0644)
	settingsInstance = settings
	return settings
}

// GetTitleDBURL returns the URL for a given locale, using default pattern if not specified in settings
func (s *AppSettings) GetTitleDBURL(locale string) string {
	if s.TitleDBUrls != nil {
		if url, exists := s.TitleDBUrls[locale]; exists {
			return url
		}
	}
	// Default pattern: https://raw.githubusercontent.com/blawar/titledb/master/<locale>.json
	return fmt.Sprintf("https://raw.githubusercontent.com/blawar/titledb/master/%s.json", locale)
}

// GetLanguageMapping maps locale codes to NACP language names
func GetLanguageMapping() map[string]string {
	return map[string]string{
		"KR.ko": "Korean",
		"US.en": "AmericanEnglish",
		"GB.en": "BritishEnglish",
		"JP.ja": "Japanese",
		"FR.fr": "French",
		"DE.de": "German",
		"ES.es": "Spanish",
		"IT.it": "Italian",
		"NL.nl": "Dutch",
		"CA.fr": "CanadianFrench",
		"PT.pt": "Portuguese",
		"RU.ru": "Russian",
		"TW.zh": "Taiwanese",
		"CN.zh": "Chinese",
	}
}

func CheckForUpdates() (bool, error) {

	localVer := SLM_VERSION

	res, err := http.Get(SLM_VERSION_URL)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	remoteValues := map[string]string{}
	err = json.Unmarshal(body, &remoteValues)
	if err != nil {
		return false, err
	}

	remoteVer := remoteValues["version"]

	if version.CompareSimple(remoteVer, localVer) > 0 {
		return true, nil
	}

	return false, nil
}
