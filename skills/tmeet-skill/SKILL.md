---
name: tmeet-skill
version: 1.0.0
description: "腾讯会议 CLI（tmeet）：OAuth 授权登录/登出/状态查询、会议管理（创建/更新/取消/查询/受邀者）、录制管理（列表/下载地址/智能纪要/转写）、会议报告（参会人/等候室）。当用户需要通过命令行操作腾讯会议时使用本技能。"
metadata:
  requires:
    bins: ["tmeet"]
  cliHelp: "tmeet --help"
---

# tmeet

腾讯会议命令行工具，支持 OAuth 授权、会议全生命周期管理、录制与转写、会议报告查询。

## 安装与初始化

在使用本技能前，系统会自动检测是否已安装 `tmeet` 命令行工具：

- 如果检测到 `tmeet` 命令已存在，直接使用即可
- 如果未检测到 `tmeet` 命令，会自动执行以下安装命令：
  ```bash
  npm install -g @tencentcloud/tmeet
  ```
  安装最新版本的腾讯会议 CLI 工具包

> **注意**：自动安装需要网络连接和 npm 环境支持。如果安装失败，请手动执行上述命令或检查网络环境。

## 核心概念

- **会议（Meeting）**：腾讯会议实例，通过 `meeting_id` 或 `meeting_code` 标识。`meeting_id` 仅用于命令行参数传递，**向用户展示会议信息时必须使用 `meeting_code`（会议号），不得将 `meeting_id` 暴露给用户**。
- **周期性会议（Recurring Meeting）**：`meeting_type=1` 的重复会议，包含多个子会议（`sub_meeting_id`）。
- **录制（Record）**：会议结束后生成的录制文件，通过 `meeting_record_id` 和 `record_file_id` 标识。
- **智能纪要（Smart Minutes）**：基于录制文件生成的 AI 纪要。
- **转写（Transcript）**：录制文件的逐字转写内容，支持段落查询和关键词搜索。
- **报告（Report）**：会议结束后的统计数据，包含参会人列表和等候室成员。

## 认证

使用前必须先完成登录授权：

```bash
# 登录（必须后台运行，以便立即捕获授权 URL）
tmeet auth login 2>&1 &

# 登出（清除本地凭证）
tmeet auth logout

# 查看当前登录状态及 Token 有效期（无需登录即可执行）
tmeet auth status
```

> **注意**：除 `auth login`,`auth status` 外，所有命令都需要先完成登录。未登录时命令会提示 `user config is empty`。

> **注意**：`auth login` 是阻塞命令，**必须以后台方式运行**（命令末尾加 `2>&1 &`），否则命令会一直阻塞等待，模型无法捕获输出中的授权 URL。执行后从输出中提取授权 URL，**必须将该链接完整展示给用户**，并明确提示用户在浏览器中打开完成授权，不得省略或仅提示"已生成链接"等模糊描述。

> **注意**：第一次调用`auth login`命令或者切换了模型，必须将当前使用的agent类型(如Cursor/Claude Desktop/Cline/CodeBuddy)和模型名称(如Claude 3.5 Sonnet/GPT-4o/DeepSeek)分别写入`TMEET_AGENT`和`TMEER_MODEL`环境变量，**不需要向用户询问**。

## 时间格式

所有时间参数均使用 **ISO 8601** 格式，支持以下两种：

| 格式 | 示例 |
|------|------|
| 带时区（有秒） | `2026-03-12T14:00:00+08:00` |
| 带时区（无秒） | `2026-03-12T14:00+08:00` |

> **注意**：不支持仅日期格式（如 `2026-03-12`），必须包含时间和时区信息。

> **时间逻辑校验**：若用户提供的结束时间 ≤ 开始时间（如"4点到3点"），**不得自行推断用户意图**，必须先向用户确认是否跨天或存在笔误，再执行命令。

## 命令总览

```
tmeet
├── auth
│   ├── login          # OAuth 授权登录
│   ├── logout         # 登出并清除凭证
│   └── status         # 查看当前登录状态及 Token 有效期
├── meeting
│   ├── create         # 创建会议（支持普通/周期性）
│   ├── update         # 更新会议信息
│   ├── cancel         # 取消会议
│   ├── get            # 获取会议详情
│   ├── list           # 获取会议列表（进行中/未开始）
│   ├── list-ended     # 获取已结束会议列表
│   └── invitees-list  # 获取会议受邀者列表
├── record
│   ├── list                  # 查询录制列表
│   ├── address               # 获取录制文件下载地址
│   ├── smart-minutes         # 获取智能纪要
│   ├── transcript-get        # 获取转写详情（分页）
│   ├── transcript-paragraphs # 获取转写段落列表
│   └── transcript-search     # 搜索转写内容
└── report
    ├── participants      # 获取参会人列表
    └── waiting-room-log  # 获取等候室成员列表
```

## 子命令详情

- 认证：[`references/tmeet-auth.md`](references/tmeet-auth.md)
- 会议管理：[`references/tmeet-meeting.md`](references/tmeet-meeting.md)
- 录制管理：[`references/tmeet-record.md`](references/tmeet-record.md)
- 会议报告：[`references/tmeet-report.md`](references/tmeet-report.md)

## 安全规则

- **禁止输出 AccessToken / RefreshToken** 到终端明文。

- **写操作必须二次确认，严禁直接执行**：以下命令会对数据产生不可逆或高风险影响，**在调用命令前必须先向用户展示将要执行的操作详情，并明确获得用户确认后才能执行**，不得跳过确认步骤：

  | 命令 | 风险说明 |
  |------|---------|
  | `meeting cancel` | 取消会议，不可恢复 |
  | `meeting update` | 修改会议信息（时间、主题等），影响所有参会人 |
  | `auth logout` | 清除本地登录凭证 |

  **确认流程**：
  1. 向用户展示即将执行的操作及关键信息（使用 `meeting_code` 会议号标识会议，不得展示 `meeting_id`）；
  2. 等待用户明确回复"确认"、"是"、"yes"等肯定指令；
  3. 收到确认后再执行命令；
  4. 若用户未明确确认或表示取消，则终止操作。

- **必填参数缺失时，必须向用户确认补充，禁止自行填充**：若执行命令所需的必填参数未由用户提供，**不得自行推断或填充默认值**，必须明确告知用户缺少哪些参数并请求补充，待用户提供后再执行命令。

## 响应处理规则

- **只展示关键信息**：在用户没有明确要求的前提下，仅展示与用户问题直接相关的核心字段，不得输出冗余字段。
- **禁止擅自聚合或排序**：未经用户要求，不得对返回结果进行任何聚合统计或排序操作，按原始结果如实呈现。

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `user config is empty` | 未登录 | 执行 `tmeet auth login` |
| `--start format error` | 时间格式不合法（如缺少时区） | 改用 `2026-03-12T14:00:00+08:00` 格式 |
| `--meeting-id is required` | 缺少必填参数 | 补充对应必填参数 |
| `user has been initialized` | 已登录，重复执行 login | 直接使用，或先 logout 再 login |