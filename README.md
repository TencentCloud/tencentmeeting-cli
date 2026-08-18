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

自 `v1.0.5` 起，所有支持分页的命令统一采用 **`--page-token` + `--page-size`** 方案。原先的 `--page` / `--pos` / `--size` 参数被标记为 **deprecated**，仍可使用但不再推荐，未来版本可能移除。

> 说明：`record transcript-get` 的 `--pid` / `--limit` 是该命令用于段落定位的独立参数，**不属于**通用分页参数，未被弃用。

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
| `meeting search` | 30  | 30 | — |
| `meeting invitees-list` | 30  | 30 | `--pos` |
| `record list` | 30  | 30 | `--page` |
| `record address` | 30  | 30 | `--page` |
| `record search` | 30  | 30 | — |
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
│   ├── search         # 按关键词/会议号/时间范围搜索会议
│   ├── invitees-list    # 获取会议受邀者列表
│   ├── invitees-add     # 添加会议受邀者
│   ├── invitees-remove  # 移除会议受邀者
│   ├── invitees-replace # 替换会议受邀者列表
│   ├── join-as-agent    # 以子账号身份加入会议并自动开启实时转写
│   └── leave-as-agent   # 以子账号身份离开会议
├── contact
│   ├── search         # 搜索企业通讯录成员
│   ├── lookup-by-email # 通过邮箱反查用户信息
│   └── lookup-by-phone # 通过手机号反查用户信息
├── record
│   ├── list           # 查询录制列表
│   ├── address        # 获取录制文件下载地址
│   ├── search         # 按关键词/会议号/会议ID/时间范围搜索录制
│   ├── smart-minutes  # 获取智能纪要
│   ├── transcript-get          # 获取转写详情
│   ├── transcript-paragraphs   # 获取转写段落列表
│   ├── transcript-search       # 搜索转写内容
│   ├── permission-apply-prepare # 预览录制权限申请信息（申请前确认）
│   └── permission-apply-commit  # 提交录制权限申请（用户确认后执行）
├── report
│   ├── participants         # 获取参会人列表
│   ├── waiting-room-log     # 获取等候室成员列表
│   ├── participants-export  # 导出参会成员明细（异步任务）
│   └── job-result           # 获取异步任务结果
├── control
│   ├── call           # 呼叫成员入会（会中邀请呼叫）
│   ├── kick           # 将成员踢出会议（会中踢人）
│   └── waiting-room   # 等候室管理（移入会议/移回等候室/移出）
├── tshoot
│   ├── log               # 导出本地日志（支持按时间范围过滤，可选 --upload 上传至服务器）
│   └── feedback          # 上报问题排查反馈到服务器
├── event
│   ├── list           # 列出可订阅的 EventKey
│   ├── schema         # 查看 EventKey 的参数 / 输出 schema
│   ├── consume        # 订阅 EventKey，按 NDJSON 流式输出事件
│   ├── status         # 查看本机 bus 守护进程状态
│   └── stop           # 停止本机 bus 守护进程（可选 --force 清理残留）
└── agent
    ├── create         # 在主账号下创建子账号（agent）并保存凭证
    ├── delete         # 删除当前主账号下的子账号并清除本地凭证
    ├── token          # 为当前子账号重新签发 access_token / refresh_token
    ├── list           # 列出当前主账号下的子账号列表
    └── get            # 查询单个子账号详情
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
| `--water-mark-type` | int | — | `2` | 文字水印：`0`-单排，`1`-双排，`2`-关闭<br>● 个人账号：默认为2<br>● 企业/组织账号：<br>  ✧ 企业设置强制态-使用企业设置作为强制态，入参不生效<br>  ✧ 企业未设置强制态-使用企业设置作为默认值，入参覆盖默认值 |
| `--audio-watermark` | bool | — | `false` | 音频水印：`true`-开启，`false`-关闭<br>● 个人账号：默认为false<br>● 企业/组织账号：<br>  ✧ 企业设置强制态-使用企业设置作为强制态，入参不生效<br>  ✧ 企业未设置强制态-使用企业设置作为默认值，入参覆盖默认值 |
| `--auto-record-type` | string | — | `none` | 主持人入会后自动录制会议：`none`-关，`local`-本地，`cloud`-云录制<br>● 个人账号：默认none<br>● 企业/组织账号：<br>  ✧ 企业设置强制态-使用企业设置作为强制态，入参不生效<br>  ✧ 企业未设置强制态-使用企业设置作为默认值，入参覆盖默认值 |
| `--auto-asr` | bool | — | `false` | 自动文字转写：`true`-开，`false`-关<br>● 个人账号：默认false<br>● 企业/组织账号：<br>  ✧ 企业设置强制态-使用企业设置作为强制态，入参不生效<br>  ✧ 企业未设置强制态-使用企业设置作为默认值，入参覆盖默认值 |

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

