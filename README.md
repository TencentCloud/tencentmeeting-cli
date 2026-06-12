# tencentmeeting-cli

[English](README_EN.md) | 中文

腾讯会议命令行工具（CLI），基于腾讯会议开放平台 OAuth2 授权，支持会议管理、录制管理、参会报告等功能。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org)

## 功能特性

- 🔐 **OAuth2 授权登录** — 设备码授权流程，安全无密码
- 📅 **会议管理** — 创建、查询、更新、取消会议，支持周期性会议，管理受邀成员
- 🎬 **录制管理** — 查询录制列表、获取下载地址、智能纪要、转写详情与搜索
- 📊 **参会报告** — 查询参会人列表、等候室成员记录
- 👥 **通讯录** — 按用户名/职位/部门检索企业通讯录成员
- 🛠️ **问题排查** — 导出本地日志，支持按时间范围过滤，打包为 zip 文件
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
| `--compact` | — | `false` | 精简输出模式：仅保留关键字段，过滤冗余字段以降低响应体积，适用于查询/列表类命令 |
| `--version` | `-V` | — | 查看版本号 |

**示例：**

```bash
# 查看版本号
tmeet -V

# 以缩进格式输出响应
tmeet meeting get --meeting-id "6953553464429888300" --format json-pretty

# 以精简模式输出查询结果（仅保留关键字段）
tmeet record list --meeting-id "6953553464429888300" --compact
```

---

## 分页参数说明

自 `v1.0.5` 起，所有支持分页的命令统一采用 **`--page-token` + `--page-size`** 方案。原先的 `--page` / `--pos` / `--pid` / `--size` / `--limit` 参数被标记为 **deprecated**，仍可使用但不再推荐，未来版本可能移除。

**统一用法：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `--page-token` | string | 分页游标。**首次查询不传**；后续翻页请将上一次响应中的 `next_page_token` 传入 |
| `--page-size` | int | 每页大小，不同命令默认值与上限不同，详见各命令说明 |

**典型分页流程：**

```bash
# 1) 首次查询（不传 page-token）
tmeet record list --meeting-id "6953553464429888300" --page-size 30

# 2) 从响应中取出 next_page_token，用于下一页
tmeet record list \
  --meeting-id "6953553464429888300" \
  --page-size 30 \
  --page-token "<next_page_token>"

# 3) 重复直到 next_page_token 为空，即已到最后一页
```

**各命令 `--page-size` 默认值/最大值速查：**

| 命令 | 默认值 | 最大值 | 旧参数（已弃用） |
|------|:---:|:------:|------|
| `meeting list` | 20  | 20 | — |
| `meeting list-ended` | 30  | 30 | `--page` |
| `meeting invitees-list` | 30  | 30 | `--pos` |
| `record list` | 30  | 30 | `--page` |
| `record address` | 30  | 30 | `--page` |
| `report participants` | 100 | 100 | `--pos` / `--size` |
| `report waiting-room-log` | 100 | 100 | `--page` |

> `record transcript-get` / `record transcript-paragraphs` / `record transcript-search` 暂不支持基于 `--page-token` 的新分页方案。
>
> 兼容性说明：当未传入 `--page-token` 且同时传入了旧分页参数（如 `--page`、`--pos`）时，CLI 会按旧模式发起请求（`page_type=0`）；否则一律按新模式（`page_type=1`）发起请求。

---

## 命令总览

