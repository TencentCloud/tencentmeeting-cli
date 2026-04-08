# tmeet

[中文](README.md) | English

A command-line interface (CLI) tool for Tencent Meeting, based on Tencent Meeting Open Platform OAuth2 authorization. Supports meeting management, recording management, attendance reports, and more.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)

## Features

- 🔐 **OAuth2 Authorization** — Device code authorization flow, secure and passwordless
- 📅 **Meeting Management** — Create, query, update, and cancel meetings; supports recurring meetings and invitee management
- 🎬 **Recording Management** — Query recording lists, get download URLs, smart minutes, transcript details and search
- 📊 **Attendance Reports** — Query participant lists and waiting room member records
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

This outputs an authorization URL. Open it in your browser to complete the QR code authorization. The CLI automatically polls for the result (5-minute timeout) and saves the credentials encrypted locally.

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
| `--version` | `-V` | — | Show version number |

**Examples:**

```bash
# Show version number
tmeet -V

# Output response in indented format
tmeet meeting get --meeting-id "6953553464429888300" --format json-pretty
```

---

## Command Overview

```
tmeet [--format json] [-V]
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
└── report
    ├── participants   # Get participant list
    └── waiting-room-log # Get waiting room member list
```

---

## Command Reference

### auth — Authorization Management

#### `auth login`

Login and complete OAuth2 authorization, saving credentials encrypted locally.

```bash
tmeet auth login
```

> No parameters. Follow the prompt to complete QR code authorization in your browser.

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

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--subject` | string | ✅ | — | Meeting subject/title |
| `--start` | string | ✅ | — | Meeting start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | ✅ | — | Meeting end time, ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--password` | string | — | — | Meeting password (4–6 digits) |
| `--timezone` | string | — | — | Timezone, refer to Oracle-TimeZone standard, e.g. `Asia/Shanghai` |
| `--meeting-type` | int | — | `0` | Meeting type: `0`-regular meeting, `1`-recurring meeting |
| `--join-type` | int | — | `0` | Join restriction: `1`-all members, `2`-invited members only, `3`-internal members only |
| `--waiting-room` | bool | — | `false` | Enable waiting room: `true`-enable, `false`-disable |
| `--recurring-type` | int | — | `0` | Recurrence type (when `--meeting-type=1`): `0`-daily, `1`-weekdays, `2`-weekly, `3`-biweekly, `4`-monthly, `5`-custom |
| `--until-type` | int | — | `0` | Recurrence end type (when `--meeting-type=1`): `0`-end by date, `1`-end by count |
| `--until-count` | int | — | `7` | Max occurrences (when `--meeting-type=1`): max 500 for daily/weekday/weekly; max 500 for biweekly/monthly |
| `--until-date` | string | — | — | Recurrence end date (when `--meeting-type=1`), ISO 8601, e.g. `2026-03-12T15:00+08:00` |

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

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--subject` | string | — | — | Meeting subject/title |
| `--start` | string | — | — | Meeting start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | — | — | Meeting end time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--password` | string | — | — | Meeting password (4–6 digits) |
| `--timezone` | string | — | — | Timezone, e.g. `Asia/Shanghai` |
| `--meeting-type` | int | — | `0` | Meeting type: `0`-regular meeting, `1`-recurring meeting |
| `--join-type` | int | — | `0` | Join restriction: `1`-all members, `2`-invited members only, `3`-internal members only |
| `--waiting-room` | bool | — | `false` | Enable waiting room |
| `--recurring-type` | int | — | `0` | Recurrence type (when `--meeting-type=1`): `0`-daily, `1`-weekdays, `2`-weekly, `3`-biweekly, `4`-monthly, `5`-custom |
| `--until-type` | int | — | `0` | Recurrence end type (when `--meeting-type=1`): `0`-end by date, `1`-end by count |
| `--until-count` | int | — | `7` | Max occurrences (when `--meeting-type=1`): max 500 for daily/weekday/weekly; max 500 for biweekly/monthly |
| `--until-date` | string | — | — | Recurrence end date (when `--meeting-type=1`), ISO 8601, e.g. `2026-03-12T15:00+08:00` |

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

**Examples:**

```bash
tmeet meeting list
tmeet meeting list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --show-all-sub 1
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
| `--page` | int | — | `1` | Page number, starting from 1 |
| `--page-size` | int | — | `10` | Page size, default 10, max 20 |

**Examples:**

```bash
# Query ended meetings this month
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# Paginated query
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page 2 --page-size 20
```

---

#### `meeting invitees-list` — List Meeting Invitees

```bash
tmeet meeting invitees-list --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--pos` | int | — | `0` | Starting position for paginated invitee list query |

**Examples:**

```bash
tmeet meeting invitees-list --meeting-id "6953553464429888300"
tmeet meeting invitees-list --meeting-id "6953553464429888300" --pos 20
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
| `--page` | int | — | `1` | Page number, starting from 1 |
| `--page-size` | int | — | `10` | Page size |

**Examples:**

```bash
# Query by time range
tmeet record list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page 1 --page-size 20

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
| `--page` | int | — | `1` | Page number, starting from 1 |
| `--page-size` | int | — | `50` | Page size |

**Example:**

```bash
tmeet record address --meeting-record-id "record_abc123"
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
| `--pid` | string | — | — | Starting paragraph ID for query |
| `--limit` | string | — | — | Number of paragraphs to query |

**Example:**

```bash
tmeet record transcript-get --record-file-id "file_abc123" --pid "para_001" --limit "50"
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

**Example:**

```bash
tmeet record transcript-paragraphs --record-file-id "file_abc123"
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

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--sub-meeting-id` | string | — | — | Sub-meeting ID for recurring meetings |
| `--pos` | int | — | `0` | Starting position for paginated participant list query |
| `--size` | int | — | `20` | Number of participants to fetch, max 100 per page |
| `--start` | string | — | — | Query start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | — | — | Query end time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |

**Examples:**

```bash
tmeet report participants --meeting-id "6953553464429888300" --size 50
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"
```

---

#### `report waiting-room-log` — Get Waiting Room Members

```bash
tmeet report waiting-room-log --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--page` | int | — | `1` | Page number, default 1 |
| `--page-size` | int | — | `20` | Page size, default 20 |

**Example:**

```bash
tmeet report waiting-room-log --meeting-id "6953553464429888300" --page 1 --page-size 50
```

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
