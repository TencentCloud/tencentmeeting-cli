# Changelog

All notable changes to tmeet will be documented in this file, following the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) convention.

## [v1.0.11] - 2026-07-08

### Added

- **AI agent identity tracking via `AgentConfig`** (`internal/config/agent.go`, `internal/config/agent_test.go`): New module that records the calling AI agent name and LLM model in plaintext `agent.json`, separate from user credentials. Provides atomic write (tmp + rename), idempotent clear, and `GetAgentConfig` / `SaveAgentConfig` / `ClearAgentConfig` APIs. Full unit test coverage (9 test cases: missing file, round-trip, nil save, overwrite, clear, idempotent clear, corrupted JSON, path resolution, atomic write content).
- **`auth login` now persists agent identity** (`cmd/auth/login.go`): After successful login, reads `TMEET_AGENT` / `TMEET_MODEL` environment variables and saves them to `agent.json`. Write failure is logged but non-fatal — login succeeds regardless.
- **`auth status` now displays `UserName`** (`cmd/auth/status.go`): When the refresh token is still valid, calls `/v1/cli/get-user-info` to fetch and display the user's name alongside `OpenId`. Re-reads user config after the API call to reflect any token refresh that occurred in-flight.
- **`meeting update` supports `--sub-meeting-id` for single sub-meeting modification** (`cmd/meeting/update.go`): New `--sub-meeting-id` flag (recurring meetings only) modifies a single sub-meeting's start/end time without altering the recurring rule. Mutually exclusive with `--recurring-type` / `--until-type` / `--until-count` / `--until-date` — validated at runtime with a clear error message. Extracted `buildRecurringRule()` helper for cleaner payload assembly.
- **`Tmeet-Cli-Name` request header and ApiCmd context passthrough** (`internal/proxy/rest-proxy/proxy.go`, `internal/cmdutil/middleware/api_cmd.go`): The REST proxy now attaches a `Tmeet-Cli-Name` header carrying the resolved ApiCmd identifier. New `InjectApiCmdContext` / `GetApiCmdFromContext` utilities use an unexported key type. The `WithApiCmd` middleware writes the resolved ApiCmd to both the cobra annotation and `cmd.Context()`, decoupling the proxy from annotation reads.
- **New ApiCmd constants for tshoot commands** (`internal/cmdutil/api_schema.go`): `ApiCmdTshootLogUpload` (covers the two-step log upload flow) and `ApiCmdTshootFeedback`. `tshoot feedback` is now wrapped with `WithApiCmd` middleware; `tshoot log` injects `ApiCmdTshootLogUpload` into context before the upload flow.

### Changed

- **`AccessToken` proactive refresh with 60-second leeway** (`internal/auth/auth.go`): Introduced `accessTokenRefreshLeeway = 60s`. `RefreshToken` now triggers 60s before the declared expiry to absorb clock skew and in-flight network latency, preventing server-side `TokenExpired` rejection that the rest-proxy treats as fatal (clears local credentials).
- **`GetSystemInfo` accepts `*config.AgentConfig`** (`internal/common/system.go`, `internal/common/system_test.go`): Agent and Model are now resolved via env var (`TMEET_AGENT` / `TMEET_MODEL`) → `agent.json` fallback → empty string. All test call sites updated to pass `nil`.
- **`control kick --allow-rejoin` default flipped to `true`** (`cmd/control/kick.go`): The flag now defaults to `true` (allow rejoin). Users must explicitly pass `--allow-rejoin=false` to disallow rejoin. Help text updated accordingly.
- **`internal/tmeet.go` loads `AgentConfig` at startup** (`internal/tmeet.go`): Missing `agent.json` is treated as "not set" and does not block CLI startup. `GetSystemInfo` is called with the loaded config so agent/model metadata is available for all subsequent REST requests.
- **Documentation and skill references updated** (`README.md`, `README_EN.md`, `skills/tmeet-skill/SKILL.md`, `skills/tmeet-skill/references/tmeet-auth.md`, `skills/tmeet-skill/references/tmeet-control.md`, `skills/tmeet-skill/references/tmeet-meeting.md`): Added `--sub-meeting-id` parameter tables, usage examples, and updated `--allow-rejoin` default value across all documentation surfaces. `auth status` output example now includes `UserName`. `control kick` description simplified in `SKILL.md`.

