# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Switch Library Manager (SLM) is a CLI tool for managing Nintendo Switch game libraries. The project has been restructured from a GUI/Console hybrid to a pure CLI application using Go and Cobra framework.

**Core functionality:**
- Scan Nintendo Switch game files (NSP/NSZ/XCI)
- Organize games with automatic renaming and folder structure
- Check for missing updates and DLC
- Multi-language titledb support with Korean/Japanese/English priority
- Transaction-based file operations with rollback capability

## Build and Development Commands

```bash
# Build the main CLI application
go build -o slm-new .

# Run tests
go test ./...
go test ./process  # Test specific package

# Build for development with race detection
go build -race -o slm-new .

# Clean and rebuild
rm slm-new && go build -o slm-new .
```

## Git Commit Guidelines

**Commit Policy:**
- Commit after completing each testable feature
- Use Conventional Commits format
- Update documentation before committing tested features

**Conventional Commit Format:**
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `perf`: Performance improvement
- `fix`: Bug fix
- `docs`: Documentation only changes
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```bash
# New feature with documentation
git add .
git commit -m "feat(scanner): add parallel folder scanning with streaming pipeline

- Implement parallel directory scanning for multiple folders
- Add streaming pipeline to process files as they're discovered
- Reduce memory usage by eliminating intermediate file array
- Update Context.md and Plan.md with performance results

Performance: 85.6% faster than baseline (105.9s → 15.3s)"

# Performance optimization
git commit -m "perf(scanner): optimize network I/O with godirwalk

- Replace filepath.Walk with godirwalk for 64KB buffer
- Add early filtering to skip non-game files during scan
- Reduce memory usage by 60% (1,491 → 607 files)

Test results: 26.7s vs 105.9s baseline (85% improvement)"

# Documentation update
git commit -m "docs: update performance optimization results in Context.md"
```

**Workflow:**
1. Implement feature
2. Test feature (`go build` + manual testing)
3. Update relevant documentation (Context.md, Plan.md, etc.)
4. Commit with conventional format
5. Continue to next feature

## Project Architecture

### Command Structure (cmd/)
- **Root Command**: `cmd/root.go` - Main CLI entry point with global flags
- **Scan Command**: `cmd/scan.go` - Scan game files and build database (outputs summary by default)
- **List Command**: `cmd/list.go` - Query and filter games from database with pagination
- **Config Command**: `cmd/config.go` - Configuration management
- **Cache Command**: `cmd/cache.go` - Cache and database management

### Core Packages

**pkg/scanner**: Main orchestration logic
- `scanner.go`: Multi-stage scan process with TUI progress tracking
- `output.go`: Multi-format output (table/JSON/CSV) with summary and detailed views

**pkg/config**: Configuration management
- Handles `$HOME/.config/slm/config.json` and migration from legacy `settings.json`
- Environment variable support and directory standardization

**pkg/progress**: TUI progress system using Bubble Tea
- Multi-stage progress tracking with real-time updates
- TTY/non-TTY environment detection