```
tmeet [--format json|json-pretty] [--compact] [-V]
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
│   ├── invitees-list    # 获取会议受邀者列表
│   ├── invitees-add     # 添加会议受邀者
│   ├── invitees-remove  # 移除会议受邀者
│   └── invitees-replace # 替换会议受邀者列表
├── contact
│   ├── search         # 搜索企业通讯录成员
│   ├── lookup-by-email # 通过邮箱反查用户信息
│   └── lookup-by-phone # 通过手机号反查用户信息
├── record
│   ├── list           # 查询录制列表
│   ├── address        # 获取录制文件下载地址
│   ├── smart-minutes  # 获取智能纪要
│   ├── transcript-get          # 获取转写详情
│   ├── transcript-paragraphs   # 获取转写段落列表
│   ├── transcript-search       # 搜索转写内容
│   ├── permission-apply-prepare # 预览录制权限申请信息（申请前确认）
│   └── permission-apply-commit  # 提交录制权限申请（用户确认后执行）
├── report
│   ├── participants      # 获取参会人列表
│   └── waiting-room-log  # 获取等候室成员列表
├── control
│   ├── call           # 呼叫成员入会（会中邀请呼叫）
│   └── kick           # 将成员踢出会议（会中踢人）
└── tshoot
    ├── log               # 导出本地日志（支持按时间范围过滤，可选 --upload 上传至服务器）
    └── feedback          # 上报问题排查反馈到服务器
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
| `--invitees` | strings | — | — | 邀请成员的 openid 列表，逗号分隔或重复传参（最多 100 人，例如 `--invitees open_id1,open_id2`）          |

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

# 创建会议并邀请成员
tmeet meeting create \
  --subject "需求评审" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00" \
  --invitees "open_id1,open_id2,open_id3"
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
| `--invitees` | strings | — | — | 待变更的邀请成员 openid 列表，逗号分隔或重复传参；与 `--invitees-type` 配合使用              |
| `--invitees-type` | string | — | — | 邀请变更策略：`replace`-全量替换邀请列表，`add`-新增邀请用户，`remove`-删除邀请用户；当指定 `--invitees` 时必填 |

**示例：**

```bash
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --subject "新主题" \
  --start "2026-04-10T15:00+08:00" \
  --end "2026-04-10T16:00+08:00"

# 全量替换邀请列表
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2,open_id3" \
  --invitees-type replace

# 新增邀请用户
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id4,open_id5" \
  --invitees-type add

# 删除邀请用户
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1" \
  --invitees-type remove
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
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `20` | 每页大小，默认 20，最大 20 |

**示例：**

```bash
tmeet meeting list
tmeet meeting list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --show-all-sub 1

# 翻下一页
tmeet meeting list --page-token "<next_page_token>" --page-size 20
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
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |
| `--page` | int | — | — | ⚠️ **已弃用**：页码（从 1 开始），请改用 `--page-token` |

**示例：**

```bash
# 查询本月已结束的会议
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# 分页查询（使用 page-token）
tmeet meeting list-ended \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page-token "<next_page_token>" --page-size 30
```

---

#### `meeting invitees-list` — 查询受邀成员

```bash
tmeet meeting invitees-list --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |
| `--pos` | int | — | — | ⚠️ **已弃用**：分页起始位置值，请改用 `--page-token` |

**示例：**

```bash
tmeet meeting invitees-list --meeting-id "6953553464429888300"

# 翻下一页
tmeet meeting invitees-list \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 30
```

---

#### `meeting invitees-add` — 添加受邀成员

向已存在的会议中追加受邀成员。受邀成员通过用户 `open_id` 指定，可通过 `contact search` 命令查询获得。

```bash
tmeet meeting invitees-add --meeting-id <会议ID> --invitees <open_id列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--invitees` | strings | ✅ | — | 待添加的受邀成员 `open_id` 列表，支持英文逗号分隔或重复传入该参数，最多 100 个 |

**示例：**

```bash
# 通过英文逗号分隔传入多个 open_id
tmeet meeting invitees-add \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2"

# 重复传入 --invitees 参数
tmeet meeting invitees-add \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1" \
  --invitees "open_id2"
```

---

#### `meeting invitees-remove` — 移除受邀成员

从已存在的会议中移除指定的受邀成员。

```bash
tmeet meeting invitees-remove --meeting-id <会议ID> --invitees <open_id列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--invitees` | strings | ✅ | — | 待移除的受邀成员 `open_id` 列表，支持英文逗号分隔或重复传入该参数，最多 100 个 |

**示例：**

```bash
tmeet meeting invitees-remove \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2"
```

---

#### `meeting invitees-replace` — 替换受邀成员列表

使用新的成员列表整体替换会议当前的受邀成员列表（未在 `--invitees` 中的成员将被移除）。