# 创建会议并显式关闭音频水印 / 自动文字转写
# 注：bool 参数传 false 必须使用 = 形式，不能用空格
tmeet meeting create \
  --subject "无水印会议" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00" \
  --audio-watermark=false \
  --auto-asr=false
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
| `--sub-meeting-id` | string | — | — | 子会议 ID（`--meeting-type=1` 时生效）：仅修改该场子会议的时间；**不可与 `--recurring-type` / `--until-type` / `--until-count` / `--until-date` 同时使用**。不填则修改整个周期性会议 |
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

# 仅修改周期性会议中某一场子会议的时间（不修改周期规则）
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --meeting-type 1 \
  --sub-meeting-id "100001" \
  --start "2026-04-17T10:00+08:00" \
  --end "2026-04-17T11:00+08:00"

# 显式关闭音频水印 / 自动文字转写
# 注：bool 参数传 false 必须使用 = 形式，不能用空格
tmeet meeting update \
  --meeting-id "6953553464429888300" \
  --audio-watermark=false \
  --auto-asr=false
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

#### `meeting search` — 搜索会议

按关键词、会议号、时间范围等条件搜索会议。所有过滤参数均为可选，可任意组合。

```bash
tmeet meeting search [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--query` | string | — | — | 搜索关键词 |
| `--query-field` | string | — | `all` | `--query` 的搜索字段：`subject`-会议主题；`creator`-创建者昵称/备注名；`note`-用户对会议的备注；`all`-搜索所有字段 |
| `--meeting-code` | string | — | — | 按会议号过滤（精确匹配，仅数字，无短横线） |
| `--start` | string | — | — | 搜索时间窗下限（ISO 8601，如 `2026-03-12T15:00+08:00`）。匹配条件：会议预约开始时间、实际开始时间或当前用户加入时间任一落在窗口内 |
| `--end` | string | — | — | 搜索时间窗上限（ISO 8601，如 `2026-03-12T15:00+08:00`），语义同上 |
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |

**示例：**

