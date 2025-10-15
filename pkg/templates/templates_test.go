package templates

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	slmErrors "github.com/giwty/switch-library-manager/pkg/errors"
	"github.com/giwty/switch-library-manager/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredefinedTemplates(t *testing.T) {
	templates := PredefinedTemplates()

	// Verify we have expected templates
	assert.Len(t, templates, 6)

	// Check for specific templates
	templateNames := make(map[string]bool)
	for _, tmpl := range templates {
		templateNames[tmpl.Name] = true

		// Each template should have required fields
		assert.NotEmpty(t, tmpl.Name)
		assert.NotEmpty(t, tmpl.Description)
		assert.NotEmpty(t, tmpl.File)
		assert.NotEmpty(t, tmpl.Example)
	}

	// Verify specific templates exist
	assert.True(t, templateNames["simple"])
	assert.True(t, templateNames["detailed"])
	assert.True(t, templateNames["organized"])
	assert.True(t, templateNames["korean"])
	assert.True(t, templateNames["minimal"])
	assert.True(t, templateNames["version-focused"])
}

func TestLoadTemplateFromFile_PredefinedTemplate(t *testing.T) {
	// Test loading predefined template
	template, err := LoadTemplateFromFile("simple")
	require.NoError(t, err)
	require.NotNil(t, template)

	assert.Equal(t, "simple", template.Name)
	assert.Equal(t, "Simple naming with title name only", template.Description)
	assert.Contains(t, template.File, settings.TEMPLATE_TITLE_NAME)
}

func TestLoadTemplateFromFile_FromFile(t *testing.T) {
	// Create temporary template file
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "test_template.json")

	testTemplate := Template{
		Name:        "test-template",
		Description: "Test template",
		Folder:      "{TITLE_NAME}",
		File:        "{TITLE_NAME} [{TITLE_ID}]",
		Example:     "Game Name/Game Name [0100000000000000]",
	}

	data, err := json.MarshalIndent(testTemplate, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(templatePath, data, 0644)
	require.NoError(t, err)

	// Load template from file
	template, err := LoadTemplateFromFile(templatePath)
	require.NoError(t, err)
	require.NotNil(t, template)

	assert.Equal(t, "test-template", template.Name)
	assert.Equal(t, "Test template", template.Description)
	assert.Equal(t, "{TITLE_NAME}", template.Folder)
	assert.Equal(t, "{TITLE_NAME} [{TITLE_ID}]", template.File)
}

func TestLoadTemplateFromFile_FileNotFound(t *testing.T) {
	// Test with non-existent file
	_, err := LoadTemplateFromFile("/nonexistent/template.json")
	require.Error(t, err)

	// Should be a file not found error
	assert.True(t, slmErrors.IsType(err, slmErrors.ErrorTypeFileNotFound))
}

func TestLoadTemplateFromFile_RelativePath(t *testing.T) {
	// Test with relative path (should fail)
	_, err := LoadTemplateFromFile("relative/path.json")
	require.Error(t, err)

	// Should be an invalid input error
	assert.True(t, slmErrors.IsType(err, slmErrors.ErrorTypeInvalidInput))
}