```bash
tmeet meeting invitees-replace --meeting-id <会议ID> --invitees <open_id列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--invitees` | strings | ✅ | — | 替换后的受邀成员 `open_id` 列表，支持英文逗号分隔或重复传入该参数，最多 100 个 |

**示例：**

```bash
tmeet meeting invitees-replace \
  --meeting-id "6953553464429888300" \
  --invitees "open_id1,open_id2,open_id3"
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
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |
| `--page` | int | — | — | ⚠️ **已弃用**：页码（从 1 开始），请改用 `--page-token` |

**示例：**

```bash
# 按时间范围查询
tmeet record list \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --page-token "<next_page_token>" --page-size 30

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
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |
| `--page` | int | — | — | ⚠️ **已弃用**：页码（从 1 开始），请改用 `--page-token` |

**示例：**

```bash
tmeet record address --meeting-record-id "record_abc123"

# 翻下一页
tmeet record address \
  --meeting-record-id "record_abc123" \
  --page-token "<next_page_token>" --page-size 30
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
tmeet record transcript-get --record-file-id "file_abc123"

# 指定起始段落与数量
tmeet record transcript-get --record-file-id "file_abc123" --pid "<paragraph_id>" --limit "30"
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

# 指定会议 ID
tmeet record transcript-paragraphs \
  --record-file-id "file_abc123" \
  --meeting-id "6953553464429888300"
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

#### `record permission-apply-prepare` — 预览录制权限申请

申请录制权限前先调用本命令获取审批文案/会议主题/录制所有者等信息，**展示给用户二次确认后**再执行 `record permission-apply-commit` 真正提交申请。

```bash
tmeet record permission-apply-prepare --meeting-record-id <录制ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-record-id` | string | ✅ | — | 会议录制 ID |
| `--meeting-id` | string | — | — | 会议 ID |

**示例：**

```bash
tmeet record permission-apply-prepare --meeting-record-id "record_abc123"
```

响应 `data` 主要字段：

| 字段 | 说明 |
|------|------|
| `preview.meeting_record_id` | 会议录制 ID |
| `preview.approval_name` | 申请类型文案 |
| `preview.subject` | 会议标题 |
| `preview.file_owner` | 录制所有者名称 |
| `preview.apply_note` | 权限申请备注信息 |
| `preview.applicant` | 申请人名称 |
| `expires_in` | 过期时间（秒） |

---

#### `record permission-apply-commit` — 提交录制权限申请

**写操作**：在 `permission-apply-prepare` 获取预览信息并经用户确认后调用，正式发起权限申请审批流程。

```bash
tmeet record permission-apply-commit --meeting-record-id <录制ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-record-id` | string | ✅ | — | 会议录制 ID |
| `--meeting-id` | string | — | — | 会议 ID |

**示例：**

```bash
tmeet record permission-apply-commit --meeting-record-id "record_abc123"
```

响应 `data` 主要字段：

| 字段 | 说明 |
|------|------|
| `unique_id` | 申请 ID |
| `status` | 审批状态 |
| `message` | 审批状态描述 |
| `approval_url` | 审批链接 |
| `share_text` | 申请说明描述 |

---

### contact — 通讯录

#### `contact search` — 搜索企业通讯录成员

按用户名搜索企业通讯录成员，支持通过职位或部门进一步过滤搜索结果。

```bash
tmeet contact search --username <用户名> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--username` | string | ✅ | — | 要搜索的用户名 |
| `--job-title` | string | — | — | 当用户名搜索结果过多时，用于过滤的职位名称 |
| `--department-name` | string | — | — | 当用户名搜索结果过多时，用于过滤的部门名称 |

**示例：**

```bash
# 按用户名搜索
tmeet contact search --username "张三"

# 用户名 + 职位过滤
tmeet contact search --username "张三" --job-title "工程师"