```bash
# 按主题关键词搜索
tmeet meeting search --query "周例会" --query-field subject

# 按创建者昵称搜索
tmeet meeting search --query "张三" --query-field creator

# 按会议号精确搜索
tmeet meeting search --meeting-code "931945029"

# 按时间范围搜索
tmeet meeting search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# 翻下一页
tmeet meeting search \
  --query "项目评审" \
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

#### `meeting join-as-agent` — 以子账号身份加入会议

以子账号（agent）身份加入指定会议，加入后自动开启实时转写（ASR），适用于用户无法亲自参会但需要获取会议内容的场景。

> **前置条件：** 需先通过 `tmeet agent create` 创建子账号并确保 token 有效。

```bash
tmeet meeting join-as-agent --meeting-code <会议号> --agent-id <值> [--password <值>] [--require-realtime-asr]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-code` | string | ✅ | — | 会议号，用于指定要加入的会议 |
| `--agent-id` | string | ✅ | — | Agent ID，指定要作为代理加入会议的 Agent 标识 |
| `--password` | string | — | — | 入会密码，通常是 4-6 位数字；如果会议未设置密码可省略 |
| `--require-realtime-asr` | bool | — | `false` | 若为 true 且 ASR 开启失败，bot 将自动离开会议（离开原因：asr-denied） |

**行为说明：**

1. 以子账号身份加入会议（通过 agent 凭证鉴权）
2. 加入成功后，先查询会议当前实时转写状态
3. 若转写已开启，跳过开启步骤
4. 若转写未开启，自动调用开启实时转写（ASR）
5. 若加入失败，直接返回错误
6. 若加入成功但 ASR 开启失败：
   - `--require-realtime-asr=true`：bot 自动离开会议，命令返回错误（asr-denied）
   - `--require-realtime-asr=false`（默认）：命令仍返回成功，并附带提示：`joined the meeting successfully, but failed to enable real-time transcription`

**示例：**

```bash
tmeet meeting join-as-agent --meeting-code "806146667" --agent-id "agent_xxxx"
tmeet meeting join-as-agent --meeting-code "806146667" --agent-id "agent_xxxx" --password "1234"
tmeet meeting join-as-agent --meeting-code "806146667" --agent-id "agent_xxxx" --require-realtime-asr=false
```

---

#### `meeting leave-as-agent` — 以子账号身份离开会议

以子账号（agent）身份离开指定会议。

> **前置条件：** 子账号当前已在会议中（通过 `join-as-agent` 加入）。

```bash
tmeet meeting leave-as-agent --meeting-id <会议ID> --agent-id <子账号ID>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--agent-id` | string | ✅ | — | 子账号 ID |

**示例：**

```bash
tmeet meeting leave-as-agent --meeting-id "6953553464429888300" --agent-id "agent_xxxx"
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

#### `record search` — 搜索录制

按关键词、会议号、会议 ID、时间范围、文件类型等条件搜索录制。所有过滤参数均为可选，可任意组合。

```bash
tmeet record search [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--query` | string | — | — | 搜索关键词 |
| `--query-field` | string | — | `all` | `--query` 的搜索字段：`subject`-录制主题；`creator`-会议创建者昵称/备注名；`transcript_content`-文件中的原始转写内容；`smart_minutes`-文件中的智能纪要内容（摘要 + 待办）；`timeline`-文件中的时间轴内容；`all`-搜索所有字段 |
| `--file-type` | string | — | `all` | 文件类型：`video`、`audio`、`transcript`、`upload`、`external`、`all` |
| `--meeting-id` | string | — | — | 按会议 ID 过滤 |
| `--meeting-code` | string | — | — | 按会议号过滤（精确匹配，仅数字，无短横线） |
| `--start` | string | — | — | 查询开始时间（ISO 8601，如 `2026-03-12T14:00+08:00`） |
| `--end` | string | — | — | 查询结束时间（ISO 8601，如 `2026-03-12T14:00+08:00`） |
| `--page-token` | string | — | — | 分页游标，从上一次响应中返回的 `next_page_token` 获取，首页不传 |
| `--page-size` | int | — | `30` | 每页大小，默认 30，最大 30 |

**示例：**

```bash
# 按转写内容关键词搜索
tmeet record search --query "季度目标" --query-field transcript_content

# 按智能纪要内容搜索
tmeet record search --query "待办" --query-field smart_minutes

# 按会议 ID 过滤
tmeet record search --meeting-id "6953553464429888300"

# 按时间范围 + 文件类型搜索
tmeet record search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00" \
  --file-type video

# 翻下一页
tmeet record search \
  --query "项目评审" \
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

#### `report participants-export` — 导出参会成员明细

异步导出会议参会成员明细，本命令仅提交导出任务并返回 `job_id`，需配合 `report job-result` 轮询任务状态获取下载链接。

```bash
tmeet report participants-export --meeting-id <会议ID> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--sub-meeting-id` | string | — | — | 周期性会议子会议 ID |
| `--start` | string | — | — | 查询起始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | — | — | 查询结束时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--file-type` | string | — | `xlsx` | 导出文件格式：`xlsx` 或 `json` |

**响应关键字段：**

| 字段 | 说明 |
|------|------|
| `job_id` | 异步任务 ID（用于轮询任务状态） |

