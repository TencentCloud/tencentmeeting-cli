# Changelog

All notable changes to tmeet will be documented in this file, following the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) convention.

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