# 用户名 + 部门过滤
tmeet contact search --username "张三" --department-name "研发部"
```

---

#### `contact lookup-by-email` — 通过邮箱反查用户信息

通过邮箱地址反查用户详细信息，支持批量查询多个邮箱。

```bash
tmeet contact lookup-by-email --emails <邮箱地址列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--emails` | []string | ✅ | — | 邮箱地址列表，多个邮箱用逗号分隔或重复使用该参数，最多50个<br>例如：--emails user1@example.com,user2@example.com 或 --emails user1@example.com --emails user2@example.com |

**示例：**

```bash
# 查询单个邮箱
tmeet contact lookup-by-email --emails "user@example.com"

# 批量查询多个邮箱
tmeet contact lookup-by-email --emails "user1@example.com,user2@example.com,user3@example.com"
```

---

#### `contact lookup-by-phone` — 通过手机号反查用户信息

通过手机号反查用户详细信息，支持批量查询多个手机号。

```bash
tmeet contact lookup-by-phone --phones <手机号列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--phones` | []string | ✅ | — | 手机号列表，多个手机号用逗号分隔或重复使用该参数，最多50个<br>例如：--phones 13800138000,13900139000 或 --phones 13800138000 --phones 13900139000 |

**示例：**

```bash
# 查询单个手机号
tmeet contact lookup-by-phone --phones "13800138000"

# 批量查询多个手机号
tmeet contact lookup-by-phone --phones "13800138000,13900139000,13700137000"
```

---

### report — 参会报告

#### `report participants` — 查询参会人列表

```bash
tmeet report participants --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值   | 说明                                         |
|------|------|:----:|-------|--------------------------------------------|
| `--meeting-id` | string | ✅ | —     | 会议 ID                                      |
| `--sub-meeting-id` | string | — | —     | 周期性会议子会议 ID                                |
| `--start` | string | — | —     | 查询起始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | — | —     | 查询结束时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--page-token` | string | — | —     | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传  |
| `--page-size` | int | — | `100` | 每页大小，默认 100，最大 100                         |
| `--pos` | int | — | —     | ⚠️ **已弃用**：分页起始位置值，请改用 `--page-token`      |
| `--size` | int | — | —     | ⚠️ **已弃用**：每页条数，请改用 `--page-size`          |

**示例：**

```bash
tmeet report participants --meeting-id "6953553464429888300" --page-size 50
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T10:00+08:00" \
  --end "2026-04-10T11:00+08:00"

# 翻下一页
tmeet report participants \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 50
```

---

#### `report waiting-room-log` — 查询等候室成员

```bash
tmeet report waiting-room-log --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值   | 说明                                        |
|------|------|:----:|-------|-------------------------------------------|
| `--meeting-id` | string | ✅ | —     | 会议 ID                                     |
| `--page-token` | string | — | —     | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `100` | 每页大小，默认 100，最大 100                        |
| `--page` | int | — | —     | ⚠️ **已弃用**：页码，请改用 `--page-token`          |

**示例：**

```bash
tmeet report waiting-room-log --meeting-id "6953553464429888300" --page-size 50

# 翻下一页
tmeet report waiting-room-log \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 50
```

---

### control — 会中控制

会中控制相关命令，用于在会议进行中对参会成员执行呼叫、踢出等管理操作。受邀成员通过用户 `open_id` 指定，可通过 `contact search` 命令查询获得。

#### `control call` — 呼叫成员入会

会中邀请呼叫，向指定成员发起入会呼叫。

```bash
tmeet control call --meeting-id <会议ID> --users <open_id列表>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--users` | strings | ✅ | — | 待呼叫的成员 `open_id` 列表，支持英文逗号分隔或重复传入该参数，最多 20 个 |

**示例：**

```bash
# 通过英文逗号分隔传入多个 open_id
tmeet control call \
  --meeting-id "6953553464429888300" \
  --users "open_id1,open_id2"

# 重复传入 --users 参数
tmeet control call \
  --meeting-id "6953553464429888300" \
  --users "open_id1" \
  --users "open_id2"
```

---

#### `control kick` — 踢出会议成员

会中踢人，将指定成员从会议中踢出。

