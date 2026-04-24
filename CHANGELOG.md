# Changelog

All notable changes to tmeet will be documented in this file, following the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) convention.

## [v1.0.4] - 2026-04-24

### Added

- **New `tshoot log` command** (`cmd/tshoot/`): Autonomous log-based troubleshooting support
  - `tshoot log` — Query and package local log files for self-service troubleshooting
  - `--start` / `--end` — Filter logs by ISO 8601 time range
  - `--upload` — Package and upload logs to the server (login required); returns a `log_id`
  - Without `--upload`, logs are packaged into a zip file and saved to the user's home directory
  - Supports pre-filtering by date in file names + precise filtering by in-line timestamps
  - Upload flow: fetch upload token → PUT file to COS → notify server of upload completion
- **New file-based logging module** (`internal/log/`):
  - `logging.go` — Structured file logging with four levels: DEBUG / INFO / WARN / ERROR
  - Daily log rotation with 10 MB per-file size limit and auto-incrementing sequence numbers
  - Automatic cleanup of log files older than 7 days
  - Asynchronous writes via background goroutine; callers are never blocked
  - Multi-process safety: atomic writes with `O_APPEND` + cross-process file lock for rotation
  - `trace_id.go` — TraceID generator (8-char timestamp + 4-char PID + 20-char random hex), propagated via context
- **New `CalcFileInfo` utility** (`internal/utils/file.go`): Compute file size, SHA256, and MD5 in a single pass
- **New error code** `ClientUploadToCos (2007)` and `UploadToCosError` error definition (`internal/exception/`)
- Unit tests for logging module, TraceID generator, and file utility functions

### Changed

- **Refactored `log` package → `output` package**: Renamed `internal/log/log.go` to `internal/output/print.go`; package name changed from `log` to `output`. All 16 call sites across `cmd/auth/`, `cmd/meeting/`, `cmd/record/`, `cmd/report/` updated with the new import path
- **Enhanced `cmd/root.go`**: Initialize file logging system (`log.Init`) with `defer log.Close()`; register `tshoot` subcommand group; add `commandOperationLog()` to record operation logs after command execution; inject TraceID into command context via `rootCmd.SetContext()`
- **Enhanced `internal/tmeet.go`**: Inject TraceID via `context.WithValue` in `NewTmeet()`; add `TCtx` field
- **Enhanced REST proxy** (`internal/proxy/rest-proxy/proxy.go`): Add file logging for HTTP requests and responses
- Updated `README.md` / `README_EN.md` with `tshoot log` command usage instructions
- Updated `skills/tmeet-skill/` with new `tmeet-tshoot.md` reference documentation

## [v1.0.3] - 2026-04-16

### Added

- **Auto-open browser on `auth login`**: After obtaining the OAuth2 authorization URL, `tmeet auth login` now automatically attempts to open the system default browser to the authorization page, eliminating the need for users to manually copy and paste the URL
- **New `--no-browser` flag for `auth login`**: Added `--no-browser` option to disable auto-opening the browser; when set, only the authorization URL is printed (preserving the original behavior)
- **Cross-platform browser opening** (`internal/core/browser`): New platform-specific implementations:
  - macOS (`browser_darwin.go`): Uses `open` command
  - Windows (`browser_windows.go`): Uses `rundll32 url.dll,FileProtocolHandler`
  - Linux (`browser_other.go`): Uses `xdg-open` with graphical environment detection (`DISPLAY` / `WAYLAND_DISPLAY`)
- Unit tests for browser package using `fakeExecCommand` pattern to mock system commands

### Changed

- Updated `README.md` / `README_EN.md` auth login documentation to describe auto-open browser behavior and the new `--no-browser` flag
- Updated `SKILL.md` with `--no-browser` usage example and Hermes agent-specific note for environments without a default browser
- Updated `tmeet-auth.md` parameter table to document the `--no-browser` option

## [v1.0.2] - 2026-04-15

### Added

- **Enum mapping for API responses**: New `internal/utils/enumerate` package with human-readable name mappings for API enum fields, including `meeting_type`, `meeting_status`, `meeting_recurring_type`, `meeting_recurring_until_type`, `meeting_user_role`, `record_state`, `record_type`, `record_audio_detect`, etc.
- **Time-range filtering**: New `FilterMeetingsByTimeRange` utility (`internal/utils/filter.go`) to filter meeting lists by start/end time boundaries
- **Path-mode field converter**: Enhanced `ConvertFields` (`internal/utils/converter.go`) to support dot-separated path mode (e.g., `meeting.recurring_rule.recurring_type`) for precise field matching
- Comprehensive unit tests for all new enum mappings, converter, and filter utilities

### Changed

- **Console output optimization**: Replaced `cmd.Println` with `fmt.Fprintln(cmd.OutOrStdout())` / `fmt.Fprintln(cmd.OutOrStderr())` for proper stdout/stderr separation; added structured `FormatPrint` function with `formatOutput` struct supporting `trace_id`, `message`, and `data` fields
- Refactored all command handlers (`cmd/meeting/*`, `cmd/record/*`, `cmd/report/*`) to use the new output methods
- Renamed `internal/utils/json.go` → `internal/utils/converter.go` with enhanced path-mode support
- Removed unused `FormatPrint`/`FormatPrintPretty` from `internal/proxy/rest-proxy/proxy.go` and cleaned up unused imports
- Moved existing `instanceid.go` into the new `enumerate` sub-package
- Revised descriptions in `README.md`, `README_EN.md`, and `skills/tmeet-skill/` references; added security risk warnings

### Fixed

- Removed unused `install` phony target from `Makefile` (Fixes #1)
- Fixed typos across the codebase

## [v1.0.1] - 2026-04-07

### Added

Initial release of the `tmeet` CLI tool for Tencent Meeting (WeMeet).

#### Authentication (`tmeet auth`)
- `auth login` — Log in to tmeet via OAuth2 device-code flow
- `auth logout` — Log out from tmeet
- `auth status` — Show current authentication status

#### Meeting Management (`tmeet meeting`)
- `meeting create` — Create a new meeting (supports recurring meetings)
- `meeting get` — Get meeting details by meeting ID or meeting code
- `meeting list` — List upcoming meetings
- `meeting list-ended` — List ended meetings
- `meeting update` — Update an existing meeting
- `meeting cancel` — Cancel a meeting
- `meeting invitees-list` — Get the invitees list of a meeting

#### Recording Management (`tmeet record`)
- `record list` — List meeting recordings
- `record address` — Get recording download addresses
- `record smart-minutes` — Get AI-generated smart minutes from a recording
- `record transcript-get` — Get transcript details
- `record transcript-paragraphs` — Get transcript paragraphs
- `record transcript-search` — Search transcript content by keyword

#### Meeting Reports (`tmeet report`)
- `report participants` — Get the participants list of a meeting
- `report waiting-room-log` — Get the waiting room members log

#### Other Features
- AES-256-GCM encrypted credential storage (no plaintext on disk)
- Cross-platform support (macOS / Linux / Windows)
- File-lock based concurrent access protection
- Symlink attack prevention
- Output format support: `json` (default) and `json-pretty`
