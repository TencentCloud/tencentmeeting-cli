# Command Reference

[Back to README](../README_EN.md) | [中文](./command.md)

---

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
| `--until-date` | string | — | — | Recurrence end date (when `--meeting-type=1`), ISO 8601, e.g. `2026-03-12T15:00+08:00` |
| `--invitees` | strings | — | — | Invited participants' openid list, comma-separated or repeat the flag (max 100, e.g. `--invitees open_id1,open_id2`) |
| `--water-mark-type` | int | — | `2` | Text watermark: `0`-single row, `1`-double row, `2`-off<br>● Personal account: default is 2<br>● Enterprise/Organization account:<br>  ✧ Enterprise forced setting - uses enterprise setting as forced state, input parameter does not take effect<br>  ✧ Enterprise not forced setting - uses enterprise setting as default value, input parameter overrides default value |
| `--audio-watermark` | bool | — | `false` | Audio watermark: `true`-on, `false`-off<br>● Personal account: default is false<br>● Enterprise/Organization account:<br>  ✧ Enterprise forced setting - uses enterprise setting as forced state, input parameter does not take effect<br>  ✧ Enterprise not forced setting - uses enterprise setting as default value, input parameter overrides default value |
| `--auto-record-type` | string | — | `none` | Auto record when host joins: `none`-off, `local`-local recording, `cloud`-cloud recording<br>● Personal account: default is none<br>● Enterprise/Organization account:<br>  ✧ Enterprise forced setting - uses enterprise setting as forced state, input parameter does not take effect<br>  ✧ Enterprise not forced setting - uses enterprise setting as default value, input parameter overrides default value |
| `--auto-asr` | bool | — | `false` | Auto speech recognition: `true`-on, `false`-off<br>● Personal account: default is false<br>● Enterprise/Organization account:<br>  ✧ Enterprise forced setting - uses enterprise setting as forced state, input parameter does not take effect<br>  ✧ Enterprise not forced setting - uses enterprise setting as default value, input parameter overrides default value |

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

# Create a meeting and invite participants
tmeet meeting create \
  --subject "Requirements Review" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00" \
  --invitees "open_id1,open_id2,open_id3"

# Create a meeting and explicitly turn off audio watermark / auto speech recognition
# Note: bool flags must use the `=` form when passing `false` (e.g. `--audio-watermark=false`)
tmeet meeting create \
  --subject "No Watermark Meeting" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00" \
  --audio-watermark=false \
  --auto-asr=false
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
| `--sub-meeting-id` | string | — | — | Sub-meeting ID (when `--meeting-type=1`): update only that sub-meeting's time. **Cannot be combined with `--recurring-type` / `--until-type` / `--until-count` / `--until-date`.** If omitted, the whole recurring meeting is updated |
| `--invitees` | strings | — | — | Openid list to mutate; comma-separated or repeat the flag; used together with `--invitees-type`           |
| `--invitees-type` | string | — | — | Invitees mutation strategy: `replace` / `add` / `remove`; required when `--invitees` is set              |

**Example:**