func TestLoadTemplateFromFile_InvalidJSON(t *testing.T) {
	// Create temporary file with invalid JSON
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "invalid.json")

	err := os.WriteFile(templatePath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Try to load invalid template
	_, err = LoadTemplateFromFile(templatePath)
	require.Error(t, err)

	// Should be an invalid template error
	assert.True(t, slmErrors.IsType(err, slmErrors.ErrorTypeInvalidTemplate))
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name        string
		folder      string
		file        string
		expectError bool
	}{
		{
			name:        "valid template with folder",
			folder:      "{TITLE_NAME}",
			file:        "{TITLE_NAME} [{TITLE_ID}]",
			expectError: false,
		},
		{
			name:        "valid template without folder",
			folder:      "",
			file:        "{TITLE_NAME}",
			expectError: false,
		},
		{
			name:        "valid template with title ID",
			folder:      "{TITLE_ID}",
			file:        "{TITLE_ID}",
			expectError: false,
		},
		{
			name:        "invalid folder template",
			folder:      "no variables here",
			file:        "{TITLE_NAME}",
			expectError: true,
		},
		{
			name:        "invalid file template",
			folder:      "{TITLE_NAME}",
			file:        "no variables here",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplate(tt.folder, tt.file)
			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, slmErrors.IsType(err, slmErrors.ErrorTypeInvalidTemplate))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseInlineTemplate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedFile string
		expectedFolder string
	}{
		{
			name:           "valid inline template with folder",
			input:          "folder:{TITLE_NAME};file:{TITLE_NAME} [{TITLE_ID}]",
			expectError:    false,
			expectedFile:   "{TITLE_NAME} [{TITLE_ID}]",
			expectedFolder: "{TITLE_NAME}",
		},
		{
			name:           "valid inline template without folder",
			input:          "file:{TITLE_NAME}",
			expectError:    false,
			expectedFile:   "{TITLE_NAME}",
			expectedFolder: "",
		},
		{
			name:        "invalid inline template without file",
			input:       "folder:{TITLE_NAME}",
			expectError: true,
		},
		{
			name:           "predefined template name",
			input:          "simple",
			expectError:    false,
			expectedFile:   "{TITLE_NAME}",
			expectedFolder: "{TITLE_NAME}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template, err := ParseInlineTemplate(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, template)
				assert.Equal(t, tt.expectedFile, template.File)
				assert.Equal(t, tt.expectedFolder, template.Folder)
			}
		})
	}
}

func TestApplyTemplateToOptions(t *testing.T) {
	options := &settings.OrganizeOptions{
		RenameFiles:         false,
		CreateFolderPerGame: false,
		FileNameTemplate:    "original",
		FolderNameTemplate:  "original",
	}

	template := &Template{
		Folder: "{TITLE_NAME}",
		File:   "{TITLE_NAME} [{TITLE_ID}]",
	}

	ApplyTemplateToOptions(template, options)

	assert.True(t, options.RenameFiles)
	assert.True(t, options.CreateFolderPerGame)
	assert.Equal(t, "{TITLE_NAME} [{TITLE_ID}]", options.FileNameTemplate)
	assert.Equal(t, "{TITLE_NAME}", options.FolderNameTemplate)
}

func TestApplyTemplateToOptions_NoFolder(t *testing.T) {
	options := &settings.OrganizeOptions{
		RenameFiles:         false,
		CreateFolderPerGame: true,
		FileNameTemplate:    "original",
		FolderNameTemplate:  "original",
	}

	template := &Template{
		Folder: "", // No folder
		File:   "{TITLE_NAME}",
	}

	ApplyTemplateToOptions(template, options)

	assert.True(t, options.RenameFiles)
	assert.False(t, options.CreateFolderPerGame) // Should be set to false
	assert.Equal(t, "{TITLE_NAME}", options.FileNameTemplate)
}

func TestSaveTemplateToFile(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "save_test.json")

	template := &Template{
		Name:        "save-test",
		Description: "Test saving template",
		Folder:      "{TITLE_NAME}",
		File:        "{TITLE_NAME} [{TITLE_ID}]",
		Example:     "Game/Game [ID]",
	}

	err := SaveTemplateToFile(template, templatePath)
	require.NoError(t, err)

	// Verify file exists and contains correct data
	assert.FileExists(t, templatePath)

	// Load and verify content
	loadedTemplate, err := LoadTemplateFromFile(templatePath)
	require.NoError(t, err)

	assert.Equal(t, template.Name, loadedTemplate.Name)
	assert.Equal(t, template.Description, loadedTemplate.Description)
	assert.Equal(t, template.Folder, loadedTemplate.Folder)
	assert.Equal(t, template.File, loadedTemplate.File)
	assert.Equal(t, template.Example, loadedTemplate.Example)
}

func TestCreateTemplateExample(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "example.json")

	err := CreateTemplateExample(templatePath)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, templatePath)

	// Load and verify it's a valid template
	template, err := LoadTemplateFromFile(templatePath)
	require.NoError(t, err)

	assert.Equal(t, "custom-example", template.Name)
	assert.NotEmpty(t, template.Description)
	assert.NotEmpty(t, template.Folder)
	assert.NotEmpty(t, template.File)
	assert.NotEmpty(t, template.Example)
}

func TestListTemplates(t *testing.T) {
	output := ListTemplates()

	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Available templates:")
	assert.Contains(t, output, "simple")
	assert.Contains(t, output, "detailed")
	assert.Contains(t, output, "Custom templates:")
}