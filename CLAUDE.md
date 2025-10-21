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
- **Scan Command**: `cmd/scan.go` - Primary functionality combining scanning, checking, and organizing
- **Config Command**: `cmd/config.go` - Configuration management
- **Cache Command**: `cmd/cache.go` - Cache and database management

### Core Packages

**pkg/scanner**: Main orchestration logic
- `scanner.go`: Multi-stage scan process with TUI progress tracking
- `output.go`: Multi-format output (table/JSON/CSV) with structured data models

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
- `localSwitchFilesDB.go`: Local game library database
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

## Legacy Code Notes

- **GUI Components**: Old Astilectron GUI code remains in codebase but is unused
- **Console Mode**: Legacy console.go is preserved in `old/` directory for reference
- **Settings Migration**: System automatically migrates old `settings.json` to new config format

The project prioritizes data safety, user feedback, and maintainable architecture while preserving all existing functionality in a more robust CLI-first design.