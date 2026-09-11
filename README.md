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
| `minutes search` | 20  | 50 | — |
| `minutes get` | 10  | 30 | — |

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
│   └── invitees-replace # 替换会议受邀者列表
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
├── minutes
│   ├── search         # 按关键词/时间搜索元宝纪要
│   └── get            # 查询元宝纪要详情
├── tshoot
│   ├── log               # 导出本地日志（支持按时间范围过滤，可选 --upload 上传至服务器）
│   └── feedback          # 上报问题排查反馈到服务器
├── app
│   ├── get            # 获取当前 CLI 应用信息
│   └── set            # 设置当前 CLI 应用信息
└── event
    ├── list           # 列出可订阅的 EventKey
    ├── schema         # 查看 EventKey 的参数 / 输出 schema
    ├── consume        # 订阅 EventKey，按 NDJSON 流式输出事件
    ├── status         # 查看本机 bus 守护进程状态
    └── stop           # 停止本机 bus 守护进程（可选 --force 清理残留）
```

---

## 命令参考

各子命令的完整参数说明、示例与响应字段请参阅：👉 [docs/command.md](docs/command.md)

---

## 安全与风险提示（使用前必读）

---
**腾讯会议 CLI 工具接入 OpenClaw 等AI Agent 并获得您的授权后，AI 将会获得你在腾讯会议的数据访问权限（包括但不限于您的详细用户信息、管理和查询会议、录制和纪要等文件查询导出），并以您的用户身份在授权范围内执行操作。尽管工具有安全防护，AI仍可能因模型幻觉、提示词注入、投毒攻击、执行偏差不可控等原因，导致数据泄露、越权操作等执行非预期操作的高风险后果，请您谨慎操作和使用，并遵循你所在企业的数据安全等内部管理要求，避免造成数据丢失、泄露等损失。若怀疑泄露或需停用，请立即执行登出命令 `tmeet auth logout`。**

**请您充分理解并接受上述风险后再使用本工具，安装使用CLI后即视为您自愿承担相关责任。**

---

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