```bash
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --subject "New Title" \
  --start "2026-04-10T15:00+08:00" \
  --end "2026-04-10T16:00+08:00"

# Replace the full invitee list
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2,open_id3" \
  --invitees-type replace

# Add invitees
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id4,open_id5" \
  --invitees-type add

# Remove invitees
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1" \
  --invitees-type remove

# Update only a single sub-meeting's time in a recurring meeting (recurring rule is not modified)
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --meeting-type 1 \
  --sub-meeting-id "100001" \
  --start "2026-04-17T10:00+08:00" \
  --end "2026-04-17T11:00+08:00"

# Explicitly turn off audio watermark / auto speech recognition
# Note: bool flags must use the `=` form when passing `false` (e.g. `--audio-watermark=false`)
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --audio-watermark=false \
  --auto-asr=false
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

#### `meeting search` — Search Meetings

Search meetings by keyword, meeting code, time range, or other filters. All filter parameters are optional and can be combined freely.

```bash
tmeet meeting search [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--query` | string | — | — | Search keyword |
| `--query-field` | string | — | `all` | Search field for `--query`: `subject`-meeting subject; `creator`-creator's nickname/remark name; `note`-user's note on the meeting; `all`-search all fields |
| `--meeting-code` | string | — | — | Filter by meeting code, exact match (digits only, no dashes) |
| `--start` | string | — | — | Lower bound of search time window (ISO 8601, e.g. `2026-03-12T15:00+08:00`). Matches if meeting's scheduled start time, actual start time, or user's join time falls within the window |
| `--end` | string | — | — | Upper bound of search time window (ISO 8601, e.g. `2026-03-12T15:00+08:00`); same semantics as above |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |

**Examples:**

```bash
# Search by subject keyword
tmeet meeting search --query "Weekly Standup" --query-field subject

# Search by creator nickname
tmeet meeting search --query "John" --query-field creator

# Exact search by meeting code
tmeet meeting search --meeting-code "931945029"

# Search by time range
tmeet meeting search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# Fetch the next page
tmeet meeting search \
  --query "Project Review" \
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

#### `meeting invitees-add` — Add Meeting Invitees

Add invitees to an existing meeting. Invitees are specified by user `open_id`, which can be obtained via the `contact search` command.

```bash
tmeet meeting invitees-add --meeting-id <meeting-id> --invitees <open-id-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--invitees` | strings | ✅ | — | List of invitee `open_id`s to add. Supports comma-separated values or repeating the flag, max 100 |

**Examples:**

```bash
# Pass multiple open_ids separated by commas
tmeet meeting invitees-add \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2"

# Repeat the --invitees flag
tmeet meeting invitees-add \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1" \
  --invitees "open_id2"
```

---

#### `meeting invitees-remove` — Remove Meeting Invitees

Remove specified invitees from an existing meeting.

```bash
tmeet meeting invitees-remove --meeting-id <meeting-id> --invitees <open-id-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--invitees` | strings | ✅ | — | List of invitee `open_id`s to remove. Supports comma-separated values or repeating the flag, max 100 |

**Example:**

```bash
tmeet meeting invitees-remove \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2"
```

---

#### `meeting invitees-replace` — Replace Meeting Invitees List

Replace the meeting's current invitee list with a new list (invitees not present in `--invitees` will be removed).

```bash
tmeet meeting invitees-replace --meeting-id <meeting-id> --invitees <open-id-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--invitees` | strings | ✅ | — | New invitee `open_id` list to replace the existing one. Supports comma-separated values or repeating the flag, max 100 |

**Example:**

```bash
tmeet meeting invitees-replace \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2,open_id3"
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

#### `record search` — Search Recordings

Search recordings by keyword, meeting code, meeting ID, time range, file type, or other filters. All filter parameters are optional and can be combined freely.

```bash
tmeet record search [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--query` | string | — | — | Search keyword |
| `--query-field` | string | — | `all` | Search field for `--query`: `subject`-recording subject; `creator`-meeting creator's nickname/remark name; `transcript_content`-original transcript content within the file; `smart_minutes`-smart minutes content within the file (summary + todos); `timeline`-timeline content within the file; `all`-search all fields |
| `--file-type` | string | — | `all` | File type: `video`, `audio`, `transcript`, `upload`, `external`, `all` |
| `--meeting-id` | string | — | — | Filter by meeting ID |
| `--meeting-code` | string | — | — | Filter by meeting code, exact match (digits only, no dashes) |
| `--start` | string | — | — | Query start time (ISO 8601, e.g. `2026-03-12T14:00+08:00`) |
| `--end` | string | — | — | Query end time (ISO 8601, e.g. `2026-03-12T14:00+08:00`) |
| `--page-token` | string | — | — | Pagination cursor; take `next_page_token` from the previous response; omit on first request |
| `--page-size` | int | — | `30` | Page size, default 30, max 30 |

**Examples:**

```bash
# Search transcript content by keyword
tmeet record search --query "quarterly goals" --query-field transcript_content

# Search smart minutes content by keyword
tmeet record search --query "todo" --query-field smart_minutes

# Filter by meeting ID
tmeet record search --meeting-id "6953553464429888300"

# Search by time range with file type filter
tmeet record search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --file-type video

# Fetch the next page
tmeet record search \
  --query "Project Review" \
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

#### `record permission-apply-prepare` — Preview Record Permission Application

Call this command before applying for record permission to fetch the approval text / meeting subject / record owner info. **Show the preview to the user for confirmation**, then call `record permission-apply-commit` to actually submit the application.

```bash
tmeet record permission-apply-prepare --meeting-record-id <record-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-record-id` | string | ✅ | — | Meeting record ID |
| `--meeting-id` | string | — | — | Meeting ID |

**Example:**

```bash
tmeet record permission-apply-prepare --meeting-record-id "record_abc123"
```

Key fields in response `data`:

| Field | Description |
|-------|-------------|
| `preview.meeting_record_id` | Meeting record ID |
| `preview.approval_name` | Approval type text |
| `preview.subject` | Meeting subject |
| `preview.file_owner` | Record owner name |
| `preview.apply_note` | Permission application note |
| `preview.applicant` | Applicant name |
| `expires_in` | Expiration time in seconds |

---

#### `record permission-apply-commit` — Commit Record Permission Application

**Write operation**: Call this command after `permission-apply-prepare` returns a preview and the user has confirmed the application. This formally kicks off the permission approval workflow.

```bash
tmeet record permission-apply-commit --meeting-record-id <record-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-record-id` | string | ✅ | — | Meeting record ID |
| `--meeting-id` | string | — | — | Meeting ID |

**Example:**

```bash
tmeet record permission-apply-commit --meeting-record-id "record_abc123"
```

Key fields in response `data`:

| Field | Description |
|-------|-------------|
| `unique_id` | Application ID |
| `status` | Approval status |
| `message` | Approval status description |
| `approval_url` | Approval URL |
| `share_text` | Application description text |

---

### contact — Contacts

#### `contact search` — Search Enterprise Contact Members

Search enterprise contact members by username, with optional filtering by job title or department to refine results.

```bash
tmeet contact search --username <username> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--username` | string | ✅ | — | Username to search |
| `--job-title` | string | — | — | Job title used to filter results when the username search returns too many matches |
| `--department-name` | string | — | — | Department name used to filter results when the username search returns too many matches |

**Examples:**

```bash
# Search by username
tmeet contact search --username "John"

# Username + job title filter
tmeet contact search --username "John" --job-title "Engineer"

# Username + department filter
tmeet contact search --username "John" --department-name "R&D"
```

---

#### `contact lookup-by-email` — Look Up User Information by Email Address

Look up user details by email address, supporting batch queries for multiple emails.

```bash
tmeet contact lookup-by-email --emails <email-address-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--emails` | []string | ✅ | — | Email address list, multiple emails can be comma-separated or the flag can be repeated, max 50<br>Example: --emails user1@example.com,user2@example.com or --emails user1@example.com --emails user2@example.com |

**Examples:**

```bash
# Look up a single email address
tmeet contact lookup-by-email --emails "user@example.com"

# Batch look up multiple email addresses
tmeet contact lookup-by-email --emails "user1@example.com,user2@example.com,user3@example.com"
```

---

#### `contact lookup-by-phone` — Look Up User Information by Phone Number

Look up user details by phone number, supporting batch queries for multiple phone numbers.

```bash
tmeet contact lookup-by-phone --phones <phone-number-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--phones` | []string | ✅ | — | Phone number list, multiple phone numbers can be comma-separated or the flag can be repeated, max 50<br>Example: --phones 13800138000,13900139000 or --phones 13800138000 --phones 13900139000 |

**Examples:**

```bash
# Look up a single phone number
tmeet contact lookup-by-phone --phones "13800138000"

# Batch look up multiple phone numbers
tmeet contact lookup-by-phone --phones "13800138000,13900139000,13700137000"
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

#### `report participants-export` — Export Participant Details

Asynchronously export meeting participant details. This command only submits the export job and returns a `job_id`. Use `report job-result` to poll the job status and obtain the download link.

```bash
tmeet report participants-export --meeting-id <meeting-id> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--sub-meeting-id` | string | — | — | Sub-meeting ID for recurring meetings |
| `--start` | string | — | — | Query start time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--end` | string | — | — | Query end time, ISO 8601, e.g. `2026-03-12T14:00+08:00` |
| `--file-type` | string | — | `xlsx` | Export file format: `xlsx` or `json` |

**Key response fields:**

| Field | Description |
|-------|-------------|
| `job_id` | Async job ID (used to poll job status) |

> This command only returns a `job_id` and does not wait for the job to complete. After obtaining the `job_id`, poll `report job-result` every 5 seconds until the status is "success" to get the download link, or until the status is no longer "processing" to terminate.

**Examples:**

```bash
# Export participant details (default xlsx format)
tmeet report participants-export --meeting-id "6953553464429888300"

# Export as json format
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --file-type "json"

# Export participants of a specific sub-meeting in a recurring meeting
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "200000001"

# Filter by time range
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00"
```

---

#### `report job-result` — Get Async Job Result

Query the execution status and result of an async export job. After obtaining the `job_id` from `participants-export`, poll this command every 5 seconds until the job completes or fails.

```bash
tmeet report job-result --job-id <job-id>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--job-id` | string | ✅ | — | Job ID (obtained from `participants-export`) |

**Key response fields:**

| Field | Description |
|-------|-------------|
| `status` | Job status: "success", "failed", "processing" |
| `url` | File download link (returned when status is "success", valid for 2 hours) |
| `error_msg` | Error message (returned when status is "failed") |

**Example:**

```bash
# Query async job result
tmeet report job-result --job-id "e1234567-f123-4d12-123a-12346192e332"
```

**Complete workflow for exporting participant details:**

```
1. Submit the export job and obtain job_id
   tmeet report participants-export --meeting-id "6953553464429888300"

2. Poll job-result every 5 seconds
   tmeet report job-result --job-id <job_id>

3. Determine next steps based on the returned status:
   - status = "success": download link (url) is returned (valid for 2 hours), workflow ends
   - status = "processing": wait 5 seconds and poll job-result again
   - status = "failed" or other: terminate and return error_msg
```

---

### control — In-Meeting Control

In-meeting control commands for managing participants during an ongoing meeting, including calling members in and kicking members out. Members are specified by user `open_id`, which can be obtained via the `contact search` command.

#### `control call` — Call Members into the Meeting

In-meeting invite call: send a join-meeting call to the specified members.

```bash
tmeet control call --meeting-id <meeting-id> --users <open-id-list>
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--users` | strings | ✅ | — | List of `open_id`s of members to call. Supports comma-separated values or repeating the flag, max 20 |

**Examples:**

```bash
# Pass multiple open_ids separated by commas
tmeet control call \
  --meeting-id "6953553464429888300" \
  --users "open_id1,open_id2"

# Repeat the --users flag
tmeet control call \
  --meeting-id "6953553464429888300" \
  --users "open_id1" \
  --users "open_id2"
```

---

#### `control waiting-room` — Waiting Room Management

Manage waiting room members during a meeting. Supports three operation types:

- **enter-meeting**: Host admits waiting room members into the meeting
- **back-to-waiting**: Host moves in-meeting members back to the waiting room
- **expel**: Host expels waiting room members from the meeting

```bash
tmeet control waiting-room --meeting-id <meeting-id> --operate-type <type> [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--operate-type` | string | ✅ | — | Operation type: `enter-meeting` (host admits waiting room members into the meeting), `back-to-waiting` (host moves in-meeting members back to the waiting room), `expel` (host expels waiting room members from the meeting) |
| `--users` | strings | one of three | — | List of regular member `open_id`s to operate (excluding Sip/Pstn devices). Supports comma-separated values or repeating the flag |
| `--sip-users` | strings | one of three | — | List of Sip device `ms_open_id`s to operate. Supports comma-separated values or repeating the flag |
| `--pstn-users` | strings | one of three | — | List of Pstn device `ms_open_id`s to operate. Supports comma-separated values or repeating the flag |
| `--allow-rejoin` | bool | ❌ | — | Whether expelled members are allowed to rejoin the meeting (only valid when `--operate-type=expel`); |

> At least one of `--users` / `--sip-users` / `--pstn-users` is required, and the **total number of all three combined must not exceed 20**.

**Examples:**

```bash
# Admit waiting room members into the meeting
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type enter-meeting \
  --users "open_id1,open_id2"

# Move in-meeting members back to the waiting room
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type back-to-waiting \
  --users "open_id1,open_id2"

# Expel waiting room members (disallow rejoin)
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --users "open_id1,open_id2"

# Expel waiting room members (allow rejoin)
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --allow-rejoin \
  --users "open_id1,open_id2"

# Expel waiting room members (explicitly disallow rejoin)
# Note: For bool flags, setting false MUST use the equals syntax --allow-rejoin=false; the space form --allow-rejoin false is NOT supported.
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --allow-rejoin=false \
  --users "open_id1,open_id2"

# Operate Sip and Pstn devices simultaneously
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --sip-users "ms_open_id_sip1" \
  --pstn-users "ms_open_id_pstn1"
```

---

#### `control kick` — Kick Members Out of the Meeting

In-meeting kick-out: remove the specified members from the ongoing meeting.

```bash
tmeet control kick --meeting-id <meeting-id> [--users <open-id-list>] [--sip-users <ms-open-id-list>] [--pstn-users <ms-open-id-list>] [--allow-rejoin]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--meeting-id` | string | ✅ | — | Meeting ID |
| `--users` | strings | one of three | — | List of `open_id`s of regular members to kick out (excluding CIP/Pstn devices). Supports comma-separated values or repeating the flag |
| `--sip-users` | strings | one of three | — | List of `ms_open_id`s of Sip devices to kick out. Supports comma-separated values or repeating the flag |
| `--pstn-users` | strings | one of three | — | List of `ms_open_id`s of Pstn devices to kick out. Supports comma-separated values or repeating the flag |
| `--allow-rejoin` | bool | ❌ | `true` | Whether kicked-out members are allowed to rejoin the meeting. Defaults to `true` (rejoin allowed) when not provided; pass `--allow-rejoin=false` to disallow rejoin |

> At least one of `--users` / `--sip-users` / `--pstn-users` is required, and the **total number of all three combined must not exceed 20**.

**Example:**

```bash
# Kick regular members
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --users "open_id1,open_id2"

# Kick regular members, Sip devices, and Pstn devices together (total <= 20)
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --users "open_id1" \
  --sip-users "ms_open_id_sip1" \
  --pstn-users "ms_open_id_pstn1"

# Disallow kicked-out members from rejoining
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --allow-rejoin=false \
  --users "open_id1,open_id2"
```

---

### minutes — Yuanbao Minutes

#### `minutes search` — Search Yuanbao Minutes

Search Yuanbao minutes by keyword and/or time range. All filter parameters are optional and can be combined freely.

```bash
tmeet minutes search [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--query` | string | ❌ | — | Search keyword, max 50 characters |
| `--start` | string | ❌ | — | Lower bound of search time window (ISO 8601, e.g. `2026-03-12T14:00+08:00`) |
| `--end` | string | ❌ | — | Upper bound of search time window (ISO 8601, e.g. `2026-03-12T14:00+08:00`) |
| `--page-token` | string | ❌ | — | Pagination cursor; omit for the first page, pass the previous response's `next_page_token` for subsequent pages |
| `--page-size` | int | ❌ | `20` | Page size, default 20, max 50 |

**Examples:**

```bash
# Search by keyword
tmeet minutes search --query "quarterly goals"

# Search by time range
tmeet minutes search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# Keyword + time range combined search
tmeet minutes search \
  --query "project review" \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# Next page
tmeet minutes search \
  --query "project review" \
  --page-token "<next_page_token>" --page-size 20
```

---

#### `minutes get` — Get Yuanbao Minutes Detail

Query Yuanbao minutes detail by minute ID or meeting ID. One of `--minute-id` or `--meeting-id` is required.

```bash
tmeet minutes get (--minute-id <ID> | --meeting-id <ID>) [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--minute-id` | string | One of two | — | Minute unique identifier |
| `--meeting-id` | string | One of two | — | Meeting ID (cycle-level), requires `--sub-meeting-id` for recurring meeting instance |
| `--sub-meeting-id` | string | ❌ | — | Sub-meeting ID (recurring meeting instance); omit for non-recurring meetings |
| `--overview` | bool | ❌ | `true` | Include meeting overview |
| `--summary-points` | bool | ❌ | `true` | Include summary points |
| `--todos` | bool | ❌ | `true` | Include todos |
| `--short-summary` | bool | ❌ | `false` | Include rolling summary history sequence |
| `--page-token` | string | ❌ | — | Pagination cursor; used when one `meeting_id` returns multiple minutes |
| `--page-size` | int | ❌ | `10` | Page size, default 10, max 30 |

**Examples:**

```bash
# Get by minute ID
tmeet minutes get --minute-id "minute_abc123"

# Get by meeting ID
tmeet minutes get --meeting-id "6953553464429888300"

# Get by meeting ID + sub-meeting ID (recurring meeting)
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "100001"

# Only get overview and todos, skip summary points
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --summary-points=false

# Next page (when one meeting has multiple minutes)
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 10
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

### app — Application Info

Manage the current CLI app integration info (homepage, in-meeting open layout, and SDK name).

#### `app get` — Get App Info

Query the current CLI app integration info.

```bash
tmeet app get
```

> No parameters.

---

#### `app set` — Set App Info

Set the current CLI app integration info. At least one of `--homepage` / `--layout-style` / `--sdk-name` must be provided; only the fields you pass will be updated.

```bash
tmeet app set [options]
```

| Parameter | Type | Required | Default | Description |
|-----------|------|:--------:|---------|-------------|
| `--homepage` | string | one of three | — | App homepage URL; must use `http://` or `https://`; when `https://` is used, the address must have an SSL/TLS certificate **trusted by the local machine**, otherwise opening it in the meeting will be blocked; up to **200 characters**.<br/>⚠️ Tencent Meeting client versions **prior to 3.45.10 do not support opening `http://` pages** — only `https://` is supported; version 3.45.10 and later support both. If the target user's client version cannot be confirmed, prefer `https://` |
| `--layout-style` | string | one of three | server default `sidebar` | In-meeting open layout: `sidebar` (narrow sidebar) \| `wide_sidebar` (wide sidebar) \| `popout` (standalone popout window). **When this flag is omitted, the CLI does not send the field**: if the server has no existing value, it defaults to `sidebar`; if a value is already set, it is left unchanged |
| `--sdk-name` | string | one of three | — | App SDK name; up to **20 in display width** (ASCII counts as 1, non-ASCII such as Chinese counts as 2) |

**Examples:**

```bash
# Set the app homepage URL
tmeet app set --homepage "https://example.com"

# Set the in-meeting open layout to a standalone popout window
tmeet app set --layout-style popout

# Set the SDK name
tmeet app set --sdk-name "my-sdk"

# Set multiple fields at once
tmeet app set --homepage "https://example.com" --layout-style wide_sidebar --sdk-name "my-sdk"
```

---

### event — Real-time Event Subscription

Subscribe to Tencent Meeting real-time events (e.g. `meeting.started`, `meeting.end`) through a per-host background **bus daemon**. All `tmeet event consume` consumers share a single WSS long connection, with handshake / heartbeat / auto-reconnect centrally managed by the bus.

Events are emitted as NDJSON (one JSON object per line) to **stdout**; control-plane diagnostics such as handshake progress, source state changes and drop warnings go to **stderr**. The stream is designed to be piped directly into an Agent or shell script.

**General conventions:**

- **stdout / stderr split**: business events go only to stdout (NDJSON); all diagnostics / state / warnings go only to stderr.
- **Ready marker**: once `event consume` finishes handshake and is ready, it emits a stable readiness line to stderr:

  ```text
  [event] ready event_key=<key>
  ```

  This line is never suppressed, even with `--quiet`. Agents can grep this line to know the subscription is live before triggering follow-up actions.
- **Exit marker**: `event consume` also emits an exit summary to stderr on exit (also not affected by `--quiet`):

  ```text
  [event] exited — received <N> event(s) in <duration> (reason: <reason>)
  ```

  `reason` is one of: `limit` / `timeout` / `signal` / `shutdown`.
- **Exit codes**:
  - `0` — normal exit (reached `--max-events` / `--timeout` / received SIGINT/SIGTERM / bus initiated shutdown).
  - `1` — fatal error (Hello rejected, unknown EventKey, IO error, subscribe failure, etc.).
  - `2` — only `event status --fail-on-orphan` and `event stop` use this when in `refused` / `errored` state, so health-check scripts can branch on it.

> `event _bus` is a hidden subcommand (auto-forked by `event consume`); manual invocation is not recommended.

---

#### `event list` — List available EventKeys

Lists every EventKey baked into the current CLI build, as a JSON array sorted by `(domain, key)`.

> This command reads the local built-in registry; it **does not require login** and makes no remote calls.

```bash
tmeet event list [options]
```

| Flag | Type | Required | Default | Description |
|------|------|:--------:|---------|-------------|
| `--domain` | string | — | — | Show only EventKeys under the given domain (e.g. `meeting`, `record`); an unknown domain exits with code 1 and prints the list of known domains |

**Output fields:**

| Field | Description |
|-------|-------------|
| `key` | EventKey name, e.g. `meeting.started` |
| `domain` | Owning domain, e.g. `meeting` |
| `description` | Short description |

**Examples:**

```bash
# List all EventKeys
tmeet event list

# List only EventKeys in the meeting domain, with pretty-printed output
tmeet event list --domain meeting --format json-pretty
```

---

#### `event schema` — Show the full contract of an EventKey

Prints the params schema (the keys accepted by `--param`), the JSON Schema of the event payload, and the root path that `--jq` expressions are evaluated against.

> Also a local registry lookup; **no login required**.

```bash
tmeet event schema <EventKey>
```

| Flag | Type | Required | Description |
|------|------|:--------:|-------------|
| `<EventKey>` | string | ✅ | Positional argument — the EventKey to inspect; unknown keys exit with code 1 and hint to use `event list` |

**Output fields:**

| Field | Description |
|-------|-------------|
| `key` | EventKey name |
| `domain` | Owning domain |
| `jq_root_path` | Root path for `--jq` expressions; either `.` (full envelope) or `.payload` (payload only) |
| `params_schema` | Map describing the keys accepted by `--param key=value` |
| `resolved_output_schema` | JSON Schema of the event payload |

**Example:**

```bash
tmeet event schema meeting.started --format json-pretty
```

---

#### `event consume` — Subscribe to events and stream NDJSON

Subscribes to the event stream of the given EventKey and writes one NDJSON line per event to stdout. If the underlying bus daemon is not running, one is auto-forked.

Two run modes:

- **Batch**: pass `--max-events` and/or `--timeout`; exits with code 0 as soon as the first condition is met.
- **Long-running**: pass neither, and the process runs until it receives SIGINT/SIGTERM or until `tmeet event stop` shuts the bus down.

```bash
tmeet event consume --event-id <EventKey> [options]
```

> `--event-id` accepts a single EventKey; positional arguments are rejected with exit code 1.

| Flag | Type | Required | Default | Description |
|------|------|:--------:|---------|-------------|
| `--event-id` | string | ✅ | — | The EventKey to subscribe to (must be registered, see `event list`); accepts a single EventKey |
| `--param` | strings | — | — | Subscription filter parameters in `key=value` form; repeatable; the set of accepted keys is given by `event schema <key>.params_schema` |
| `--max-events` | int | — | `0` | Exit after receiving N events cumulatively; `0` means no limit |
| `--timeout` | duration | — | `0` | Exit N after the ready marker; `0` means no limit (e.g. `30s`, `5m`) |
| `--quiet` | bool | — | `false` | Suppress informational stderr lines; ready / exit / WARN / errors are still emitted |
| `--output-dir` | string | — | — | Additionally write each event to `<output-dir>/<trace_id>.json`; only relative paths are allowed, no `..` segments; the directory is created if missing |
| `--jq` | string | — | — | Run a gojq expression on each event: a result of null / no result drops the event; otherwise the result replaces the default NDJSON line |

**stdout format (default):**

```json
{"event":"meeting.started","trace_id":"<id>","payload":{...}}
```

**stderr control-plane diagnostics** (informational lines can be silenced by `--quiet`; ready / exit / WARN / handshake failures are always emitted):

```text
[event] starting consume key=<key>
[event] bus not running, forked daemon                      # only when bus was auto-forked
[event] handshake ok bus_version=<version>
[event] ready event_key=<key>                               # readiness marker, not silenced by --quiet
[event] received trace_id=<id>                              # one line per event
[source] <source>: <state> (<detail>)                       # upstream source state change
[event] WARN dropped <N> event(s) for key=<key> since unix=<ts>
[event] WARN subscribe failed key=<key> code=<code> (<detail>)
[event] exited — received <N> event(s) in <duration> (reason: <reason>)
```

**Examples:**

```bash
# Long-running subscription (Ctrl-C to exit)
tmeet event consume --event-id meeting.started

# Exit after 3 events
tmeet event consume --event-id meeting.started --max-events 3

# Exit after 30 seconds with no events
tmeet event consume --event-id meeting.end --timeout 30s

# Narrow the subscription with --param (see `event schema` for accepted keys)
tmeet event consume --event-id meeting.started --param meeting_id=6953553464429888300

# Project only meeting_id and subject with jq.
# Note: jq_root_path for meeting.started / meeting.end is .payload,
# i.e. the jq input root . IS the payload array itself (the server contract
# guarantees length 1), so use .[0] to take the first element before drilling in.
tmeet event consume --event-id meeting.started \
  --jq '.[0].meeting_info | {meeting_id, subject}'

# Audit every event to disk while silencing informational stderr
tmeet event consume --event-id meeting.started \
  --output-dir ./meeting_events \
  --quiet
```

> ⚠️ `event consume` requires being logged in (the OpenID is used to compute the owner_hash bound to the bus). Run `tmeet auth login` first if needed.

---

#### `event status` — Show the local bus daemon status

Reports the state of the local bus daemon. The output schema always carries a `buses` array of length 0 or 1 (tmeet allows at most one bus instance per host).

> This command only reads the local bus directory and IPC; **no login required**. It is useful for diagnosing residual state after `auth logout`.

```bash
tmeet event status [options]
```

| Flag | Type | Required | Default | Description |
|------|------|:--------:|---------|-------------|
| `--fail-on-orphan` | bool | — | `false` | Exit with code `2` (instead of `0`) when an `orphan` or `stale_owner` bus is detected, so health-check scripts can branch on it |

**`buses[].state` values:**

| State | Meaning | Recommended action |
|-------|---------|--------------------|
| `running` | Bus is alive and bound to the current logged-in user | No action needed |
| `stale_owner` | Bus is alive but bound to a different user, or no user is logged in on this host | After confirming with the original owner, run `tmeet event stop --force`, or log in again as the original account |
| `orphan` | Bus has exited but leftover files such as `bus.pid` / `bus.meta` remain on disk | Run `tmeet event stop --force` to clean up the residue |

**Key fields in `buses[]`:**

| Field | Description |
|-------|-------------|
| `state` | State enum (see table above) |
| `openid_hash` | Hash of the OpenID the bus is bound to |
| `is_active_login` | Whether the bus owner equals the currently logged-in user |
| `pid` | Bus process PID |
| `started_at` | Bus start time (local-timezone RFC3339) |
| `sock` | Unix socket path the bus listens on |
| `consumer_count` | Number of attached consumers (meaningful only when `running`) |
| `subscribed_keys` | EventKeys currently subscribed |
| `wss.state` | Underlying WSS link state (`connecting` / `steady` / `reconnecting` / `auth_failed` / `auth_expired` / `disconnected`) |
| `wss.connected_at` | WSS connect time (local-timezone RFC3339) |
| `wss.reconnect_count` | Cumulative number of WSS reconnects |
| `hint` | Suggested remediation when state is abnormal |

**Examples:**

```bash
# Plain query
tmeet event status --format json-pretty

# Health-check script: exit code 2 on orphan / stale_owner
tmeet event status --fail-on-orphan
```

---

#### `event stop` — Stop the local bus daemon

Asks the bus daemon to exit. The default path is a graceful shutdown; use `--force` for forced cleanup when needed.

> Same as `event status`, **no login required**; commonly used to clean up residual state after `auth logout`.

```bash
tmeet event stop [options]
```

| Flag | Type | Required | Default | Description |
|------|------|:--------:|---------|-------------|
| `--force` | bool | — | `false` | Skip the "active consumers" refusal guard; force-clean `orphan` / `stale_owner` states; also wipe residual `bus.pid` / `bus.meta` / `bus.sock` on disk |
| `--timeout` | duration | — | `10s` | Maximum time to wait for a graceful exit; on timeout, `--force` (if given) switches to cleanup mode |

**`results[].state` values and exit codes:**

| State | Meaning | Exit code |
|-------|---------|:---------:|
| `stopped` | Bus has exited (covers both graceful and forced cleanup) | `0` |
| `no_bus` | No bus exists on disk or at runtime; effectively a no-op | `0` |
| `refused` | Active consumers attached / `stale_owner` / `orphan` detected without `--force` | `2` |
| `errored` | Graceful shutdown timed out without `--force`, or forced cleanup failed | `2` |

**Key fields in `results[]`:**

| Field | Description |
|-------|-------------|
| `state` | See table above |
| `openid_hash` | Owner hash of the bus that was acted on |
| `pid` | Bus process PID |
| `consumers_evicted` | Number of consumers evicted on exit (meaningful when `stopped`) |
| `consumer_count` | Number of active consumers at refusal time (meaningful when `refused`) |
| `forced` | Whether the `--force` path was taken |
| `socket_cleaned` | Whether `bus.sock` was cleaned up |
| `elapsed_ms` | Graceful-shutdown elapsed time (milliseconds) |
| `hint` | Suggested next step |

**Examples:**

```bash
# Graceful shutdown: refused (exit code 2) when active consumers exist
tmeet event stop

# Force shutdown: evict active consumers; or clean up orphan / stale_owner
tmeet event stop --force

# Custom graceful wait
tmeet event stop --timeout 5s
```