## [v1.0.10] - 2026-06-19

### Added

- **`meeting create` / `meeting update` now expose four advanced meeting-settings flags** (`cmd/meeting/create.go`, `cmd/meeting/update.go`): `--water-mark-type` (text watermark, `0`-single row / `1`-double row / `2`-off, default `2`; when not `2`, `allow_screen_shared_watermark` is set in tandem), `--audio-watermark`, `--auto-record-type` (`none` / `local` / `cloud`), and `--auto-asr`. These fields are only written to the request `settings` when `cmd.Flags().Changed(...)` reports they were explicitly provided, so unspecified flags will not silently override the enterprise defaults; explicitly turning a bool flag off requires the `=` form, e.g. `--audio-watermark=false`.
- **Hints output channel for enterprise-locked settings** (`internal/cmdutil/hints/`, `internal/cmdutil/meeting.go`, `internal/output/`): Introduced the `HintProvider` interface and `output.WithHints(fn)` option, and added a top-level `hints` field to `formatOutput`. `meeting create` / `meeting update` plug into this channel: when the response `settings.corp_lock_mask` is non-zero, the output emits messages like `Enterprise has locked the [Text Watermark] setting`, covering text watermark / audio watermark / auto recording / auto speech recognition. Both top-level `settings` and `meeting_info_list[].settings` response shapes are supported, with duplicate entries deduplicated.
- **`corp_lock_mask` bitmask enum** (`internal/utils/enumerate/corp_lock_mask.go`): Defines four constants (bit0~bit3) with `CorpLockMaskName` (single bit → label) and `CorpLockMaskNames` (combined mask → labels in canonical bit order). Companion unit tests in `corp_lock_mask_test.go`.
- **`meeting_join_type` / `show_all_sub_meetings` enum mappings** (`internal/utils/enumerate/`): `1=all` / `2=invited` / `3=internal` and `0=no` / `1=yes`. The new `MeetingJoinTypeConverter` / `ShowAllSubMeetingsConverter` are wired into `meeting list` (for `only_user_join_type`, `is_show_all_sub_meetings`) and into `meeting create` / `meeting update` outputs (for `only_user_join_type`).
- **New `utils.DeleteFields` JSON blacklist utility and `output.WithFilterFields` output option** (`internal/utils/filter.go`, `internal/output/options.go`): The dual of the v1.0.5 `KeepFields` whitelist — supports both field-name mode and dot-path mode (arrays auto-expand) and uses `maxDepth` to bound recursion; `WithFilterFields` exposes it to the output layer with `maxDepth=10`.

### Changed

- **`meeting update` help-text fix** (`cmd/meeting/update.go`): Corrected the `--recurring-type` help text from `1-weekday` to `1-weekdays`, aligning it with `meeting create` and the actual semantics.
- **README and SKILL references synced for the new advanced meeting-settings flags** (`README.md` / `README_EN.md` / `skills/tmeet-skill/references/tmeet-meeting.md`): Parameter tables and examples for `--water-mark-type` / `--audio-watermark` / `--auto-record-type` / `--auto-asr` added to the `meeting create` / `meeting update` sections, with two notes called out: (1) personal vs. enterprise (organization) accounts behave differently depending on whether the enterprise has set the option to "forced"; (2) explicitly turning a bool flag off requires the `=false` form.
- **SKILL bumped to 1.0.6** (`skills/tmeet-skill/SKILL.md`).

## [v1.0.9] - 2026-06-12

### Added

- **`meeting create` now supports inviting participants at creation time** (`cmd/meeting/create.go`): A new `--invitees` flag accepts an `open_id` list (comma-separated or repeated, max 100), reusing v1.0.8's `cmdutil.PackageApiInviteesUsers` for dedup and upper-bound checks.
- **`meeting update` now supports mutating the invitee list as part of the update call** (`cmd/meeting/update.go`): Added the `--invitees` + `--invitees-type` flag pair, supporting `add` / `remove` / `replace` strategies; both flags must be set together. Complements the v1.0.8 `meeting invitees-add/remove/replace` subcommands by avoiding a second round-trip when the meeting body and the invitee list change in the same operation.

### Changed