> 本命令仅返回 `job_id`，不会自动等待任务完成。获取 `job_id` 后，需每隔 5 秒调用 `report job-result` 轮询任务状态，直到 status 为 "成功" 时获取下载链接，或 status 非 "处理中" 时终止。

**示例：**

```bash
# 导出会议参会成员明细（默认 xlsx 格式）
tmeet report participants-export --meeting-id "6953553464429888300"

# 导出为 json 格式
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --file-type "json"

# 导出周期性会议某个子会议的参会成员
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "200000001"

# 按时间范围过滤
tmeet report participants-export \
  --meeting-id "6953553464429888300" \
  --start "2026-04-10T14:00+08:00" \
  --end "2026-04-10T15:00+08:00"
```

---

#### `report job-result` — 获取异步任务结果

查询异步导出任务的执行状态与结果。调用 `participants-export` 获取 `job_id` 后，需每隔 5 秒调用本命令轮询，直到任务完成或失败。

```bash
tmeet report job-result --job-id <任务ID>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--job-id` | string | ✅ | — | 任务 ID（从 `participants-export` 获取） |

**响应关键字段：**

| 字段 | 说明 |
|------|------|
| `status` | 任务状态："成功"、"失败"、"处理中" |
| `url` | 文件下载链接（状态为 "成功" 时返回，有效期 2 小时） |
| `error_msg` | 错误信息（状态为 "失败" 时返回） |

**示例：**

```bash
# 查询异步任务结果
tmeet report job-result --job-id "e1234567-f123-4d12-123a-12346192e332"
```

**导出参会成员明细完整工作流：**

