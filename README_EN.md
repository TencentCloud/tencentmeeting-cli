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
- 👥 **Contacts** — Search enterprise contact members by username, job title, or department
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

Starting from `v1.0.5`, all list/query commands that support pagination use a unified **`--page-token` + `--page-size`** model. The legacy `--page` / `--pos` / `--size` flags are marked as **deprecated** — they still work for backward compatibility but are discouraged and may be removed in a future release.

> Note: The `--pid` / `--limit` flags of `record transcript-get` are dedicated paragraph-locating parameters of that specific command, **not** part of the generic pagination scheme, and are **not deprecated**.

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
| `meeting search` |   30    | 30 | — |
| `meeting invitees-list` |   30    | 30 | `--pos` |
| `record list` |   30    | 30 | `--page` |
| `record address` |   30    | 30 | `--page` |
| `record search` |   30    | 30 | — |
| `report participants` |   100   | 100 | `--pos` / `--size` |
| `report waiting-room-log` |   100   | 100 | `--page` |
| `minutes search` |   20    | 50 | — |
| `minutes get` |   10    | 30 | — |

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
│   ├── search         # Search meetings by keyword/code/time range
│   ├── invitees-list    # List meeting invitees
│   ├── invitees-add     # Add meeting invitees
│   ├── invitees-remove  # Remove meeting invitees
│   └── invitees-replace # Replace meeting invitees list
├── contact
│   ├── search         # Search enterprise contact members
│   ├── lookup-by-email # Look up user information by email address
│   └── lookup-by-phone # Look up user information by phone number
├── record
│   ├── list                     # Query recording list
│   ├── address                  # Get recording file download URL
│   ├── search                   # Search recordings by keyword/code/meeting-id/time range
│   ├── smart-minutes            # Get smart minutes
│   ├── transcript-get           # Get transcript details
│   ├── transcript-paragraphs    # Get transcript paragraph list
│   ├── transcript-search        # Search transcript content
│   ├── permission-apply-prepare # Preview record permission application (before commit)
│   └── permission-apply-commit  # Commit record permission application (after user confirmation)
├── report
│   ├── participants         # Get participant list
│   ├── waiting-room-log     # Get waiting room member list
│   ├── participants-export  # Export participant details (async job)
│   └── job-result           # Get async job result
├── control
│   ├── call           # Call members into the meeting (in-meeting invite call)
│   ├── kick           # Kick members out of the meeting (in-meeting kick-out)
│   └── waiting-room   # Waiting room management (admit / send back / expel)
├── minutes
│   ├── search         # Search Yuanbao minutes by keyword/time range
│   └── get            # Get Yuanbao minutes detail
├── tshoot
│   ├── log            # Export local logs (supports time range filter, optional --upload to server)
│   └── feedback       # Report troubleshooting feedback to the server
├── app
│   ├── get            # Get current CLI app info
│   └── set            # Set current CLI app info
└── event
    ├── list           # List available EventKeys
    ├── schema         # Show the params / output schema of an EventKey
    ├── consume        # Subscribe to an EventKey and emit events as NDJSON
    ├── status         # Show the local bus daemon status
    └── stop           # Stop the local bus daemon (with optional --force cleanup)
```

---

## Command Reference

For full parameter tables, examples, and response fields of each subcommand, see: 👉 [docs/command_en.md](docs/command_en.md)

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