- **REST proxy error handling and retry policy adjusted** (`internal/proxy/rest-proxy/proxy.go`): HTTP `408` / `504` gateway timeouts are now classified as `NetworkError` to trigger retries; all other non-200 responses (except token expiration) are uniformly classified as `NotRetryRequestError` and no longer retried. The "should-retry" decision shifts from a server-business-code allowlist to the more conservative "anything other than a network error is not retried", reducing the risk of write operations (calls, kicks, invitee mutations) being silently replayed.
- **README and SKILL references synced for the new invitation flags** (`README.md` / `README_EN.md` / `skills/tmeet-skill/references/tmeet-meeting.md`): Parameter tables and examples for `--invitees` / `--invitees-type` added to the `meeting create` / `meeting update` sections.
- **SKILL bumped to 1.0.5** (`skills/tmeet-skill/SKILL.md`).

### Removed

- **Removed the "specific business-code non-retry" allowlist mechanism introduced in v1.0.7** (`internal/exception/server_code.go`): Deleted the `ServerCodeRecordNotExist (500277)` constant, the `notRetryCodeMap`, and the `IsNotRetryCode` function; their semantics are now subsumed by the broader "non-200 is never retried" policy above.

## [v1.0.8] - 2026-06-11

### Added

- **New `contact` subcommand group** (`cmd/contact/`): Address-book lookup capabilities, primarily intended to resolve names ↔ `open_id` for the "meeting invitation" and "in-meeting call" scenarios.
  - `contact search` (`cmd/contact/search.go`) — Search enterprise address-book members by username, with optional job-title / department filtering when the username matches too many people; when only a single member is matched, the response is trimmed to keep `open_id` only (via the new `output.WithContactSearchLogic` hook).
    - `--username` (required) — Username to search
    - `--job-title` (optional) — Job title filter when the username yields too many matches
    - `--department-name` (optional) — Department filter when the username yields too many matches
  - `contact lookup-by-phone` (`cmd/contact/lookup_by_phone.go`) — Batch-look up users by phone number; up to 50 numbers per call; each number is pre-validated by `utils.ValidatePhone` before the request is sent.
    - `--phones` (required) — Phone-number list, comma-separated or repeat the flag, max 50
  - `contact lookup-by-email` (`cmd/contact/lookup_by_email.go`) — Batch-look up users by email address; up to 50 emails per call; each email is pre-validated by `utils.ValidateEmail` before the request is sent.
    - `--emails` (required) — Email list, comma-separated or repeat the flag, max 50
- **New `control` subcommand group** (`cmd/control/`): In-meeting real-time control capabilities.
  - `control call` (`cmd/control/call.go`) — In-meeting batch call to bring members into the meeting, up to 20 members per call. Classified as a **write operation** and listed in the SKILL dangerous-operations table.
    - `--meeting-id` (required) — Meeting ID
    - `--users` (required) — Member `open_id` list to call
  - `control kick` (`cmd/control/kick.go`) — Kick members out of an ongoing meeting, supporting three identity types (regular member / SIP / PSTN), capped at 20 in total per call. Classified as a **write operation**, listed in the SKILL dangerous-operations table, and SKILL further enforces a hard rule that the kick targets **must** come from `report participants` and **must not** come from any `contact` lookup result.
    - `--meeting-id` (required) — Meeting ID
    - `--allow-rejoin` (optional) — Allow kicked members to re-join
    - `--users` / `--sip-users` / `--pstn-users` (at least one required) — Map respectively to regular members' `open_id`, SIP devices' `ms_open_id` (`instanceid=9`), and PSTN devices' `ms_open_id` (`instanceid=0`)
- **New invitee-management subcommands under `meeting`** (`cmd/meeting/`): Building on the existing `invitees-list` from v1.0.7, this release adds incremental add / remove / full-replace capabilities.
  - `meeting invitees-add` (`cmd/meeting/invitees_add.go`) — Append invitees to an existing meeting.
  - `meeting invitees-remove` (`cmd/meeting/invitess_remove.go`) — Remove specified invitees from an existing meeting. Classified as a **write operation** and listed in the SKILL dangerous-operations table.
  - `meeting invitees-replace` (`cmd/meeting/invitees_replace.go`) — Replace the meeting's invitee list with a new one; an empty list clears all invitees. Classified as a **write operation** and listed in the SKILL dangerous-operations table.
  - All three share `--meeting-id` (required) and `--invitees` (the `open_id` list to apply, comma-separated or repeat the flag, max 100); `--invitees` is required for `invitees-add` / `invitees-remove` and optional (empty meaning "clear all") for `invitees-replace`.
