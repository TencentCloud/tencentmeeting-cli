# tmeet record — 录制管理

> **前置条件：** 先执行 `tmeet auth login` 完成登录授权。

时间参数格式：`2026-03-12T14:00:00+08:00` 或 `2026-03-12T14:00+08:00`（必须包含时区）。

---

## list — 查询录制列表

```bash
# 按会议 ID 查询
tmeet record list --meeting-id "100000000"

# 按会议码查询
tmeet record list --meeting-code "123456789"

# 按时间范围查询
tmeet record list \
  --start "2026-04-01T00:00:00+08:00" \
  --end "2026-04-30T23:59:59+08:00"

# 组合使用：会议 ID + 时间范围（进一步缩小结果范围）
tmeet record list \
  --meeting-id "100000000" \
  --start "2026-04-01T00:00:00+08:00" \
  --end "2026-04-30T23:59:59+08:00"

# 分页查询
tmeet record list \
  --meeting-id "100000000" \
  --page 2 \
  --page-size 20
```

### 参数

| 参数 | 必填 | 默认值 | 说明                                             |
|------|------|--------|------------------------------------------------|
| `--meeting-id <id>` | 至少一组 | — | 会议 ID                                          |
| `--meeting-code <code>` | 至少一组 | — | 会议码                                            |
| `--start <time>` + `--end <time>` | 至少一组 | — | 时间范围（ISO 8601，含时区，建议 `--start` 与 `--end` 同时提供） |
| `--page <n>` | 否 | `1` | 页码，从 `1` 开始                                    |
| `--page-size <n>` | 否 | `10` | 每页数量，最大20                                      |

> `--meeting-id`、`--meeting-code`、`--start + --end` 三组**至少提供一组**，多组可叠加使用以缩小查询范围。

---

## address — 获取录制文件下载地址

```bash
# 获取录制文件下载地址
tmeet record address --meeting-record-id "record_abc123"

# 分页获取
tmeet record address \
  --meeting-record-id "record_abc123" \
  --page 1 \
  --page-size 50
```

### 参数

| 参数 | 必填 | 默认值 | 说明                             |
|------|------|--------|--------------------------------|
| `--meeting-record-id <id>` | ✅ | — | 会议录制 ID（从 `record list` 结果中获取） |
| `--page <n>` | 否 | `1` | 页码，从 `1` 开始                    |
| `--page-size <n>` | 否 | `50` | 每页数量，最大50                      |

---

## smart-minutes — 获取智能纪要

```bash
# 获取录制文件的智能纪要（默认原文）
tmeet record smart-minutes --record-file-id "file_abc123"

# 获取中文翻译版纪要
tmeet record smart-minutes \
  --record-file-id "file_abc123" \
  --lang zh

# 带访问密码的录制文件
tmeet record smart-minutes \
  --record-file-id "file_abc123" \
  --pwd "123456"
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--record-file-id <id>` | ✅ | — | 录制文件 ID（从 `record address` 结果中获取） |
| `--lang <lang>` | 否 | `default` | 语言：`default`-原文，`zh`-简体中文，`en`-英文，`ja`-日语 |
| `--pwd <pwd>` | 否 | — | 录制文件访问密码 |

---

## transcript-get — 获取转写详情

```bash
# 获取转写详情
tmeet record transcript-get --record-file-id "file_abc123"

# 指定会议 ID 和分页参数
tmeet record transcript-get \
  --record-file-id "file_abc123" \
  --meeting-id "100000000" \
  --pid "paragraph_001" \
  --limit "50"
```

### 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--record-file-id <id>` | ✅ | 录制文件 ID |
| `--meeting-id <id>` | 否 | 会议 ID |
| `--pid <id>` | 否 | 查询起始段落 ID（用于分页） |
| `--limit <n>` | 否 | 查询段落数量 |

---

## transcript-paragraphs — 获取转写段落列表

```bash
# 获取转写段落列表
tmeet record transcript-paragraphs --record-file-id "file_abc123"

# 指定会议 ID
tmeet record transcript-paragraphs \
  --record-file-id "file_abc123" \
  --meeting-id "100000000"
```

### 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--record-file-id <id>` | ✅ | 录制文件 ID |
| `--meeting-id <id>` | 否 | 会议 ID |

---

## transcript-search — 搜索转写内容

```bash
# 在转写内容中搜索关键词
tmeet record transcript-search \
  --record-file-id "file_abc123" \
  --text "季度目标"

# 指定会议 ID 搜索
tmeet record transcript-search \
  --record-file-id "file_abc123" \
  --meeting-id "100000000" \
  --text "行动项"
```

### 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--record-file-id <id>` | ✅ | 录制文件 ID |
| `--text <keyword>` | ✅ | 搜索关键词 |
| `--meeting-id <id>` | 否 | 会议 ID |

---

## 典型工作流

```
1. 查询录制列表，获取 meeting_record_id
   tmeet record list --meeting-id "..."

2. 获取录制文件下载地址，获取 record_file_id
   tmeet record address --meeting-record-id <meeting_record_id>

3. 获取智能纪要 / 转写内容
   tmeet record smart-minutes --record-file-id <record_file_id>
   tmeet record transcript-paragraphs --record-file-id <record_file_id>
   tmeet record transcript-search --record-file-id <record_file_id> --text "关键词"
```

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `one of the following groups is required` | 缺少必填参数组 | 提供 `--meeting-id`、`--meeting-code` 或 `--start + --end` 其中一组 |
| `--start format error` | 时间格式不合法（如缺少时区） | 改用 `2026-03-12T14:00:00+08:00` 格式 |
| `--record-file-id is required` | 缺少必填参数 | 先通过 `record list` + `record address` 获取 |
| `--text is required` | 搜索缺少关键词 | 补充 `--text` |

## 参考

- [tmeet](../SKILL.md) — 全部命令概览
- [tmeet-meeting](tmeet-meeting.md) — 会议管理
- [tmeet-report](tmeet-report.md) — 会议报告