**process/**: File operations and game processing
- `organizefolderStructure.go`: Legacy organize functions
- `organizeTransactional.go`: New transaction-based organize with rollback
- `transaction.go`: Transaction system for safe file operations
- `incompleteTitleProcessor.go`: Missing content detection

**db/**: Database layer (BoltDB)
- `localSwitchFilesDB.go`: Local game library database with filtering and pagination (ListGames)
- `switchTitlesDB.go`: Title metadata from remote sources
- `persistentDB.go`: Database persistence layer

**switchfs/**: Nintendo Switch file format parsing
- NSP/XCI/NSZ file handling and metadata extraction
- Crypto operations for decryption (requires prod.keys)

**settings/**: Legacy configuration system (being phased out)

### Key Architecture Patterns

**Multi-stage Pipeline**: Scan operations follow a consistent pattern:
1. Download/update title databases
2. Scan local files and build library
3. Check for missing content (optional)
4. Organize files (optional)
5. Update database paths (if organized)

**Transaction Safety**: File organization uses transaction pattern:
- Plan all operations first
- Execute atomically with rollback on failure
- Update database only after successful completion

**Configuration Priority**: Settings resolved in order:
1. CLI flags (highest priority)
2. Environment variables (`SLM_CONFIG_DIR`, etc.)
3. `$HOME/.config/slm/config.json`
4. Default values

**Output Abstraction**: Consistent output interface across formats:
- Table format for human-readable output
- JSON for machine consumption and scripting
- CSV for data analysis and spreadsheet import

**Scan/List Separation Pattern**: Clean separation of concerns for better UX:
- `scan`: Builds database and outputs concise summary (total games, missing content, database location)
- `list`: Queries existing database with filtering, sorting, and pagination
- Use `--show-table` flag on `scan` to show full table immediately after scanning
- This pattern prevents overwhelming output for large game libraries (3000+ titles)

## Important Implementation Details

**Database Path Synchronization**: After file organization, the system re-scans to update database entries with new file paths. This ensures database consistency.

**Korean Character Support**: Special handling for Korean filenames and titles using proper Unicode preservation rather than romanization.

**Concurrent Safety**: Database operations use scan-path-based separation to allow concurrent execution across different game libraries.

**Dry Run Mode**: All organize operations support dry-run mode showing planned changes without executing them.

**Progress Feedback**: TUI provides detailed progress with multiple stages, file counts, and current operation status.

## Configuration and Data Layout

```
$HOME/.config/slm/
├── config.json              # Main configuration
├── cache/
│   ├── versions.json        # Version cache
│   ├── titledb/            # Language-specific title databases
│   └── db/                 # Local game databases per scan path
├── logs/
└── keys/
    └── prod.keys           # Nintendo Switch crypto keys (optional)
```

## Testing Considerations

**Test with Empty Directories**: Core functionality should handle empty game libraries gracefully and output appropriate empty results.

**Transaction Testing**: Organize operations should be testable with dry-run mode to verify planned changes before execution.

**Output Format Testing**: All three output formats (table/JSON/CSV) should produce consistent data structures.

## Common Usage Examples

**Basic Workflow:**
```bash
# 1. Scan your game library (creates database and shows summary)
slm scan -f /path/to/games

# 2. List games with various filters
slm list                          # First 50 games
slm list --missing-updates        # Games needing updates
slm list --title "zelda"          # Search by title
slm list --page 2 --per-page 100  # Pagination
slm list --format json            # JSON output

# 3. Scan with full table output (for smaller libraries)
slm scan -f /games --show-table

# 4. Organize games after scanning
slm scan -f /games --rename --create-folders --dry-run  # Preview changes
slm scan -f /games --rename --create-folders            # Apply changes
```

**List Command Examples:**
```bash
# Filter by missing content
slm list --missing-updates           # Games with available updates
slm list --missing-dlc              # Games with available DLC

# Search and filter
slm list --title "pokemon"          # Case-insensitive title search
slm list --title-id 0100f2c         # Search by title ID prefix

# Sorting
slm list --sort title               # Sort by title (default)
slm list --sort title-id            # Sort by title ID
slm list --sort missing             # Sort by missing content count (descending)

# Pagination
slm list --page 3                   # Show page 3
slm list --per-page 25              # 25 results per page
slm list --limit 10                 # Show only first 10 results

# Output formats
slm list --format table             # Human-readable table (default)
slm list --format json              # JSON for scripting
slm list --format csv               # CSV for spreadsheets

# Combined filters
slm list --missing-updates --sort missing --limit 20  # Top 20 games needing updates
```

**Scan Command Options:**
```bash
# Basic scanning
slm scan -f /games                  # Scan single folder
slm scan -F "/games1,/games2"       # Scan multiple folders
slm scan -f /games --no-recursive   # Non-recursive scan

# Content checking
slm scan -f /games --no-check       # Skip missing content check
slm scan -f /games --check-updates  # Check updates only
slm scan -f /games --check-dlc      # Check DLC only

# File organization
slm scan -f /games --rename                           # Rename files
slm scan -f /games --create-folders                   # Create per-game folders
slm scan -f /games --rename --create-folders --dry-run # Preview changes
slm scan -f /games --delete-old-updates               # Remove old updates

# Output control
slm scan -f /games                  # Summary only (default)
slm scan -f /games --show-table     # Show full table after scan
slm scan -f /games --show-table --format json  # Full output in JSON

# Performance tuning
slm scan -f /games --max-workers 8  # Use 8 parallel workers
slm scan -f /games --profile        # Enable performance profiling
```

## Legacy Code Notes

- **GUI Components**: Old Astilectron GUI code remains in codebase but is unused
- **Console Mode**: Legacy console.go is preserved in `old/` directory for reference
- **Settings Migration**: System automatically migrates old `settings.json` to new config format

The project prioritizes data safety, user feedback, and maintainable architecture while preserving all existing functionality in a more robust CLI-first design.