- **New shared user-list packagers for invitee / in-meeting-control commands** (`internal/cmdutil/users_help.go`): Extracted three reusable helpers — `PackageApiInviteesUsers` (invitees, capped by `InviteesListMax = 100`), `PackageMeetingControlUsers` (regular members, `open_id`, `to_operator_id_type=2`), and `PackageMeetingControlSpecialUsers` (SIP / PSTN, `ms_open_id`, `to_operator_id_type=4` plus `instanceid`) — that uniformly handle dedup, empty-value validation, and upper-bound checks. The constant `MeetingControlUsersListMax = 20` is also defined here.
- **New phone / email format validation utilities** (`internal/utils/validate.go`):
  - `ValidatePhone` — 11-digit phone-number regex check (`^1[3-9]\d{9}$`).
  - `ValidateEmail` — Total length ≤ 100; must contain `@` with non-empty local / domain parts; rejects quoted local part, consecutive dots, and IP-address domains; finalizes with a regex format check.
  - `SplitAndTrim` — Generic "split-by-comma + trim + drop empty" helper.
  - These helpers are wired in as pre-call validators for `cmd/contact/lookup_by_phone.go` / `cmd/contact/lookup_by_email.go`.
  - Companion unit tests in `internal/utils/validate_test.go` cover valid, invalid, and boundary cases for both formats.
- **New `output.WithContactSearchLogic` output option** (`internal/output/options.go`): Applied to `contact search` responses; when the `users` array contains exactly one entry, only the `open_id` field is preserved, avoiding leakage of unnecessary personal information to the agent (mirroring the SKILL constraint that the address book is not a general-purpose people-info lookup).
- **New API Schema identifiers** (`internal/cmdutil/api_schema.go`): Added the ApiCmd constants `meeting_invite_add` / `meeting_invite_remove` / `meeting_invite_replace` / `contact_search` / `contact_lookup_by_phone` / `contact_lookup_by_email` / `control_call` / `control_kick`, so the new commands can plug into the v1.0.5 `--compact` and middleware pipeline.

### Changed

- **Root command registers the `contact` and `control` subcommand groups** (`cmd/root.go`): `contact.NewBaseCmd` and `control.NewBaseCmd` are added between the existing `auth` / `meeting` / `report` / `record` / `tshoot` registrations.
- **SKILL bumped to 1.0.4** (`skills/tmeet-skill/SKILL.md`):
  - Top-level `description` now mentions the "address book" and "in-meeting control" capabilities.
  - The command tree is updated with `meeting invitees-add/remove/replace`, the new `contact` group, and the new `control` group.
  - The dangerous-operations table now includes `meeting invitees-remove`, `meeting invitees-replace`, `control call`, and `control kick`, all requiring user confirmation before execution.
  - Added a **"`contact search` is restricted to specific scenarios"** rule: it may only be used for meeting invitations, in-meeting calls, and back-filling invitee names — not as a general-purpose people-info lookup.
  - Added a **"hard rule on kick-target source"**: the `open_id` / `ms_open_id` passed to `control kick` **must** come from `report participants`, and **must not** come from any `contact` lookup result.
  - Added a **"multiple results require user confirmation"** rule: when commands such as `contact search` return more than one candidate, the model must list them and let the user choose, rather than picking one on its own.
  - Added a **"reply template for meeting-invitee mutations"**: after `meeting invitees-add/remove/replace` succeeds, the reply must follow a strict "topic / time / meeting code / join URL / current invitees" template, and the invitee list must show **names** rather than `open_id`s, looking them up via `meeting invitees-list` + `contact search` when needed.
- **SKILL reference docs added / synced** (`skills/tmeet-skill/references/`): Added `tmeet-contact.md` and `tmeet-control.md`; `tmeet-meeting.md` is updated with parameter tables and usage notes for `invitees-add` / `invitees-remove` / `invitees-replace`.
- **README sync** (`README.md` / `README_EN.md`): Command tree and command reference updated with `meeting invitees-add/remove/replace`, `contact search/lookup-by-phone/lookup-by-email`, and `control call/kick`, including the corresponding parameter descriptions and examples.