```
1. 提交导出任务，获取 job_id
   tmeet report participants-export --meeting-id "6953553464429888300"

2. 每隔 5 秒调用 job-result 轮询任务状态
   tmeet report job-result --job-id <job_id>

3. 根据返回的 status 判断：
   - status = "成功"：返回文件下载链接 url（有效期 2 小时），流程结束
   - status = "处理中"：等待 5 秒后再次调用 job-result 继续轮询
   - status = "失败" 或其他值：终止并返回 error_msg
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

#### `control waiting-room` — 等候室管理

管理会议中等候室成员，支持三种操作类型：

- **enter-meeting**：主持人将等候室成员移入会议
- **back-to-waiting**：主持人将会中成员移回等候室
- **expel**：主持人将等候室成员移出（踢出会议）

```bash
tmeet control waiting-room --meeting-id <会议ID> --operate-type <操作类型> [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--meeting-id` | string | ✅ | — | 会议 ID |
| `--operate-type` | string | ✅ | — | 操作类型：`enter-meeting`（主持人将等候室成员移入会议）、`back-to-waiting`（主持人将会中成员移入等候室）、`expel`（主持人将等候室成员移出） |
| `--users` | strings | 三选一 | — | 待操作的普通成员 `open_id` 列表（不含 Sip/Pstn 设备），支持英文逗号分隔或重复传入该参数 |
| `--sip-users` | strings | 三选一 | — | 待操作的 Sip 设备 `ms_open_id` 列表，支持英文逗号分隔或重复传入该参数 |
| `--pstn-users` | strings | 三选一 | — | 待操作的 Pstn 设备 `ms_open_id` 列表，支持英文逗号分隔或重复传入该参数 |
| `--allow-rejoin` | bool | ❌ | — | 移出后是否允许再次加入会议（仅 `--operate-type=expel` 时生效）； |

> `--users` / `--sip-users` / `--pstn-users` **三者至少必填一种**，且**三者总数合计最多 20 个**。

**示例：**

```bash
# 将等候室成员移入会议
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type enter-meeting \
  --users "open_id1,open_id2"

# 将会中成员移回等候室
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type back-to-waiting \
  --users "open_id1,open_id2"

# 将等候室成员移出，不允许再次加入
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --users "open_id1,open_id2"

# 将等候室成员移出，允许再次加入会议
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --allow-rejoin \
  --users "open_id1,open_id2"

# 将等候室成员移出，显式不允许再次加入会议
# 注意：bool 类型显式设为 false 时必须使用等号语法 --allow-rejoin=false，不能写成 --allow-rejoin false
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --allow-rejoin=false \
  --users "open_id1,open_id2"

# 同时操作 sip 设备和 pstn 设备
tmeet control waiting-room \
  --meeting-id "6953553464429888300" \
  --operate-type expel \
  --sip-users "ms_open_id_sip1" \
  --pstn-users "ms_open_id_pstn1"
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
| `--allow-rejoin` | bool | ❌ | `true` | 被踢出的成员是否允许重新加入会议；不传则默认 `true`（允许重新入会），传 `--allow-rejoin=false` 不允许重新入会 |

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

# 不允许被踢成员重新入会
tmeet control kick \
  --meeting-id "6953553464429888300" \
  --allow-rejoin=false \
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

### event — 实时事件订阅

通过本机后台 **bus 守护进程**（per-host daemon）订阅腾讯会议的实时事件（如 `meeting.started`、`meeting.end` 等）。所有 `tmeet event consume` 消费者复用同一条 WSS 长连接，由 bus 进程统一管理握手 / 心跳 / 自动重连。

事件以 NDJSON（每行一个 JSON 对象）方式写入 **stdout**；连接握手、source 状态、丢弃告警等控制面诊断信息写入 **stderr**；适合直接被 Agent 或脚本通过管道消费。

**通用约定：**

- **stdout / stderr 分离**：业务事件只写 stdout（NDJSON），所有诊断 / 状态 / 告警仅写 stderr。
- **ready 标记**：`event consume` 完成握手并就绪后，会在 stderr 输出一行稳定的就绪标记：

  ```text
  [event] ready event_key=<key>
  ```

  即使开启 `--quiet` 也不会被屏蔽。Agent 可 grep 此行来判断订阅就绪、再触发后续动作。
- **退出标记**：`event consume` 退出时同样会在 stderr 输出汇总（不受 `--quiet` 屏蔽）：

  ```text
  [event] exited — received <N> event(s) in <duration> (reason: <reason>)
  ```

  `reason` 取值：`limit` / `timeout` / `signal` / `shutdown`。
- **退出码约定**：
  - `0` — 正常退出（达到 `--max-events` / `--timeout` / 收到 SIGINT/SIGTERM / bus 主动关闭）。
  - `1` — 致命错误（Hello 被拒、未知 EventKey、IO 错误、订阅失败等）。
  - `2` — 仅 `event status --fail-on-orphan` 与 `event stop` 在 `refused` / `errored` 状态时返回，便于健康检查脚本分支处理。

> `event _bus` 为隐藏子命令（由 `event consume` 自动拉起），不建议人工调用。

---

#### `event list` — 列出可订阅的 EventKey

列出当前 CLI 编译时内置的全部 EventKey，输出为按 `(domain, key)` 排序的 JSON 数组。

> 该命令读取本地内置注册表，**不依赖登录**，也不发起任何远程调用。

```bash
tmeet event list [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--domain` | string | — | — | 仅展示该 domain 下的 EventKey（如 `meeting`、`record`）；未知 domain 会以退出码 1 返回并提示已知 domain 列表 |

**输出字段：**

| 字段 | 说明 |
|------|------|
| `key` | EventKey 名称，例如 `meeting.started` |
| `domain` | 所属 domain，例如 `meeting` |
| `description` | 简短描述 |

**示例：**

```bash
# 列出全部 EventKey
tmeet event list

# 仅展示 meeting 域下的 EventKey，并以缩进格式输出
tmeet event list --domain meeting --format json-pretty
```

---

#### `event schema` — 查看 EventKey 的完整契约

输出指定 EventKey 的参数 schema（`--param` 可用的 key）、事件 payload 的 JSON Schema、以及 jq 表达式使用的根路径。

> 同样为本地注册表查询，**不依赖登录**。

```bash
tmeet event schema <EventKey>
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| `<EventKey>` | string | ✅ | 位置参数，要查询的 EventKey；未知 key 会以退出码 1 返回并提示用 `event list` 查询 |

**输出字段：**

| 字段 | 说明 |
|------|------|
| `key` | EventKey 名称 |
| `domain` | 所属 domain |
| `jq_root_path` | `--jq` 表达式的根路径，取值为 `.`（整包络）或 `.payload`（仅 payload） |
| `params_schema` | `--param key=value` 可接受的参数定义（map） |
| `resolved_output_schema` | 事件 payload 的 JSON Schema |

**示例：**

```bash
tmeet event schema meeting.started --format json-pretty
```

---

#### `event consume` — 订阅事件并按 NDJSON 流式输出

订阅指定 EventKey 的事件流，每条事件以一行 NDJSON 写入 stdout。底层 bus 守护进程未运行时，会自动 fork 一个出来。

两种运行模式：

- **批处理**：传入 `--max-events` 或 `--timeout`，首次满足条件即退出（退出码 0）。
- **常驻**：两者都不传，直到收到 SIGINT/SIGTERM、或由 `tmeet event stop` 关闭 bus 时退出。

```bash
tmeet event consume --event-id <EventKey> [选项]
```

> `--event-id` 只接受单个 EventKey；传入位置参数会以退出码 1 拒绝。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--event-id` | string | ✅ | — | 要订阅的 EventKey（必须为 `event list` 中已注册的 key）；只接受单个 EventKey |
| `--param` | strings | — | — | 形如 `key=value` 的订阅过滤参数，可重复传入；可用 key 由 `event schema <key>.params_schema` 给出 |
| `--max-events` | int | — | `0` | 累计接收到 N 条事件后退出，`0` 表示不限制 |
| `--timeout` | duration | — | `0` | 自 ready 标记起 N 时长后退出，`0` 表示不限制（如 `30s`、`5m`） |
| `--quiet` | bool | — | `false` | 抑制信息型 stderr 输出；ready / exit / WARN / 错误等关键诊断仍会输出 |
| `--output-dir` | string | — | — | 额外把每条事件写入 `<output-dir>/<trace_id>.json`，仅允许相对路径，不允许 `..` 段；目录不存在会自动创建 |
| `--jq` | string | — | — | 对每条事件执行 gojq 表达式：返回 null / 无结果时丢弃该事件，否则其输出替换默认的 NDJSON 行 |

**stdout 输出格式（默认）：**

```json
{"event":"meeting.started","trace_id":"<id>","payload":{...}}
```

**stderr 控制面诊断**（信息型可被 `--quiet` 抑制；ready / exit / WARN / 握手失败始终输出）：

```text
[event] starting consume key=<key>
[event] bus not running, forked daemon                      # 仅在自动拉起 bus 时输出
[event] handshake ok bus_version=<version>
[event] ready event_key=<key>                               # 就绪标记，--quiet 也不屏蔽
[event] received trace_id=<id>                              # 每条事件一行
[source] <source>: <state> (<detail>)                       # 上游 source 状态变化
[event] WARN dropped <N> event(s) for key=<key> since unix=<ts>
[event] WARN subscribe failed key=<key> code=<code> (<detail>)
[event] exited — received <N> event(s) in <duration> (reason: <reason>)
```

**示例：**

```bash
# 长时间订阅（Ctrl-C 退出）
tmeet event consume --event-id meeting.started

# 仅消费 3 条事件后退出
tmeet event consume --event-id meeting.started --max-events 3

# 30 秒内若没事件也退出
tmeet event consume --event-id meeting.end --timeout 30s

# 用 --param 缩小订阅范围（具体可用 key 见 event schema）
tmeet event consume --event-id meeting.started --param meeting_id=6953553464429888300

# 用 jq 投影只输出 meeting_id 和 subject
# 注意：meeting.started / meeting.end 的 jq_root_path 是 .payload，
# 即 jq 的输入根 . 已经是 payload 数组本身（服务端契约保证长度恒为 1），
# 需用 .[0] 取首元素后再下钻字段。
tmeet event consume --event-id meeting.started \
  --jq '.[0].meeting_info | {meeting_id, subject}'

# 把全量事件落盘做审计，同时静默信息型 stderr
tmeet event consume --event-id meeting.started \
  --output-dir ./meeting_events \
  --quiet
```

> ⚠️ `event consume` 要求已登录（用 OpenID 计算 owner_hash 与 bus 绑定）。未登录请先执行 `tmeet auth login`。

---

#### `event status` — 查看本机 bus 守护进程状态

报告本机 bus 守护进程的状态。输出 schema 始终包含一个长度为 0 或 1 的 `buses` 数组（tmeet 每台主机最多一个 bus 实例）。

> 本命令仅读取本机 bus 目录与 IPC，**不依赖登录**；用于在 `auth logout` 后排查残留状态。

```bash
tmeet event status [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--fail-on-orphan` | bool | — | `false` | 当存在 `orphan` 或 `stale_owner` 状态的 bus 时，以退出码 `2` 返回（默认 0），方便健康检查脚本分支处理 |

**`buses[].state` 取值：**

| 状态 | 含义 | 建议操作 |
|------|------|--------|
| `running` | bus 存活且绑定到当前登录用户 | 无需处理 |
| `stale_owner` | bus 存活但绑定到其他用户，或本机未登录 | 与原用户确认后执行 `tmeet event stop --force`，或重新以原账户登录 |
| `orphan` | bus 已退出但残留了 `bus.pid` / `bus.meta` 等磁盘文件 | 执行 `tmeet event stop --force` 清理残留 |

**输出 `buses[]` 主要字段：**

| 字段 | 说明 |
|------|------|
| `state` | 状态枚举，见上表 |
| `openid_hash` | bus 绑定的 OpenID 哈希 |
| `is_active_login` | 该 bus 的 owner 是否为当前登录用户 |
| `pid` | bus 进程 PID |
| `started_at` | bus 启动时间（本地时区 RFC3339） |
| `sock` | bus 监听的 unix socket 路径 |
| `consumer_count` | 当前挂接的消费者数量（仅 `running` 时有意义） |
| `subscribed_keys` | 当前订阅的 EventKey 列表 |
| `wss.state` | 底层 WSS 链路状态（`connecting` / `steady` / `reconnecting` / `auth_failed` / `auth_expired` / `disconnected`） |
| `wss.connected_at` | WSS 建链时间（本地时区 RFC3339） |
| `wss.reconnect_count` | WSS 累计重连次数 |
| `hint` | 异常状态下的处理建议 |

**示例：**

```bash
# 普通查询
tmeet event status --format json-pretty

# 健康检查脚本：发现 orphan / stale_owner 时退出码 2
tmeet event status --fail-on-orphan
```

---

#### `event stop` — 停止本机 bus 守护进程

请求 bus 守护进程退出。默认走优雅关闭，必要时通过 `--force` 强制清理。

> 与 `event status` 一致，**不依赖登录**；常用于 `auth logout` 之后清理残留状态。

```bash
tmeet event stop [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--force` | bool | — | `false` | 跳过"还有活跃消费者"的拒绝保护；强制清理 `orphan` / `stale_owner` 状态；并在磁盘上清除残留的 `bus.pid` / `bus.meta` / `bus.sock` |
| `--timeout` | duration | — | `10s` | 等待 bus 优雅退出的最长时间，超时后若加 `--force` 会自动转入清理 |

**`results[].state` 取值与退出码：**

| 状态 | 含义 | 退出码 |
|------|------|:------:|
| `stopped` | bus 已退出（优雅 / 强制清理均归此类） | `0` |
| `no_bus` | 磁盘与运行时均无 bus，相当于 no-op | `0` |
| `refused` | 有活跃消费者 / 检测到 `stale_owner` / `orphan` 且未加 `--force` | `2` |
| `errored` | 优雅关闭超时且未加 `--force`，或强制清理失败 | `2` |

**输出 `results[]` 主要字段：**

| 字段 | 说明 |
|------|------|
| `state` | 见上表 |
| `openid_hash` | 被操作 bus 的 owner 哈希 |
| `pid` | bus 进程 PID |
| `consumers_evicted` | 退出时被驱逐的消费者数量（`stopped` 时有意义） |
| `consumer_count` | 拒绝时的活跃消费者数量（`refused` 时有意义） |
| `forced` | 是否走了 `--force` 分支 |
| `socket_cleaned` | 是否清理了 `bus.sock` |
| `elapsed_ms` | 优雅关闭耗时（毫秒） |
| `hint` | 建议的后续操作 |

**示例：**

```bash
# 优雅关闭：有活跃消费者时会拒绝（退出码 2）
tmeet event stop

# 强制关闭：驱逐活跃消费者；或清理 orphan / stale_owner
tmeet event stop --force

# 自定义优雅等待时长
tmeet event stop --timeout 5s
```

---

### agent — 子账号管理

子账号（agent）相关命令，用于在主账号下创建/删除一个独立身份的子账号，管理子账号的访问凭证。子账号的访问凭证与主账号分开管理。

> **前置条件**：执行任意 `agent` 命令前必须先完成 `tmeet auth login`。

#### `agent create` — 创建子账号

在当前已登录主账号下创建一个子账号（agent），并把子账号凭证（`access_token` / `refresh_token` 及过期时间）加密保存到本地。同一主账号下若已存在子账号，则会拒绝重复创建并展示已有子账号信息。

```bash
tmeet agent create
```

> 无参数。

创建成功后输出 `AgentId` 和 `CreateTime`；若已存在子账号，会提示 `agent already exists` 并展示当前 `AgentId` / `CreateTime`，需先 `agent delete` 后才能重新创建。

---

#### `agent delete` — 删除子账号

删除当前主账号下的子账号，并清除本地保存的子账号凭证。

```bash
tmeet agent delete --agent-id <AgentID>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--agent-id` | string | ✅ | — | 要删除的 Agent ID |
| `--force` | bool | — | `false` | 跳过二次确认，直接执行删除 |

未指定 `--force` 时，命令会先展示待删除的 agent 信息，并提示用户输入 `yes/y` 确认后才执行删除。执行前会读取本地子账号配置：未创建过子账号或 `--agent-id` 与本地配置不匹配时返回 `agent not found`。删除成功后输出被删除的 `AgentId` 和原 `CreateTime`。

---

#### `agent token` — 刷新子账号 Token

为当前主账号下的子账号重新签发一对 `access_token` / `refresh_token`，并覆盖本地已保存的凭证。常用于本地凭证损坏、被清空，或主动轮换 Token 的场景。

```bash
tmeet agent token
```

> 无参数。

命令会先检查本地 `refresh_token` 是否仍在有效期内：若未过期，则直接提示"refresh_token 未过期，无需刷新"并展示剩余有效期，不会调用服务端接口；若已过期，则向服务端签发新的 token 对。刷新成功后输出 `AgentId` 和 `RefreshTokenExpires`（ISO 8601 带时区格式）。本地未创建子账号时返回 `agent not found`。

---

#### `agent list` — 列出子账号

列出当前主账号下的子账号列表（读取本地配置）。

```bash
tmeet agent list
```

> 无参数。

若本地存在子账号，则以带序号的列表形式展示每个 agent 的 `AgentId`、`CreateTime`、`AccessTokenExpires`、`RefreshTokenExpires`（均为 ISO 8601 带时区格式）；若本地无子账号，则提示"当前主账号下没有子账号（agent），请通过 `tmeet agent create` 创建"。

---

#### `agent get` — 查询单个子账号详情

查询指定 Agent ID 的子账号详情（读取本地配置）。

```bash
tmeet agent get --agent-id <AgentID>
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--agent-id` | string | ✅ | — | Agent ID |

匹配成功时展示该 agent 的 `AgentId`、`CreateTime`、`AccessTokenExpires`、`RefreshTokenExpires`（均为 ISO 8601 带时区格式）；若 `--agent-id` 与本地配置不匹配或本地无子账号，返回错误 `agent not found`。

---

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