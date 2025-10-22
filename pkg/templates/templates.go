package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	slmErrors "github.com/giwty/switch-library-manager/pkg/errors"
	"github.com/giwty/switch-library-manager/settings"
)

// Template represents a naming template
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Folder      string `json:"folder"`
	File        string `json:"file"`
	Example     string `json:"example"`
}

// TemplateSet contains predefined templates
type TemplateSet struct {
	Templates []Template `json:"templates"`
}

// PredefinedTemplates returns built-in templates
func PredefinedTemplates() []Template {
	return []Template{
		{
			Name:        "simple",
			Description: "Simple naming with title name only",
			Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			File:        fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			Example:     "Super Mario Odyssey/Super Mario Odyssey",
		},
		{
			Name:        "detailed",
			Description: "Detailed naming with title, ID, and version",
			Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			File:        fmt.Sprintf("{%s} [{%s}][v{%s}]", settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_TITLE_ID, settings.TEMPLATE_VERSION),
			Example:     "Super Mario Odyssey/Super Mario Odyssey [0100000000010000][v0]",
		},
		{
			Name:        "organized",
			Description: "Organized by type with clear naming",
			Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			File:        fmt.Sprintf("{%s} ({%s}) [{%s}][v{%s}]", settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_TYPE, settings.TEMPLATE_TITLE_ID, settings.TEMPLATE_VERSION),
			Example:     "Super Mario Odyssey/Super Mario Odyssey (BASE) [0100000000010000][v0]",
		},
		{
			Name:        "korean",
			Description: "Korean-friendly naming with region info",
			Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			File:        fmt.Sprintf("{%s} ({%s}) [{%s}]", settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_REGION, settings.TEMPLATE_TITLE_ID),
			Example:     "슈퍼 마리오 오디세이/슈퍼 마리오 오디세이 (KR) [0100000000010000]",
		},
		{
			Name:        "minimal",
			Description: "Minimal naming with no folders",
			Folder:      "",
			File:        fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			Example:     "Super Mario Odyssey",
		},
		{
			Name:        "version-focused",
			Description: "Version-focused naming for update tracking",
			Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
			File:        fmt.Sprintf("{%s} v{%s} ({%s})", settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_VERSION_TXT, settings.TEMPLATE_TYPE),
			Example:     "Super Mario Odyssey/Super Mario Odyssey v1.3.0 (UPD)",
		},
	}
}

// LoadTemplateFromFile loads template from a file
func LoadTemplateFromFile(templatePath string) (*Template, error) {
	// Check if it's a predefined template name first
	predefined := PredefinedTemplates()
	for _, tmpl := range predefined {
		if tmpl.Name == templatePath {
			return &tmpl, nil
		}
	}

	// Try to load from file
	if !filepath.IsAbs(templatePath) {
		return nil, slmErrors.New(slmErrors.ErrorTypeInvalidInput,
			"template file path must be absolute").WithContext("path", templatePath)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, slmErrors.Wrap(err, slmErrors.ErrorTypeFileNotFound,
				"template file not found").WithContext("path", templatePath)
		}
		return nil, slmErrors.Wrap(err, slmErrors.ErrorTypeFilePermission,
			"failed to read template file").WithContext("path", templatePath)
	}

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, slmErrors.Wrap(err, slmErrors.ErrorTypeInvalidTemplate,
			"failed to parse template file").WithContext("path", templatePath)
	}

	return &template, nil
}

// ValidateTemplate validates template syntax
func ValidateTemplate(folder, file string) error {
	// Check if folder template contains required elements (if not empty)
	if folder != "" && !containsRequiredTemplateVar(folder) {
		return slmErrors.New(slmErrors.ErrorTypeInvalidTemplate,
			fmt.Sprintf("folder template must contain either {%s} or {%s}",
				settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_TITLE_ID))
	}

	// Check if file template contains required elements
	if !containsRequiredTemplateVar(file) {
		return slmErrors.New(slmErrors.ErrorTypeInvalidTemplate,
			fmt.Sprintf("file template must contain either {%s} or {%s}",
				settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_TITLE_ID))
	}

	return nil
}

// containsRequiredTemplateVar checks if template contains required variables
func containsRequiredTemplateVar(template string) bool {
	return strings.Contains(template, fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME)) ||
		   strings.Contains(template, fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_ID))
}

// ListTemplates returns a formatted list of available templates
func ListTemplates() string {
	var result strings.Builder
	result.WriteString("Available templates:\n\n")

	for _, tmpl := range PredefinedTemplates() {
		result.WriteString(fmt.Sprintf("  %s\n", tmpl.Name))
		result.WriteString(fmt.Sprintf("   %s\n", tmpl.Description))
		result.WriteString(fmt.Sprintf("   Example: %s\n\n", tmpl.Example))
	}

	result.WriteString("Custom templates:\n")
	result.WriteString("   Use a file path to load custom templates from JSON files\n")
	result.WriteString("   Or specify inline: --template 'folder:{TITLE_NAME};file:{TITLE_NAME}[{TITLE_ID}]'\n")

	return result.String()
}

// ParseInlineTemplate parses inline template specification
func ParseInlineTemplate(template string) (*Template, error) {
	// Handle inline format: "folder:{TITLE_NAME};file:{TITLE_NAME}[{TITLE_ID}]"
	if strings.Contains(template, "folder:") || strings.Contains(template, "file:") {
		parts := strings.Split(template, ";")
		folder := ""
		file := ""

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "folder:") {
				folder = strings.TrimPrefix(part, "folder:")
			} else if strings.HasPrefix(part, "file:") {
				file = strings.TrimPrefix(part, "file:")
			}
		}

		if file == "" {
			return nil, slmErrors.New(slmErrors.ErrorTypeInvalidTemplate,
				"inline template must specify file template")
		}

		return &Template{
			Name:   "inline",
			Folder: folder,
			File:   file,
		}, nil
	}

	// Try as predefined template name or file path
	return LoadTemplateFromFile(template)
}

// ApplyTemplateToOptions applies template to organize options
func ApplyTemplateToOptions(template *Template, options *settings.OrganizeOptions) {
	if template.Folder != "" {
		options.FolderNameTemplate = template.Folder
		options.CreateFolderPerGame = true
	} else {
		options.CreateFolderPerGame = false
	}

	options.FileNameTemplate = template.File
	options.RenameFiles = true
}

// SaveTemplateToFile saves a template to a JSON file
func SaveTemplateToFile(template *Template, filePath string) error {
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// CreateTemplateExample creates an example template file
func CreateTemplateExample(filePath string) error {
	example := Template{
		Name:        "custom-example",
		Description: "Custom template example",
		Folder:      fmt.Sprintf("{%s}", settings.TEMPLATE_TITLE_NAME),
		File:        fmt.Sprintf("{%s} [{%s}][v{%s}]", settings.TEMPLATE_TITLE_NAME, settings.TEMPLATE_TITLE_ID, settings.TEMPLATE_VERSION),
		Example:     "Game Name/Game Name [0100000000000000][v0]",
	}

	return SaveTemplateToFile(&example, filePath)
}