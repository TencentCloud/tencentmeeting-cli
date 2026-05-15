# Changelog

All notable changes to tmeet will be documented in this file, following the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) convention.

## [v1.0.5] - 2026-05-16

### Added

- **New global `--compact` output mode**: Added a persistent `--compact` flag on the root command. When enabled, responses keep only the key fields advertised by the server and strip redundant content. Wired into `meeting list` / `meeting list-ended` / `meeting invitees-list` / `record list` / `report participants` / `report waiting-room-log`.
- **New Cobra command middleware framework** (`internal/cmdutil/middleware/`): Provides `Chain` (onion-model composition), `WithApiCmd` (resolves the ApiCmd and writes it to the command annotation), and `WithCompact` (fetches the schema for the resolved ApiCmd and propagates the compact-field whitelist via context). All API-backed subcommands' `RunE` are now built on this chain.
- **New API Schema module** (`internal/cmdutil/`): Pulls per-ApiCmd compact fields, field metadata, and cache policy from `/v1/api/compact-schema`. Ships with `ApiCmdResolver` (`StaticApiCmd` / `FlagSwitch` / `ResolverFunc`), command annotation helpers (`annotations.go`), and pagination helpers (`pagination.go`: `ClampingPageSize` / `ChoosePageOrToken` / `ChoosePosOrToken`).
- **New local file cache primitive** (`internal/core/filecache/`): TTL-based file KV cache; cross-process safety via `core/filelock` write locks, in-process safety via per-key mutex; writes use temp-file + rename for atomic replacement; corrupted files transparently degrade to a miss. Used as the on-disk store for API Schema responses.
- **New `KeepFields` JSON whitelist utility** (`internal/utils/filter.go`): Recursively keeps only the specified fields, supporting both field-name mode and dot-path mode (e.g. `meeting.recurring_rule.recurring_type`); the underlying primitive that powers `--compact` field trimming.
- **New `MeetingUserJoinRoleConverter` enum mapping**: Converts `creator` / `hoster` / `invitee` into "创建者 / 主持人 / 被邀请者" Chinese display names; applied in `meeting list`.
- **Options-style output layer**: `output.FormatPrint` now accepts variadic `...Option`, with two built-ins — `WithCompact` and `WithConvert` — moving compact trimming and field conversion out of individual commands and into the output layer.
- Unit tests covering the middleware, file cache (concurrent writes, expiration, corruption fallback, cross-process locking), `KeepFields`, and the new enum.

### Changed

- **Unified pagination flags to `--page-token` + `--page-size`**: All paginated commands now use the new pagination protocol (`page_type=1`); the legacy `--page` / `--pos` flags are marked **deprecated** (still honored but with lower priority than `--page-token`). `--page-size` defaults / caps are aligned accordingly:
  - `meeting list` / `meeting list-ended` / `meeting invitees-list` / `record list` / `record address`: default raised to `30`, max unified to `30`;
  - `report participants` / `report waiting-room-log`: default raised to `100`, max `100`.
- **All API subcommands now flow through the middleware chain**: `RunE` is built as `middleWare.Chain(opts.Run, WithApiCmd(...), WithCompact(...))`, and the per-command `utils.ConvertFields(...)` calls have been pushed down into `output.FormatPrint(..., WithConvert(...))`.
- **Field mapping tweaks for `meeting list`**: Removed the base64 decoding of `time_zone` (the server now returns plaintext); added the `join_meeting_role` Chinese mapping; dropped the no-longer-required `cursory` query parameter.
- **Annotation migration for `auth login` / `auth status`**: The hand-written `cmd.Annotations = map[string]string{"skipPreCheck": "true"}` has been replaced with `cmdutil.InjectSkipPreCheckAnnotation(cmd)` so that annotation keys are managed in one place.
- Updated `README.md` / `README_EN.md` with a new "Pagination" section, the `--compact` description, and a per-command `--page-size` default/max quick-reference table; `skills/tmeet-skill/` reference docs synced to the new pagination flags and defaults.

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
