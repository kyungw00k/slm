package db

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type TitleAttributes struct {
	Id          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Version     json.Number `json:"version,omitempty"`
	Region      string      `json:"region,omitempty"`
	ReleaseDate int         `json:"releaseDate,omitempty"`
	Publisher   string      `json:"publisher,omitempty"`
	IconUrl     string      `json:"iconUrl,omitempty"`
	Screenshots []string    `json:"screenshots,omitempty"`
	BannerUrl   string      `json:"bannerUrl,omitempty"`
	Description string      `json:"description,omitempty"`
	Size        int         `json:"size,omitempty"`
}

type SwitchTitle struct {
	Attributes TitleAttributes
	Updates    map[int]string
	Dlc        map[string]TitleAttributes
}

type SwitchTitlesDB struct {
	TitlesMap map[string]*SwitchTitle
}

func CreateSwitchTitleDB(titlesFile, versionsFile io.Reader) (*SwitchTitlesDB, error) {
	//parse the titles objects
	var titles = map[string]TitleAttributes{}
	err := decodeToJsonObject(titlesFile, &titles)
	if err != nil {
		return nil, err
	}

	//parse the titles objects
	//titleID -> versionId-> release date
	var versions = map[string]map[int]string{}
	err = decodeToJsonObject(versionsFile, &versions)
	if err != nil {
		return nil, err
	}

	result := SwitchTitlesDB{TitlesMap: map[string]*SwitchTitle{}}
	for _, attr := range titles {
		// Use the actual Nintendo title ID from the 'id' field, not the map key
		// Skip entries without a valid Nintendo title ID
		if attr.Id == "" {
			continue
		}
		id := strings.ToLower(attr.Id)

		//TitleAttributes id rules:
		//main TitleAttributes ends with 000
		//Updates ends with 800
		//Dlc have a running counter (starting with 001) in the 4 last chars
		idPrefix := id[0 : len(id)-4]
		switchTitle := &SwitchTitle{Dlc: map[string]TitleAttributes{}}
		if t, ok := result.TitlesMap[idPrefix]; ok {
			switchTitle = t
		}
		result.TitlesMap[idPrefix] = switchTitle

		//process Updates
		if strings.HasSuffix(id, "800") {
			updates := versions[id[0:len(id)-3]+"000"]
			switchTitle.Updates = updates
			continue
		}

		//process main TitleAttributes
		if strings.HasSuffix(id, "000") {
			switchTitle.Attributes = attr
			continue
		}

		//not an update, and not main TitleAttributes, so treat it as a DLC
		switchTitle.Dlc[id] = attr

	}

	return &result, nil
}

// CreateSwitchTitleDBMultiLang creates a SwitchTitlesDB by merging multiple title files in priority order.
// The first file in titleFiles has the highest priority (e.g., KR.ko), followed by subsequent files (JP.ja, US.en).
// Titles from higher priority files will override those from lower priority files.
func CreateSwitchTitleDBMultiLang(titleFiles []io.Reader, versionsFile io.Reader) (*SwitchTitlesDB, error) {
	if len(titleFiles) == 0 {
		return nil, fmt.Errorf("at least one title file is required")
	}

	// Parse versions file first
	var versions = map[string]map[int]string{}
	err := decodeToJsonObject(versionsFile, &versions)
	if err != nil {
		return nil, err
	}

	// Merge title files in reverse order (lowest priority first)
	// This allows higher priority files to overwrite lower priority ones
	mergedTitles := map[string]TitleAttributes{}

	for i := len(titleFiles) - 1; i >= 0; i-- {
		var titles = map[string]TitleAttributes{}
		err := decodeToJsonObject(titleFiles[i], &titles)
		if err != nil {
			return nil, fmt.Errorf("failed to parse title file %d: %v", i, err)
		}

		// Merge titles - higher priority (lower index) overwrites lower priority
		for id, attr := range titles {
			mergedTitles[id] = attr
		}
	}

	// Build result using merged titles
	result := SwitchTitlesDB{TitlesMap: map[string]*SwitchTitle{}}
	for _, attr := range mergedTitles {
		// Use the actual Nintendo title ID from the 'id' field, not the map key
		// Skip entries without a valid Nintendo title ID
		if attr.Id == "" {
			continue
		}
		id := strings.ToLower(attr.Id)

		//TitleAttributes id rules:
		//main TitleAttributes ends with 000
		//Updates ends with 800
		//Dlc have a running counter (starting with 001) in the 4 last chars
		idPrefix := id[0 : len(id)-4]
		switchTitle := &SwitchTitle{Dlc: map[string]TitleAttributes{}}
		if t, ok := result.TitlesMap[idPrefix]; ok {
			switchTitle = t
		}
		result.TitlesMap[idPrefix] = switchTitle

		//process Updates
		if strings.HasSuffix(id, "800") {
			updates := versions[id[0:len(id)-3]+"000"]
			switchTitle.Updates = updates
			continue
		}

		//process main TitleAttributes
		if strings.HasSuffix(id, "000") {
			switchTitle.Attributes = attr
			continue
		}

		//not an update, and not main TitleAttributes, so treat it as a DLC
		switchTitle.Dlc[id] = attr
	}

	return &result, nil
}
