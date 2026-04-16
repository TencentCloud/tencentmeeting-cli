# tmeet

[English](README_EN.md) | 中文

腾讯会议命令行工具（CLI），基于腾讯会议开放平台 OAuth2 授权，支持会议管理、录制管理、参会报告等功能。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)

## 功能特性

- 🔐 **OAuth2 授权登录** — 设备码授权流程，安全无密码
- 📅 **会议管理** — 创建、查询、更新、取消会议，支持周期性会议，管理受邀成员
- 🎬 **录制管理** — 查询录制列表、获取下载地址、智能纪要、转写详情与搜索
- 📊 **参会报告** — 查询参会人列表、等候室成员记录
- 🔒 **安全存储** — 凭证使用 AES-256-GCM 加密，明文不落盘
- 🖥️ **跨平台** — 支持 macOS、Linux、Windows

## 安装

### 第一步：安装 CLI

#### 方式一：通过 npm 安装（推荐）

```bash
npm install -g @tencentcloud/tmeet
```

安装完成后即可直接使用 `tmeet` 命令。

> 💡 如果提示 `npm: command not found`，说明尚未安装 Node.js。请前往 [Node.js 官网](https://nodejs.org/) 下载并安装 LTS 版本（已包含 npm）。

#### 方式二：从源码构建

```bash
git clone https://github.com/TencentCloud/tencentmeeting-cli
cd tencentmeeting-cli
go build -ldflags "-X tmeet/cmd.Version=v1.0.0" -o tmeet .
# 或
make build VERSION=v1.0.0
```

### 第二步：安装 CLI-SKILL

```bash
npx skills add TencentCloud/tencentmeeting-cli -y -g
```

## 快速开始

### 1. 登录授权

```bash
tmeet auth login
```

执行后会自动尝试打开系统默认浏览器跳转到授权 URL；若无默认浏览器，则输出授权 URL，手动在浏览器中打开完成扫码授权。CLI 自动轮询结果（超时 5 分钟），凭证加密保存到本地。

> 如需禁用自动打开浏览器，可使用 `--no-browser` 参数：`tmeet auth login --no-browser`

### 2. 创建会议

```bash
tmeet meeting create \
  --subject "周例会" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"
```

### 3. 查询会议列表

```bash
# 查询进行中/即将开始的会议
tmeet meeting list

# 查询已结束的会议
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"
```

### 4. 登出

```bash
tmeet auth logout
```

---

## 全局标志

所有命令均支持以下全局标志：

| 标志 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--format` | — | `json` | 输出格式：`json`（紧凑格式）\| `json-pretty`（缩进格式） |
| `--version` | `-V` | — | 查看版本号 |

**示例：**

```bash
# 查看版本号
tmeet -V

# 以缩进格式输出响应
tmeet meeting get --meeting-id "6953553464429888300" --format json-pretty
```

---

## 命令总览

```
tmeet [--format json] [-V]
├── auth
│   ├── login          # OAuth 授权登录
│   ├── logout         # 登出并清除凭证
│   └── status         # 查看当前登录状态
├── meeting
│   ├── create         # 创建会议（支持普通/周期性）
│   ├── update         # 更新会议信息
│   ├── cancel         # 取消会议
│   ├── get            # 获取会议详情
│   ├── list           # 获取进行中/即将开始的会议列表
│   ├── list-ended     # 获取已结束的会议列表
│   └── invitees-list  # 获取会议受邀者列表
├── record
│   ├── list           # 查询录制列表
│   ├── address        # 获取录制文件下载地址
│   ├── smart-minutes  # 获取智能纪要
│   ├── transcript-get        # 获取转写详情
│   ├── transcript-paragraphs # 获取转写段落列表
│   └── transcript-search     # 搜索转写内容
└── report
    ├── participants   # 获取参会人列表
    └── waiting-room-log # 获取等候室成员列表
```

---

## 命令参考

### auth — 授权管理

#### `auth login`

登录并完成 OAuth2 授权，将凭证加密保存到本地。

```bash
tmeet auth login [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--no-browser` | bool | — | `false` | 禁用自动打开浏览器。`false`（默认）会尝试自动打开系统默认浏览器跳转到授权 URL；`true` 则仅输出授权 URL，需用户手动在浏览器中打开 |

执行后会输出授权 URL，CLI 自动轮询授权结果（超时 5 分钟），凭证加密保存到本地。

---

#### `auth logout`

登出并清除本地认证凭证。

```bash
tmeet auth logout
```

> 无参数。

---

#### `auth status`

查看当前登录状态，包括 OpenId、AccessToken / RefreshToken 的过期状态和剩余有效时间。

```bash
tmeet auth status
```

> 无参数。未登录时提示 `Not logged in`，已登录时展示凭证有效期信息。

---

### meeting — 会议管理

#### `meeting create` — 创建会议

```bash
tmeet meeting create --subject <主题> --start <开始时间> --end <结束时间> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明                                                                   |
|------|------|:----:|--------|----------------------------------------------------------------------|
| `--subject` | string | ✅ | — | 会议主题                                                                 |
| `--start` | string | ✅ | — | 会议开始时间，ISO 8601，如 `2026-03-12T14:00+08:00`                           |
| `--end` | string | ✅ | — | 会议结束时间，ISO 8601，如 `2026-03-12T15:00+08:00`                           |
| `--password` | string | — | — | 会议密码（4~6 位数字）                                                        |
| `--timezone` | string | — | — | 时区，可参见 Oracle-TimeZone 标准，如 `Asia/Shanghai`                          |
| `--meeting-type` | int | — | `0` | 会议类型：`0`-普通会议，`1`-周期性会议                                              |
| `--join-type` | int | — | `0` | 成员入会限制：`1`-所有成员可入会，`2`-仅受邀成员可入会，`3`-仅企业内部成员可入会                       |
| `--waiting-room` | bool | — | `false` | 是否开启等候室，`true`-开启，`false`-不开启                                        |
| `--recurring-type` | int | — | `0` | 周期类型（`--meeting-type=1` 时生效）：`0`-每天，`1`-每周一至周五，`2`-每周，`3`-每两周，`4`-每月 |
| `--until-type` | int | — | `0` | 周期结束类型（`--meeting-type=1` 时生效）：`0`-按日期结束重复，`1`-按次数结束重复               |
| `--until-count` | int | — | `7` | 限定会议次数（`--meeting-type=1` 时生效）：每天/每个工作日/每周最大 500，每两周/每月最大 50         |
| `--until-date` | string | — | — | 周期结束日期（`--meeting-type=1` 时生效），ISO 8601，如 `2026-03-12T15:00+08:00`   |

**示例：**

```bash
# 创建普通会议
tmeet meeting create \
  --subject "项目评审" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T16:00+08:00" \
  --password "123456" \
  --waiting-room

# 创建每周重复会议（共 10 次）
tmeet meeting create \
  --subject "每周站会" \
  --start "2026-04-10T09:30+08:00" \
  --end "2026-04-10T10:00+08:00" \
  --meeting-type 1 \
  --recurring-type 2 \
  --until-type 1 \
  --until-count 10
```

---

#### `meeting get` — 查询会议详情

`--meeting-id` 和 `--meeting-code` 二选一，`--meeting-id` 优先级更高。

```bash
tmeet meeting get --meeting-id <会议ID>
tmeet meeting get --meeting-code <会议码>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `--meeting-id` | string | 二选一 | 会议 ID（优先级高于会议码） |
| `--meeting-code` | string | 二选一 | 会议码 |

**示例：**

```bash
tmeet meeting get --meeting-id "6953553464429888300"
tmeet meeting get --meeting-code "931945029"
```

---

#### `meeting update` — 更新会议

仅传入需要修改的字段，未传入的字段保持不变。

```bash
tmeet meeting update --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明                                                                   |
|------|------|:----:|--------|----------------------------------------------------------------------|
| `--meeting-id` | string | ✅ | — | 会议 ID                                                                |
| `--subject` | string | — | — | 会议主题                                                                 |
| `--start` | string | — | — | 会议开始时间，ISO 8601，如 `2026-03-12T14:00+08:00`                           |
| `--end` | string | — | — | 会议结束时间，ISO 8601，如 `2026-03-12T14:00+08:00`                           |
| `--password` | string | — | — | 会议密码（4~6 位数字）                                                        |
| `--timezone` | string | — | — | 时区，如 `Asia/Shanghai`                                                 |
| `--meeting-type` | int | — | `0` | 会议类型：`0`-普通会议，`1`-周期性会议                                              |
| `--join-type` | int | — | `0` | 成员入会限制：`1`-所有成员可入会，`2`-仅受邀成员可入会，`3`-仅企业内部成员可入会                       |
| `--waiting-room` | bool | — | `false` | 是否开启等候室                                                              |
| `--recurring-type` | int | — | `0` | 周期类型（`--meeting-type=1` 时生效）：`0`-每天，`1`-每周一至周五，`2`-每周，`3`-每两周，`4`-每月 |
| `--until-type` | int | — | `0` | 周期结束类型（`--meeting-type=1` 时生效）：`0`-按日期结束重复，`1`-按次数结束重复               |
| `--until-count` | int | — | `7` | 限定会议次数（`--meeting-type=1` 时生效）：每天/每个工作日/每周最大 500，每两周/每月最大 50         |
| `--until-date` | string | — | — | 周期结束日期（`--meeting-type=1` 时生效），ISO 8601，如 `2026-03-12T15:00+08:00`   |

**示例：**

```bash
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --subject "新主题" \
  --start "2026-04-10T15:00+08:00" \
  --end "2026-04-10T16:00+08:00"
```

---

#### `meeting cancel` — 取消会议

```bash
tmeet meeting cancel --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--sub-meeting-id` | string | — | — | 周期性会议子会议 ID，取消周期性会议的某个子会议时需要传入 |
| `--meeting-type` | int | — | `0` | 会议类型：`0`-普通会议，`1`-周期性会议（取消整场周期性会议时传 `1`） |

**示例：**

```bash
# 取消普通会议
tmeet meeting cancel --meeting-id "6953553464429888300"

# 取消周期性会议中的某个子会议
tmeet meeting cancel \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "100001"

# 取消整场周期性会议
tmeet meeting cancel \
  --meeting-id "6953553464429888300" \
  --meeting-type 1
```

---

#### `meeting list` — 查询会议列表

查询进行中或即将开始的会议列表。

```bash
tmeet meeting list [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--start` | string | — | — | 分页查询起始时间值，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--end` | string | — | — | 分页查询结束时间值，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--show-all-sub` | int | — | `0` | 是否展示全部子会议：`0`-不展示，`1`-展示 |

**示例：**

```bash
tmeet meeting list
tmeet meeting list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --show-all-sub 1
```

---

#### `meeting list-ended` — 查询已结束会议列表

查询历史已结束的会议列表，支持按时间范围分页查询。

```bash
tmeet meeting list-ended [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--start` | string | — | — | 查询开始时间，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--end` | string | — | — | 查询结束时间，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--page` | int | — | `1` | 页码，从 1 开始 |
| `--page-size` | int | — | `10` | 每页大小，默认 10，最大 20 |

**示例：**

```bash
# 查询本月已结束的会议
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# 分页查询
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page 2 --page-size 20
```

---

#### `meeting invitees-list` — 查询受邀成员

```bash
tmeet meeting invitees-list --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--pos` | int | — | `0` | 分页获取受邀成员列表的查询起始位置值 |

**示例：**

```bash
tmeet meeting invitees-list --meeting-id "6953553464429888300"
tmeet meeting invitees-list --meeting-id "6953553464429888300" --pos 20
```

---

### record — 录制管理

#### `record list` — 查询录制列表

以下三组参数**任选其一**（均不传则报错）：
- `--start` + `--end`（时间范围）
- `--meeting-id`（会议 ID）
- `--meeting-code`（会议号）

```bash
tmeet record list (--start <开始时间> --end <结束时间> | --meeting-id <ID> | --meeting-code <会议号>) [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--start` | string | 三选一 | — | 查询开始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | 三选一 | — | 查询结束时间，ISO 8601，如 `2026-03-12T14:00+08:00`（与 `--start` 配合使用） |
| `--meeting-id` | string | 三选一 | — | 会议 ID |
| `--meeting-code` | string | 三选一 | — | 会议号 |
| `--page` | int | — | `1` | 页码，从 1 开始 |
| `--page-size` | int | — | `10` | 每页大小，最大 `20` |

**示例：**

```bash
# 按时间范围查询
tmeet record list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page 1 --page-size 20

# 按会议 ID 查询
tmeet record list --meeting-id "6953553464429888300"

# 按会议号查询
tmeet record list --meeting-code "931945029"
```

---

#### `record address` — 获取录制下载地址

```bash
tmeet record address --meeting-record-id <录制ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-record-id` | string | ✅ | — | 会议录制 ID |
| `--page` | int | — | `1` | 页码，从 1 开始 |
| `--page-size` | int | — | `50` | 每页大小，最大 `50` |

**示例：**

```bash
tmeet record address --meeting-record-id "record_abc123"
```

---

#### `record smart-minutes` — 获取智能纪要

```bash
tmeet record smart-minutes --record-file-id <文件ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--record-file-id` | string | ✅ | — | 录制文件 ID |
| `--lang` | string | — | `default` | 翻译语言选择：`default`-原文（不翻译），`zh`-简体中文，`en`-英文，`ja`-日语 |
| `--pwd` | string | — | — | 录制文件访问密码 |

**示例：**

```bash
tmeet record smart-minutes --record-file-id "file_abc123" --lang zh
```

---

#### `record transcript-get` — 获取转写详情

```bash
tmeet record transcript-get --record-file-id <文件ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--record-file-id` | string | ✅ | — | 录制文件 ID |
| `--meeting-id` | string | — | — | 会议 ID |
| `--pid` | string | — | — | 查询的起始段落 ID |
| `--limit` | string | — | — | 查询的段落数 |

**示例：**

```bash
tmeet record transcript-get --record-file-id "file_abc123" --pid "para_001" --limit "50"
```

---

#### `record transcript-paragraphs` — 获取转写段落列表

```bash
tmeet record transcript-paragraphs --record-file-id <文件ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--record-file-id` | string | ✅ | — | 录制文件 ID |
| `--meeting-id` | string | — | — | 会议 ID |

**示例：**

```bash
tmeet record transcript-paragraphs --record-file-id "file_abc123"
```

---

#### `record transcript-search` — 搜索转写内容

```bash
tmeet record transcript-search --record-file-id <文件ID> --text <关键词> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--record-file-id` | string | ✅ | — | 录制文件 ID |
| `--text` | string | ✅ | — | 搜索关键词 |
| `--meeting-id` | string | — | — | 会议 ID |

**示例：**

```bash
tmeet record transcript-search --record-file-id "file_abc123" --text "季度目标"
```

---

### report — 参会报告

#### `report participants` — 查询参会人列表

```bash
tmeet report participants --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--sub-meeting-id` | string | — | — | 周期性会议子会议 ID |
| `--pos` | int | — | `0` | 分页获取参会成员列表的查询起始位置值 |
| `--size` | int | — | `20` | 拉取参会成员条数，目前每页支持最大 100 条 |
| `--start` | string | — | — | 查询起始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | — | — | 查询结束时间，ISO 8601，如 `2026-03-12T14:00+08:00` |

**示例：**

```bash
tmeet report participants --meeting-id "6953553464429888300" --size 50
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"
```

---

#### `report waiting-room-log` — 查询等候室成员

```bash
tmeet report waiting-room-log --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--page` | int | — | `1` | 页码，默认为 1 |
| `--page-size` | int | — | `20` | 每页数量，最大 `50` |

**示例：**

```bash
tmeet report waiting-room-log --meeting-id "6953553464429888300" --page 1 --page-size 50
```

---

## 安全与风险提示（使用前必读）

---
**腾讯会议 CLI 工具接入 OpenClaw 等AI Agent 并获得您的授权后，AI 将会获得你在腾讯会议的数据访问权限（包括但不限于您的详细用户信息、管理和查询会议、录制和纪要等文件查询导出），并以您的用户身份在授权范围内执行操作。尽管工具有安全防护，AI仍可能因模型幻觉、提示词注入、投毒攻击、执行偏差不可控等原因，导致数据泄露、越权操作等执行非预期操作的高风险后果，请您谨慎操作和使用，并遵循你所在企业的数据安全等内部管理要求，避免造成数据丢失、泄露等损失。若怀疑泄露或需停用，请立即执行登出命令 `tmeet auth logout`。**

**请您充分理解并接受上述风险后再使用本工具，安装使用CLI后即视为您自愿承担相关责任。**



## 配置说明

配置文件默认存储在 `~/.tmeet/` 目录下，支持通过环境变量覆盖：

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `TMEET_CLI_CONFIG_DIR` | 配置文件目录 | `~/.tmeet/` |
| `TMEET_CLI_DATA_DIR` | 加密数据目录 | 平台相关默认路径 |

> **注意**：所有时间参数均使用 **ISO 8601** 格式，例如 `2026-04-10T14:00+08:00`。响应中的时间戳字段会自动转换为 ISO 8601 格式展示。

## 贡献指南

欢迎提交 Issue 和 Pull Request，请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 安全

如发现安全漏洞，请参阅 [SECURITY.md](SECURITY.md) 了解如何私下报告。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。