```bash
tmeet control kick --meeting-id <会议ID> [--users <open_id列表>] [--sip-users <ms_open_id列表>] [--pstn-users <ms_open_id列表>] [--allow-rejoin]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--users` | strings | 三选一 | — | 待踢出的普通成员 `open_id` 列表（不包含 Sip/Pstn 设备），支持英文逗号分隔或重复传入该参数 |
| `--sip-users` | strings | 三选一 | — | 待踢出的 Sip 设备 `ms_open_id` 列表，支持英文逗号分隔或重复传入该参数 |
| `--pstn-users` | strings | 三选一 | — | 待踢出的 Pstn 设备 `ms_open_id` 列表，支持英文逗号分隔或重复传入该参数 |
| `--allow-rejoin` | bool | ❌ | `false` | 被踢出的成员是否允许重新加入会议；不传则默认 `false`（不允许重新入会） |

> `--users` / `--sip-users` / `--pstn-users` **三者至少必填一种**，且**三者总数合计最多 20 个**。

**示例：**

```bash
# 踢出普通成员
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --users "open_id1,open_id2"

# 同时踢出普通成员、Sip 设备、Pstn 设备（三者合计不超过 20）
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --users "open_id1" \
  --sip-users "ms_open_id_sip1" \
  --pstn-users "ms_open_id_pstn1"

# 允许被踢成员重新入会
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --allow-rejoin \
  --users "open_id1,open_id2"
```

---

### tshoot — 问题排查

#### `tshoot log` — 导出本地日志

将本地日志打包为 zip 文件，输出到 `~/tmeet_ts_{datetime}.zip`，可用于问题排查。支持按时间范围过滤，不传时间参数则导出全部日志。

```bash
tmeet tshoot log [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--start` | string | 与 `--end` 同时使用 | — | 日志查询开始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | 与 `--start` 同时使用 | — | 日志查询结束时间，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--upload` | bool | 否 | `false` | 上传日志到服务器，需要登录 |

> `--start` 和 `--end` 必须同时传入或同时不传。

**示例：**

```bash
# 导出全部日志
tmeet tshoot log

# 导出指定时间范围内的日志
tmeet tshoot log \
  --start "2026-04-10T00:00+08:00" \
  --end "2026-04-10T23:59+08:00"

# 导出日志并上传到服务器（需要登录）
tmeet tshoot log --upload
```

输出示例：
```
output log saved to: ~/tmeet_ts_20260410_153000.zip
```

---

#### `tshoot feedback` — 上报问题排查反馈

将 Agent 在使用 CLI 过程中遇到的问题或建议上报至服务器，便于后续优化工具能力。

```bash
tmeet tshoot feedback --category <分类> --intent <原始意图> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--category` | string | ✅ | — | 反馈分类，可选值：`tool_not_found`（想做某事但找不到匹配工具）、`tool_error`（调用工具但返回错误）、`tool_inadequate`（工具存在但能力/参数不足）、`unexpected_result`（调用成功但结果未达预期）、`suggestion`（一般性建议或改进想法） |
| `--intent` | string | ✅ | — | Agent 的原始意图，最多 200 字符 |
| `--actions-tried` | string | — | — | Agent 已尝试过的动作，最多 500 字符 |
| `--result` | string | — | — | 已尝试动作的结果或阻塞点，最多 500 字符 |
| `--tool-name` | string | — | — | 使用的工具/命令名 |
| `--error-code` | string | — | — | 工具返回的错误码 |

**示例：**

```bash
# 反馈：找不到匹配工具
tmeet tshoot feedback \
  --category "tool_not_found" \
  --intent "想批量导出某个时间段的所有会议纪要" \
  --actions-tried "查看了 record 和 meeting 子命令" \
  --result "未找到批量导出纪要的命令"

# 反馈：工具调用返回错误
tmeet tshoot feedback \
  --category "tool_error" \
  --intent "获取录制下载地址" \
  --tool-name "record address" \
  --error-code "200003" \
  --result "接口返回权限不足"

# 反馈：一般性建议
tmeet tshoot feedback \
  --category "suggestion" \
  --intent "希望支持按主题模糊搜索会议"
```

> 该命令需要登录后才能使用。

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