## [v1.0.7] - 2026-06-10

### Added

- **New `record permission-apply-prepare` command** (`cmd/record/permission_apply_prepare.go`): When `record address` / `record smart-minutes` / `record transcript-*` fail due to missing permission, this command fetches the approval preview (approval text, meeting topic, recording owner, request notes, etc.) so the agent can show it to the user for confirmation before any write call.
  - `--meeting-record-id` (required) — Meeting recording ID
  - `--meeting-id` (optional) — Meeting ID
- **New `record permission-apply-commit` command** (`cmd/record/permission_apply_commit.go`): Submits the recording-permission approval after user confirmation; returns `unique_id` / `status` / `approval_url` / `share_text`. Classified as a **write operation** and listed in the SKILL dangerous-operations table.
  - `--meeting-record-id` (required) — Must match the value used in `prepare`
  - `--meeting-id` (optional) — Meeting ID
- **Non-retryable server error mechanism** (`internal/exception/`): Introduced `ServerCodeRecordNotExist (500277)` together with `notRetryCodeMap` / `IsNotRetryCode(code)`; added a new client-side error `NotRetryRequestError` and error code `ClientNotRetryRequest (2008)`.

### Changed

- **REST proxy retry policy** (`internal/proxy/rest-proxy/proxy.go`): `RetryIf` now also excludes `NotRetryRequestError` in addition to `TokenExpiredError`; server-side business errors that hit `IsNotRetryCode` are surfaced as `NotRetryRequestError` immediately, avoiding pointless retries on cases such as "recording does not exist".
- **Documentation sync**: `README.md` / `README_EN.md` command tree and command reference updated with the two new permission-apply commands; `skills/tmeet-skill/SKILL.md` bumped to `1.0.3` with a "recording permission apply" capability note and `record permission-apply-commit` added to the dangerous-operations list requiring user confirmation; `skills/tmeet-skill/references/tmeet-record.md` adds the "permission apply workflow" (prepare → user confirm → commit → show `approval_url`) and error-handling guidance.

## [v1.0.6] - 2026-06-03

### Added

- **New `tshoot feedback` command** (`cmd/tshoot/feedback.go`): Lets the agent (or end user) report troubleshooting feedback to the server, closing the loop between CLI usage and product iteration.
  - `--category` (required) — Feedback category, one of: `tool_not_found` / `tool_error` / `tool_inadequate` / `unexpected_result` / `suggestion`
  - `--intent` (required) — Original intent of the agent (max 200 characters)
  - `--actions-tried` — Actions the agent has already tried (max 500 characters)
  - `--result` — Result or blocker of the tried actions (max 500 characters)
  - `--tool-name` — Tool/command name involved in the feedback
  - `--error-code` — Error code returned by the tool, if any
- **New `CharacterLimit` string validation utility** (`internal/utils/string.go`): UTF-8 rune-count based length check; returns `InvalidArgsError` with the offending flag name, the configured limit, and the actual length when exceeded. Used by `tshoot feedback` to enforce the `--intent` / `--actions-tried` / `--result` length caps.
- Unit tests for `CharacterLimit` covering ASCII, multi-byte (CJK), boundary, and over-limit cases (`internal/utils/string_test.go`).

### Changed

- **Error logging on failure paths**: `cmd/root.go` now logs `execute failed: %v` via `log.Errorf` whenever `rootCmd.Execute()` returns a non-nil error, and `internal/auth/auth.go` logs `refresh token failed: %v` before clearing the local config in `TmeetAuth.RefreshToken`. Combined with the file-based logging introduced in v1.0.4, these traces are now recoverable through `tmeet tshoot log`.
- `cmd/tshoot/base.go` registers the new `feedback` subcommand alongside `log` under the `tshoot` group.
- Updated `README.md` / `README_EN.md`, `skills/tmeet-skill/SKILL.md`, and `skills/tmeet-skill/references/tmeet-tshoot.md` with `tshoot feedback` usage, parameter tables, and category guidance.

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