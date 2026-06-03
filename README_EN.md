# tencentmeeting-cli

[中文](README.md) | English

A command-line interface (CLI) tool for Tencent Meeting, based on Tencent Meeting Open Platform OAuth2 authorization. Supports meeting management, recording management, attendance reports, and more.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)

## Features

- 🔐 **OAuth2 Authorization** — Device code authorization flow, secure and passwordless
- 📅 **Meeting Management** — Create, query, update, and cancel meetings; supports recurring meetings and invitee management
- 🎬 **Recording Management** — Query recording lists, get download URLs, smart minutes, transcript details and search
- 📊 **Attendance Reports** — Query participant lists and waiting room member records
- 🛠️ **Troubleshooting** — Export local logs with optional time range filter, packaged as a zip file
- 🔒 **Secure Storage** — Credentials encrypted with AES-256-GCM, no plaintext stored on disk
- 🖥️ **Cross-Platform** — Supports macOS, Linux, and Windows

## Installation

### Step 1: Install CLI

#### Option 1: Install via npm (Recommended)

```bash
npm install -g @tencentcloud/tmeet
```

After installation, the `tmeet` command is available directly.

> 💡 If you see `npm: command not found`, it means Node.js is not installed. Please visit the [Node.js official website](https://nodejs.org/) to download and install the LTS version (npm is included).

#### Option 2: Build from Source

```bash
git clone https://github.com/TencentCloud/tencentmeeting-cli
cd tencentmeeting-cli
go build -ldflags "-X tmeet/cmd.Version=v1.0.0" -o tmeet .
# or
make build VERSION=v1.0.0
```

### Step 2: Install CLI-SKILL

```bash
npx skills add TencentCloud/tencentmeeting-cli -y -g
```

## Quick Start

### 1. Login & Authorization

```bash
tmeet auth login
```

This automatically attempts to open the system default browser to the authorization URL. If no default browser is available, it prints the URL for you to open manually. The CLI polls for the result automatically (5-minute timeout) and saves the credentials encrypted locally.

> To disable auto-opening the browser, use the `--no-browser` flag: `tmeet auth login --no-browser`

### 2. Create a Meeting

```bash
tmeet meeting create \
  --subject "Weekly Standup" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"
```

### 3. List Meetings

```bash
# List ongoing or upcoming meetings
tmeet meeting list

# List ended meetings
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"
```

### 4. Logout

```bash
tmeet auth logout
```

---

## Global Flags

All commands support the following global flags:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | — | `json` | Output format: `json` (compact) \| `json-pretty` (indented) |
| `--compact` | — | `false` | Compact output mode: keeps only key fields and filters out redundant ones to reduce response size; recommended for query/list commands |
| `--version` | `-V` | — | Show version number |

**Examples:**

```bash
# Show version number
tmeet -V

# Output response in indented format
tmeet meeting get --meeting-id "6953553464429888300" --format json-pretty

# Output query results in compact mode (only key fields)
tmeet record list --meeting-id "6953553464429888300" --compact
```

---

## Pagination

Starting from `v1.0.5`, all list/query commands that support pagination use a unified **`--page-token` + `--page-size`** model. The legacy `--page` / `--pos` / `--pid` / `--size` / `--limit` flags are marked as **deprecated** — they still work for backward compatibility but are discouraged and may be removed in a future release.

**Unified usage:**

| Flag | Type | Description |
|------|------|-------------|
| `--page-token` | string | Pagination cursor. **Omit on the first request**; for subsequent pages, pass the `next_page_token` returned by the previous response |
| `--page-size` | int | Items per page. Defaults and upper limits vary by command (see per-command docs below) |

**Typical pagination flow:**

```bash
# 1) First request (no page-token)
tmeet record list --meeting-id "6953553464429888300" --page-size 30

# 2) Take next_page_token from the response and request the next page
tmeet record list \
  --meeting-id "6953553464429888300" \
  --page-size 30 \
  --page-token "<next_page_token>"

# 3) Repeat until next_page_token is empty (last page reached)
```

**`--page-size` defaults / maximums per command:**

| Command | Default | Max | Legacy flag (deprecated) |
|---------|:-------:|:---:|-------------------------|
| `meeting list` |   20    | 20 | — |
| `meeting list-ended` |   30    | 30 | `--page` |
| `meeting invitees-list` |   30    | 30 | `--pos` |
| `record list` |   30    | 30 | `--page` |
| `record address` |   30    | 30 | `--page` |
| `report participants` |   100   | 100 | `--pos` / `--size` |
| `report waiting-room-log` |   100   | 100 | `--page` |

> `record transcript-get` / `record transcript-paragraphs` / `record transcript-search` do not support the new `--page-token` based pagination.
>
> Compatibility: when `--page-token` is not provided but a legacy flag (e.g. `--page`, `--pos`) is set, the CLI falls back to the legacy mode (`page_type=0`); otherwise the new mode (`page_type=1`) is used.

---

## Command Overview

```
tmeet [--format json|json-pretty] [--compact] [-V]
├── auth
│   ├── login          # OAuth authorization login
│   ├── logout         # Logout and clear credentials
│   └── status         # View current login status
├── meeting
│   ├── create         # Create a meeting (regular or recurring)
│   ├── update         # Update meeting information
│   ├── cancel         # Cancel a meeting
│   ├── get            # Get meeting details
│   ├── list           # List ongoing or upcoming meetings
│   ├── list-ended     # List ended meetings
│   └── invitees-list  # List meeting invitees
├── record
│   ├── list           # Query recording list
│   ├── address        # Get recording file download URL
│   ├── smart-minutes  # Get smart minutes
│   ├── transcript-get        # Get transcript details
│   ├── transcript-paragraphs # Get transcript paragraph list
│   └── transcript-search     # Search transcript content
├── report
│   ├── participants   # Get participant list
│   └── waiting-room-log # Get waiting room member list
└── tshoot
    ├── log            # Export local logs (supports time range filter, optional --upload to server)
    └── feedback       # Report troubleshooting feedback to the server
```

---

## Command Reference

### auth — Authorization Management

#### `auth login`

Login and complete OAuth2 authorization, saving credentials encrypted locally.

```bash
tmeet auth login [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--no-browser` | bool | — | `false` | Disable auto-opening the browser. `false` (default) attempts to open the system default browser to the authorization URL automatically; `true` only prints the authorization URL and requires the user to open it manually. |

After execution, the authorization URL is printed. The CLI polls for the authorization result automatically (timeout: 5 minutes) and saves the credentials encrypted locally.

---

#### `auth logout`

Logout and clear local authentication credentials.

```bash
tmeet auth logout
```

> No parameters.

---

#### `auth status`

View current login status, including OpenId, AccessToken / RefreshToken expiration status and remaining validity time.

```bash
tmeet auth status
```

> No parameters. Displays `Not logged in` when not authenticated; shows credential validity information when logged in.

---

### meeting — Meeting Management

#### `meeting create` — Create a Meeting

```bash
tmeet meeting create --subject <title> --start <start-time> --end <end-time> [options]
```

| Parameter | Type | Required | Default | Description                                                                                               |
|-----------|------|:--------:|---------|-----------------------------------------------------------------------------------------------------------|
| `--subject` | string | ✅ | — | Meeting subject/title                                                                                     |
| `--start` | string | ✅ | — | Meeting start time, ISO 8601, e.g. `2026-03-12T14:00+08:00`                                               |
| `--end` | string | ✅ | — | Meeting end time, ISO 8601, e.g. `2026-03-12T15:00+08:00`                                                 |
| `--password` | string | — | — | Meeting password (4–6 digits)                                                                             |
| `--timezone` | string | — | — | Timezone, refer to Oracle-TimeZone standard, e.g. `Asia/Shanghai`                                         |
| `--meeting-type` | int | — | `0` | Meeting type: `0`-regular meeting, `1`-recurring meeting                                                  |
| `--join-type` | int | — | `0` | Join restriction: `1`-all members, `2`-invited members only, `3`-internal members only                    |
| `--waiting-room` | bool | — | `false` | Enable waiting room: `true`-enable, `false`-disable                                                       |
| `--recurring-type` | int | — | `0` | Recurrence type (when `--meeting-type=1`): `0`-daily, `1`-weekdays, `2`-weekly, `3`-biweekly, `4`-monthly |
| `--until-type` | int | — | `0` | Recurrence end type (when `--meeting-type=1`): `0`-end by date, `1`-end by count                          |
| `--until-count` | int | — | `7` | Max occurrences (when `--meeting-type=1`): max 500 for daily/weekday/weekly; max 50 for biweekly/monthly  |
| `--until-date` | string | — | — | Recurrence end date (when `--meeting-type=1`), ISO 8601, e.g. `2026-03-12T15:00+08:00`                    |

**Examples:**

```bash
# Create a regular meeting
tmeet meeting create \
  --subject "Project Review" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T16:00+08:00" \
  --password "123456" \
  --waiting-room

# Create a weekly recurring meeting (10 occurrences)
tmeet meeting create \
  --subject "Weekly Standup" \
  --start "2026-04-10T09:30+08:00" \
  --end "2026-04-10T10:00+08:00" \
  --meeting-type 1 \
  --recurring-type 2 \
  --until-type 1 \
  --until-count 10
```

---

#### `meeting get` — Get Meeting Details

Use either `--meeting-id` or `--meeting-code` (one required); `--meeting-id` takes priority.

```bash
tmeet meeting get --meeting-id <meeting-id>
tmeet meeting get --meeting-code <meeting-code>
```

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `--meeting-id` | string | one of two | Meeting ID (higher priority than meeting code) |
| `--meeting-code` | string | one of two | Meeting code |

**Examples:**

```bash
tmeet meeting get --meeting-id "6953553464429888300"
tmeet meeting get --meeting-code "931945029"
```

---

#### `meeting update` — Update a Meeting

Only pass the fields you want to modify; unspecified fields remain unchanged.

```bash
tmeet meeting update --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description                                                                                               |
|-----------|------|:--------:|---------|-----------------------------------------------------------------------------------------------------------|
| `--meeting-id` | string | ✅ | — | Meeting ID                                                                                                |
| `--subject` | string | — | — | Meeting subject/title                                                                                     |
| `--start` | string | — | — | Meeting start time, ISO 8601, e.g. `2026-03-12T14:00+08:00`                                               |
| `--end` | string | — | — | Meeting end time, ISO 8601, e.g. `2026-03-12T14:00+08:00`                                                 |
| `--password` | string | — | — | Meeting password (4–6 digits)                                                                             |
| `--timezone` | string | — | — | Timezone, e.g. `Asia/Shanghai`                                                                            |
| `--meeting-type` | int | — | `0` | Meeting type: `0`-regular meeting, `1`-recurring meeting                                                  |
| `--join-type` | int | — | `0` | Join restriction: `1`-all members, `2`-invited members only, `3`-internal members only                    |
| `--waiting-room` | bool | — | `false` | Enable waiting room                                                                                       |
| `--recurring-type` | int | — | `0` | Recurrence type (when `--meeting-type=1`): `0`-daily, `1`-weekdays, `2`-weekly, `3`-biweekly, `4`-monthly |
| `--until-type` | int | — | `0` | Recurrence end type (when `--meeting-type=1`): `0`-end by date, `1`-end by count                          |
| `--until-count` | int | — | `7` | Max occurrences (when `--meeting-type=1`): max 500 for daily/weekday/weekly; max 50 for biweekly/monthly  |
| `--until-date` | string | — | — | Recurrence end date (when `--meeting-type=1`), ISO 8601, e.g. `2026-03-12T15:00+08:00`                    |

**Example:**

```bash
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --subject "New Title" \
  --start "2026-04-10T15:00+08:00" \
  --end "2026-04-10T16:00+08:00"
```

---

#### `meeting cancel` — Cancel a Meeting

```bash
tmeet meeting cancel --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--sub-meeting-id` | string | — | — | Sub-meeting ID for recurring meetings; required when canceling a specific occurrence |
| `--meeting-type` | int | — | `0` | Meeting type: `0`-regular meeting, `1`-recurring meeting (pass `1` to cancel the entire recurring series) |

**Examples:**

```bash
# Cancel a regular meeting
tmeet meeting cancel --meeting-id "6953553464429888300"

# Cancel a specific occurrence of a recurring meeting
tmeet meeting cancel \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "100001"

# Cancel the entire recurring meeting series
tmeet meeting cancel \
  --meeting-id "6953553464429888300" \
  --meeting-type 1
```

---

#### `meeting list` — List Meetings

List ongoing or upcoming meetings.

```bash
tmeet meeting list [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--start` | string | — | — | Pagination start time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--end` | string | — | — | Pagination end time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--show-all-sub` | int | — | `0` | Show all sub-meetings: `0`-no, `1`-yes |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `20` | Page size, default 20, max 20 |

**Examples:**

```bash
tmeet meeting list
tmeet meeting list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --show-all-sub 1

# Fetch the next page
tmeet meeting list --page-token "<next_page_token>" --page-size 20
```

---

#### `meeting list-ended` — List Ended Meetings

Query historical ended meetings with time range pagination support.

```bash
tmeet meeting list-ended [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--start` | string | — | — | Query start time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--end` | string | — | — | Query end time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |
| `--page` | int | — | — | ⚠️ **Deprecated**: page number (starting from 1); use `--page-token` instead |

**Examples:**

```bash
# Query ended meetings this month
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# Paginated query using page-token
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page-token "<next_page_token>" --page-size 30
```

---

#### `meeting invitees-list` — List Meeting Invitees

```bash
tmeet meeting invitees-list --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |
| `--pos` | int | — | — | ⚠️ **Deprecated**: starting position; use `--page-token` instead |

**Examples:**

```bash
tmeet meeting invitees-list --meeting-id "6953553464429888300"

# Fetch the next page
tmeet meeting invitees-list \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 30
```

---

### record — Recording Management

#### `record list` — Query Recording List

Choose **one** of the following three parameter groups (error if none provided):
- `--start` + `--end` (time range)
- `--meeting-id` (meeting ID)
- `--meeting-code` (meeting code)

```bash
tmeet record list (--start <start-time> --end <end-time> | --meeting-id <id> | --meeting-code <code>) [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--start` | string | one of three | — | Query start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | one of three | — | Query end time, ISO 8601, e.g. `2026-03-12T14:00+08:00` (used with `--start`) |
| `--meeting-id` | string | one of three | — | Meeting ID |
| `--meeting-code` | string | one of three | — | Meeting code |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |
| `--page` | int | — | — | ⚠️ **Deprecated**: page number (starting from 1); use `--page-token` instead |

**Examples:**

```bash
# Query by time range
tmeet record list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page-token "<next_page_token>" --page-size 30

# Query by meeting ID
tmeet record list --meeting-id "6953553464429888300"

# Query by meeting code
tmeet record list --meeting-code "931945029"
```

---

#### `record address` — Get Recording Download URL

```bash
tmeet record address --meeting-record-id <record-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-record-id` | string | ✅ | — | Meeting recording ID |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |
| `--page` | int | — | — | ⚠️ **Deprecated**: page number (starting from 1); use `--page-token` instead |

**Examples:**

```bash
tmeet record address --meeting-record-id "record_abc123"

# Fetch the next page
tmeet record address \
  --meeting-record-id "record_abc123" \
  --page-token "<next_page_token>" --page-size 30
```

---

#### `record smart-minutes` — Get Smart Minutes

```bash
tmeet record smart-minutes --record-file-id <file-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--record-file-id` | string | ✅ | — | Recording file ID |
| `--lang` | string | — | `default` | Translation language: `default`-original (no translation), `zh`-Simplified Chinese, `en`-English, `ja`-Japanese |
| `--pwd` | string | — | — | Recording file access password |

**Example:**

```bash
tmeet record smart-minutes --record-file-id "file_abc123" --lang zh
```

---

#### `record transcript-get` — Get Transcript Details

```bash
tmeet record transcript-get --record-file-id <file-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--record-file-id` | string | ✅ | — | Recording file ID |
| `--meeting-id` | string | — | — | Meeting ID |
| `--pid` | string | — | — | Starting paragraph ID |
| `--limit` | string | — | — | Number of paragraphs to query |

**Examples:**

```bash
tmeet record transcript-get --record-file-id "file_abc123"

# Specify starting paragraph and count
tmeet record transcript-get --record-file-id "file_abc123" --pid "<paragraph_id>" --limit "30"
```

---

#### `record transcript-paragraphs` — Get Transcript Paragraph List

```bash
tmeet record transcript-paragraphs --record-file-id <file-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--record-file-id` | string | ✅ | — | Recording file ID |
| `--meeting-id` | string | — | — | Meeting ID |

**Examples:**

```bash
tmeet record transcript-paragraphs --record-file-id "file_abc123"

# Specify meeting ID
tmeet record transcript-paragraphs \
  --record-file-id "file_abc123" \
  --meeting-id "6953553464429888300"
```

---

#### `record transcript-search` — Search Transcript Content

```bash
tmeet record transcript-search --record-file-id <file-id> --text <keyword> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--record-file-id` | string | ✅ | — | Recording file ID |
| `--text` | string | ✅ | — | Search keyword |
| `--meeting-id` | string | — | — | Meeting ID |

**Example:**

```bash
tmeet record transcript-search --record-file-id "file_abc123" --text "quarterly goals"
```

---

### report — Attendance Reports

#### `report participants` — Get Participant List

```bash
tmeet report participants --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description                                                                                 |
|-----------|------|:--------:|---------|---------------------------------------------------------------------------------------------|
| `--meeting-id` | string | ✅ | —       | Meeting ID                                                                                  |
| `--sub-meeting-id` | string | — | —       | Sub-meeting ID for recurring meetings                                                       |
| `--start` | string | — | —       | Query start time, ISO 8601, e.g. `2026-03-12T14:00+08:00`                                   |
| `--end` | string | — | —       | Query end time, ISO 8601, e.g. `2026-03-12T14:00+08:00`                                     |
| `--page-token` | string | — | —       | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `100`   | Page size, default 100, max 100                                                             |
| `--pos` | int | — | —       | ⚠️ **Deprecated**: starting position; use `--page-token` instead                            |
| `--size` | int | — | —       | ⚠️ **Deprecated**: items per page; use `--page-size` instead                                |

**Examples:**

```bash
tmeet report participants --meeting-id "6953553464429888300" --page-size 50
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"

# Fetch the next page
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 50
```

---

#### `report waiting-room-log` — Get Waiting Room Members

```bash
tmeet report waiting-room-log --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description                                                                                 |
|-----------|------|:--------:|---------|---------------------------------------------------------------------------------------------|
| `--meeting-id` | string | ✅ | —       | Meeting ID                                                                                  |
| `--page-token` | string | — | —       | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `100`   | Page size, default 100, max 100                                                             |
| `--page` | int | — | —       | ⚠️ **Deprecated**: page number; use `--page-token` instead                                  |

**Examples:**

```bash
tmeet report waiting-room-log --meeting-id "6953553464429888300" --page-size 50

# Fetch the next page
tmeet report waiting-room-log \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 50
```

---

### tshoot — Troubleshooting

#### `tshoot log` — Export Local Logs

Packages local logs into a zip file and saves it to `~/tmeet_ts_{datetime}.zip`, useful for troubleshooting. Supports optional time range filtering; if no time parameters are provided, all logs are exported.

```bash
tmeet tshoot log [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--start` | string | used with `--end` | — | Log query start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | used with `--start` | — | Log query end time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--upload` | bool | No | `false` | Upload log to server, login required |

> `--start` and `--end` must be provided together or both omitted.

**Examples:**

```bash
# Export all logs
tmeet tshoot log

# Export logs within a specific time range
tmeet tshoot log \
  --start "2026-04-10T00:00+08:00" \
  --end "2026-04-10T23:59+08:00"

# Upload log to server (login required)
tmeet tshoot log --upload
```

Output example:
```
output log saved to: ~/tmeet_ts_20260410_153000.zip
```

---

#### `tshoot feedback` — Report Troubleshooting Feedback

Report issues or suggestions encountered by the Agent while using the CLI to the server, helping to improve tool capabilities.

```bash
tmeet tshoot feedback --category <category> --intent <intent> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--category` | string | ✅ | — | Feedback category. Options: `tool_not_found` (want to do something but cannot find a matching tool), `tool_error` (called a tool but it returned an error), `tool_inadequate` (tool exists but its capability/parameters are insufficient), `unexpected_result` (call succeeded but the result did not meet expectations), `suggestion` (general suggestion or improvement idea) |
| `--intent` | string | ✅ | — | Original intent of the agent, max 200 characters |
| `--actions-tried` | string | — | — | Actions the agent has tried, max 500 characters |
| `--result` | string | — | — | Result or blocker of the tried actions, max 500 characters |
| `--tool-name` | string | — | — | Tool/command name used |
| `--error-code` | string | — | — | Error code returned by the tool |

**Examples:**

```bash
# Feedback: no matching tool found
tmeet tshoot feedback \
  --category "tool_not_found" \
  --intent "Want to batch export smart minutes within a time range" \
  --actions-tried "Checked record and meeting subcommands" \
  --result "No batch export command found"

# Feedback: tool returned an error
tmeet tshoot feedback \
  --category "tool_error" \
  --intent "Get recording download URL" \
  --tool-name "record address" \
  --error-code "200003" \
  --result "API returned permission denied"

# Feedback: general suggestion
tmeet tshoot feedback \
  --category "suggestion" \
  --intent "Support fuzzy search for meetings by subject"
```

> This command requires login.

---

## Security & Risk Notice (Please Read Before Use)

---
**After the Tencent Meeting CLI tool is connected to AI Agents such as OpenClaw and granted your authorization, the AI will gain access to your Tencent Meeting data (including but not limited to your detailed user information, meeting management and queries, recordings, smart minutes, and other file exports), and will perform operations on your behalf within the authorized scope. Although the tool has security protections in place, the AI may still cause data leakage, unauthorized operations, or other unintended consequences due to model hallucinations, prompt injection, poisoning attacks, uncontrollable execution deviations, and other risks. Please use this tool with caution and comply with your organization's internal data security policies to avoid data loss or leakage. If you suspect a breach or need to disable access, immediately run `tmeet auth logout`.**

**By installing and using this CLI, you acknowledge that you have fully understood and accepted the above risks and voluntarily assume the associated responsibilities.**

---

## Configuration

Configuration files are stored in `~/.tmeet/` by default. You can override settings via environment variables:

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `TMEET_CLI_CONFIG_DIR` | Configuration file directory | `~/.tmeet/` |
| `TMEET_CLI_DATA_DIR` | Encrypted data directory | Platform-specific default path |

> **Note**: All time parameters use **ISO 8601** format, e.g. `2026-04-10T14:00+08:00`. Timestamp fields in responses are automatically converted to ISO 8601 format for display.

## Contributing

Issues and Pull Requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## Security

If you discover a security vulnerability, please refer to [SECURITY.md](SECURITY.md) for instructions on how to report it privately.

## License

This project is open-sourced under the [MIT License](LICENSE).
