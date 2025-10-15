# Switch Library Manager (SLM)

A powerful CLI tool for managing your Nintendo Switch game library backups. Scan, organize, and check for missing updates and DLC with advanced performance optimizations and multi-language support.

## ✨ Features

### Core Functionality
- **Pure CLI Interface** - Fast, lightweight, and server-friendly
- **Multi-format Support** - NSP, NSZ, XCI files with proper decryption
- **Advanced Scanning** - Concurrent file processing with adaptive performance
- **Smart Organization** - Template-based file/folder naming with dry-run support
- **Missing Content Detection** - Find missing updates and DLC
- **Cross-platform** - Windows, macOS, Linux support

### Advanced Features
- **Multi-language Titledb Support** with locale priority (Korean → Japanese → English)
- **Korean Filename Support** with proper Hangul character preservation
- **Performance Optimization** - Environment-aware worker pools (2-5x faster)
- **Progress Feedback** - Real-time TUI with file discovery and processing stats
- **Template System** - Predefined and custom naming templates
- **Transaction Safety** - Rollback capability for organize operations
- **Structured Output** - Table, JSON, CSV formats for automation

## 🚀 Quick Start

### Installation

#### From Release (Recommended)
1. Download the latest binary from [Releases](https://github.com/giwty/switch-library-manager/releases)
2. Extract and place in your PATH
3. Run `slm --help` to verify installation

#### From Source
```bash
git clone https://github.com/giwty/switch-library-manager.git
cd switch-library-manager
go build -o slm .
```

### Basic Usage

```bash
# Scan your game library
slm scan -f /path/to/games

# Scan and organize files
slm scan -f /path/to/games --rename --create-folders

# Scan multiple folders with JSON output
slm scan -F "/games1,/games2" --format json

# Preview changes before applying
slm scan -f /path/to/games --rename --create-folders --dry-run
```

## 📖 Command Reference

### Main Commands

#### `slm scan` - Scan and Process Library
The primary command that combines scanning, checking, and organizing in one operation.

```bash
# Basic scanning
slm scan -f /games                         # Scan single folder
slm scan -F "/games1,/games2"              # Scan multiple folders
slm scan -f /games --no-recursive          # Non-recursive scan
slm scan -f /games --locale KR.ko          # Use specific locale

# Output formats
slm scan -f /games --format table          # Human-readable table (default)
slm scan -f /games --format json           # Machine-readable JSON
slm scan -f /games --format csv            # Spreadsheet-compatible CSV

# Missing content checks
slm scan -f /games --check-all             # Check for all missing content (default)
slm scan -f /games --check-updates         # Check updates only
slm scan -f /games --check-dlc             # Check DLC only
slm scan -f /games --no-check              # Skip missing content checks
slm scan -f /games --ignore-dlc "id1,id2"  # Ignore specific DLC IDs

# Organization options
slm scan -f /games --rename                # Rename files based on metadata
slm scan -f /games --create-folders        # Create folders per game
slm scan -f /games --delete-old-updates    # Remove old update files
slm scan -f /games --template detailed     # Use predefined template
slm scan -f /games --dry-run               # Preview changes only

# Performance tuning
slm scan -f /games --max-workers 16        # Set worker count manually
slm scan -f /games --memory-limit 1024     # Set memory limit (MB)
slm scan -f /games --profile               # Enable performance profiling

# Output control
slm scan -f /games --output-mode auto      # Auto-detect best output
slm scan -f /games --output-mode simple    # Simple progress output
slm scan -f /games --output-mode rich      # Rich TUI progress
slm scan -f /games --output-mode json      # JSON-only output
```

#### `slm config` - Configuration Management

```bash
slm config                                 # Show all settings
slm config debug                           # Get specific value
slm config debug true                      # Set value
slm config --reset                         # Reset to defaults
slm config --migrate /old/settings.json    # Migrate from old format
```

#### `slm cache` - Cache Management

```bash
slm cache                                  # Show cache status
slm cache --clean                          # Clear all cache
slm cache --update                         # Force update title databases
slm cache --path                           # Show cache directory
```

### Global Flags

```bash
--config-dir <path>         # Custom config directory
--output-mode <mode>        # Output mode: auto, simple, rich, json, csv
--help, -h                  # Show help
--version                   # Show version
```

## 🎯 Templates

### Predefined Templates

Use `slm scan --list-templates` to see all available templates:

- **simple**: `{TITLE_NAME}` - Just the game name
- **detailed**: `{TITLE_NAME} [{TITLE_ID}][v{VERSION}]` - Name with ID and version
- **organized**: `{TITLE_NAME}/{TITLE_NAME} [{TITLE_ID}][v{VERSION}]` - Folder per game
- **korean**: Optimized for Korean titles with fallback
- **minimal**: `{TITLE_ID}` - Title ID only
- **version-focused**: `{TITLE_NAME} v{VERSION_TXT}` - Emphasizes version

### Custom Templates

Create your own template files or use inline templates:

```bash
# Inline template
slm scan -f /games --template "folder:{TITLE_NAME};file:{TITLE_NAME} [{TITLE_ID}]"

# Template file
slm scan -f /games --template /path/to/custom-template.json
```

### Template Variables

- `{TITLE_NAME}` - Game name from titledb
- `{TITLE_ID}` - 16-character title ID
- `{VERSION}` - Version number (hex)
- `{VERSION_TXT}` - Version text (like 1.0.0)
- `{REGION}` - Region code
- `{TYPE}` - Content type [UPD, DLC]
- `{DLC_NAME}` - DLC name (for DLC only)

## ⚙️ Configuration

SLM uses a configuration file located at `$HOME/.config/slm/config.json` (or `$HOME/.slm/config.json`).

### Default Configuration

```json
{
  "prod_keys": "",
  "gui": false,
  "debug": false,
  "check_for_missing_updates": true,
  "check_for_missing_dlc": true,
  "scan_recursively": true,
  "locale_priority": ["KR.ko", "JP.ja", "US.en"],
  "organize_options": {
    "create_folder_per_game": false,
    "rename_files": false,
    "delete_empty_folders": false,
    "delete_old_update_files": false,
    "folder_name_template": "{TITLE_NAME}",
    "file_name_template": "{TITLE_NAME} [{TITLE_ID}][v{VERSION}]",
    "switch_safe_file_names": true,
    "dry_run": false
  },
  "ignore_dlc_title_ids": []
}
```

### Key Configuration Options

- **locale_priority**: Language preference order for title names
- **prod_keys**: Path to Nintendo Switch keys file (optional but recommended)
- **organize_options**: Default organization behavior
- **ignore_dlc_title_ids**: DLC titles to skip during checks

## 🔧 Performance & Environment

### Automatic Optimization

SLM automatically detects your environment and optimizes performance:

- **Local Storage**: Uses CPU core count for workers
- **Remote Mounts (NFS/SMB)**: Uses 2-3x CPU cores to utilize I/O wait time
- **Memory Pressure**: Reduces workers when memory is constrained
- **File System Type**: Adapts to different storage characteristics

### Manual Tuning

```bash
# Override automatic worker detection
slm scan -f /games --max-workers 32

# Set memory limit
slm scan -f /games --memory-limit 2048

# Enable performance profiling
slm scan -f /games --profile
```

## 🔐 Nintendo Switch Keys (Optional)

For accurate file classification, provide a `prod.keys` file:

1. Place `prod.keys` in the app directory, or
2. Place in `$HOME/.switch/prod.keys`, or
3. Set custom path: `slm config prod_keys /path/to/prod.keys`

**Note**: Only `header_key` and `key_area_key_application_XX` keys are required.

## 📁 Directory Structure

```
$HOME/.config/slm/
├── config.json              # Main configuration
├── cache/
│   ├── versions.json        # Version cache
│   ├── titledb/            # Language-specific title databases
│   │   ├── KR.ko.json
│   │   ├── US.en.json
│   │   └── JP.ja.json
│   └── db/                 # Local game databases
├── logs/
│   └── slm.log             # Application logs
└── keys/
    └── prod.keys           # Nintendo Switch keys (optional)
```

## 🤖 Automation & Scripting

### JSON Output for Scripts

```bash
# Get library status as JSON
slm scan -f /games --format json --no-check > library.json

# Check for missing updates only
slm scan -f /games --check-updates --format json | jq '.missing_updates[]'

# Automated organization
slm scan -f /games --rename --create-folders --output-mode json
```

### CSV Export for Analysis

```bash
# Export library to CSV
slm scan -f /games --format csv > library.csv

# Import into spreadsheet applications
```

## 🐛 Troubleshooting

### Enable Debug Logging

```bash
slm config debug true
slm scan -f /games --verbose
```

Logs are saved to `$HOME/.config/slm/logs/slm.log`

### Common Issues

1. **Slow scanning**: Check if using remote mount, SLM will auto-optimize
2. **Memory issues**: Reduce workers with `--max-workers` or set `--memory-limit`
3. **Missing titles**: Update title database with `slm cache --update`
4. **Permission errors**: Ensure read/write access to scan folders

### Performance Analysis

```bash
# Enable profiling for performance analysis
slm scan -f /games --profile
```

## 🔄 Migration from GUI Version

If upgrading from the old GUI version:

```bash
# Automatically migrate settings
slm config --migrate /path/to/old/settings.json

# Or manually copy configuration
cp /old/app/folder/settings.json $HOME/.config/slm/config.json
```

## 🏗️ Building from Source

### Prerequisites
- Go 1.21 or later
- Git

### Build Commands

```bash
# Clone repository
git clone https://github.com/giwty/switch-library-manager.git
cd switch-library-manager

# Build for current platform
go build -o slm .

# Cross-platform builds
GOOS=windows GOARCH=amd64 go build -o slm.exe .
GOOS=darwin GOARCH=amd64 go build -o slm-darwin .
GOOS=linux GOARCH=amd64 go build -o slm-linux .

# Build with optimizations
go build -ldflags="-s -w" -o slm .
```

### Development

```bash
# Run tests
go test ./...

# Run with race detection
go run -race . scan -f /test/folder

# Integration tests
go test -v -run "TestIntegration" .
```

## 📋 Examples

### Daily Workflow

```bash
# Check for new content in your library
slm scan -f /games --format table

# Organize new downloads
slm scan -f /downloads --rename --create-folders --template organized

# Preview organization before applying
slm scan -f /downloads --rename --create-folders --dry-run

# Generate library report for spreadsheet
slm scan -F "/games,/dlc" --format csv > library-report.csv
```

### Advanced Usage

```bash
# Scan with custom performance settings
slm scan -f /network/games --max-workers 24 --memory-limit 4096

# Multi-language setup with Korean priority
slm config locale_priority '["KR.ko", "JP.ja", "US.en"]'
slm scan -f /games --locale KR.ko

# Automated missing content checking
slm scan -f /games --check-all --format json | jq '.missing_content | length'
```

## 🙏 Acknowledgments

- [blawar's titledb](https://github.com/blawar/titledb) for title metadata
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI framework
- [Cobra](https://github.com/spf13/cobra) for CLI framework

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

For bug reports and feature requests, please use the GitHub Issues page.