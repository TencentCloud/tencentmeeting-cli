---
name: tmeet-skill
version: 1.0.3
description: "腾讯会议 CLI（tmeet）：OAuth 授权登录/登出/状态查询、会议管理（创建/更新/取消/查询/受邀者）、录制管理（列表/下载地址/智能纪要/转写/录制权限申请）、会议报告（参会人/等候室）、问题排查（导出本地日志，反馈工具缺失/失败/能力不足等问题给平台）。当用户需要通过命令行操作腾讯会议，或 Agent 在使用过程中遇到工具缺失、调用失败、能力不足等情况想反馈给平台时使用本技能。"
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
  npm install -g @tencentcloud/tmeet@latest
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
 
> **注意**：如果当前 Agent 是 **Hermes** 且系统**没有默认浏览器**，请**不要**尝试执行 `tmeet auth login` 命令，而是直接告知用户：「请在终端中手动执行 `tmeet auth login` 完成登录授权，获取授权 URL 后在浏览器中打开完成授权。」

> **注意**：第一次调用`auth login`命令或者切换了模型，必须将当前使用的agent类型(如Cursor/Claude Desktop/Cline/CodeBuddy)和模型名称(如Claude 3.5 Sonnet/GPT-4o/DeepSeek)分别写入`TMEET_AGENT`和`TMEET_MODEL`环境变量，**不需要向用户询问**。

## 时间格式

所有时间参数均使用 **ISO 8601** 格式，支持以下两种：

| 格式 | 示例 |
|------|------|
| 带时区（有秒） | `2026-03-12T14:00:00+08:00` |
| 带时区（无秒） | `2026-03-12T14:00+08:00` |

> **注意**：不支持仅日期格式（如 `2026-03-12`），必须包含时间和时区信息。

> **时间逻辑校验**：若用户提供的结束时间 ≤ 开始时间（如"4点到3点"），**不得自行推断用户意图**，必须先向用户确认是否跨天或存在笔误，再执行命令。

## 全局参数

所有子命令均继承以下全局参数。

### `--format`：输出 JSON 形态

用于控制输出 JSON 的排版形态，**不改变字段内容**。输出结构统一为 `{trace_id, message, data}`。

| 取值 | 含义 | 适用场景 |
|------|------|---------|
| `json`（默认） | 单行紧凑 JSON，体积小、便于管道传递 | 模型解析、脚本处理、`jq` 过滤 |
| `json-pretty` | 多行缩进 JSON，可读性强 | 需要将原始结果直接呈现给用户阅读时 |

**使用示例**：

```bash
# 默认紧凑格式（模型解析场景推荐，省略 --format 即可）
tmeet meeting get --meeting-id 123456789

# 美化缩进格式（需要直接展示给用户阅读时使用）
tmeet meeting list --start 2026-03-12T00:00:00+08:00 --end 2026-03-12T23:59:59+08:00 --format json-pretty
```

> **使用准则**：
> - 模型在解析工具输出时**优先使用默认 `json`**，无需显式传入 `--format`；
> - 仅当用户明确要求"以美化/格式化 JSON 展示"或需要把原始 JSON 完整呈现给用户时，才追加 `--format json-pretty`；
> - 即便使用 `json-pretty`，响应处理规则仍然适用——**只展示关键信息，不得擅自聚合或排序**。

### `--compact`：精简响应字段

布尔开关（默认 `false`），用于**裁剪响应体 `data` 中的字段**，只保留该命令业务上必要的少量字段，从而显著降低输出 token 量。

- 启用后，中间件会根据当前命令的 API 注解从远端拉取"精简字段列表"（compact fields），并对响应 `data` 按该列表进行字段保留；`trace_id`、`message` 等顶层字段不受影响。
- 若当前命令未声明 API 注解、或远端拉取失败，中间件会**透明放行**，不会阻塞主流程，此时输出等同于未开启 `--compact` 的结果。
- 与 `--format` 相互独立：`--format` 决定 JSON 排版，`--compact` 决定返回字段的数量，两者可同时使用。

**使用示例**：

```bash
# 仅返回必要字段（推荐模型解析场景使用，节省 token）
tmeet meeting list --start 2026-03-12T00:00:00+08:00 --end 2026-03-12T23:59:59+08:00 --compact

# 同时启用精简字段 + 美化排版（便于用户直接阅读关键信息）
tmeet record list --meeting-id 123456789 --compact --format json-pretty
```

> **使用准则**：
> - **查询类命令优先启用**：模型在调用查询/读取类命令时，**默认追加 `--compact`** 以降低上下文占用；
> - **何时不使用**：当用户明确要求"完整结果"、"原始字段"或需要某个非必要字段时，**不要**使用 `--compact`。

## 分页

所有支持分页的查询/列表类命令统一采用 **`--page-token` + `--page-size`** 方案；原有的 `--page` / `--pos` / `--pid` / `--size` / `--limit` 参数均已标记为 **已弃用**，仅为兼容保留，**模型不得主动使用**。

| 参数 | 说明 |
|------|------|
| `--page-token <token>` | 分页游标。**首次查询不传**；翻页时将上一次响应 `data.next_page_token` 的值原样传入 |
| `--page-size <n>` | 每页数量，不同命令默认值与上限不同，详见各子命令文档 |

**使用准则**：

- **优先使用 `--page-token` 翻页**：调用下一页时，必须从上一次响应的 `data.next_page_token` 字段取值传入 `--page-token`，不得自行拼接、递增或猜测该值。
- **到达末页的判定**：当响应中的 `next_page_token` 为空字符串或字段缺失时，即为最后一页，不再继续翻页。
- **禁止使用已弃用参数**：即便用户对话中使用了"第 X 页"、"偏移 Y 条"等表达，也应以 `--page-token` 分页策略实现（首次查询 → 读取 `next_page_token` → 继续翻页），**不得**使用 `--page` / `--pos` / `--pid` / `--size` / `--limit`。
- **`record transcript-search` 暂不支持分页**，无需传入分页参数。

**典型翻页流程**：

```bash
# 1) 首次查询（不传 --page-token）
tmeet record list --meeting-id "100000000" --page-size 30 --compact

# 2) 从响应中取出 data.next_page_token，继续翻页
tmeet record list \
  --meeting-id "100000000" \
  --page-token "<next_page_token>" \
  --page-size 30 --compact
```

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
│   ├── list                     # 查询录制列表
│   ├── address                  # 获取录制文件下载地址
│   ├── smart-minutes            # 获取智能纪要
│   ├── transcript-get           # 获取转写详情
│   ├── transcript-paragraphs    # 获取转写段落列表
│   ├── transcript-search        # 搜索转写内容
│   ├── permission-apply-prepare # 预览录制权限申请信息（申请前确认）
│   └── permission-apply-commit  # 提交录制权限申请（用户确认后执行）
├── report
│   ├── participants      # 获取参会人列表
│   └── waiting-room-log  # 获取等候室成员列表
└── tshoot
    ├── log               # 导出本地日志（支持按时间范围过滤，可选 --upload 上传至服务器）
    └── feedback          # 反馈工具缺失/失败/能力不足等问题至平台（Agent 自助上报）
```

## 子命令详情

- 认证：[`references/tmeet-auth.md`](references/tmeet-auth.md)
- 会议管理：[`references/tmeet-meeting.md`](references/tmeet-meeting.md)
- 录制管理：[`references/tmeet-record.md`](references/tmeet-record.md)
- 会议报告：[`references/tmeet-report.md`](references/tmeet-report.md)
- 问题排查：[`references/tmeet-tshoot.md`](references/tmeet-tshoot.md)

## 安全规则

- **禁止输出 AccessToken / RefreshToken** 到终端明文。

- **写操作必须二次确认，严禁直接执行**：以下命令会对数据产生不可逆或高风险影响，**在调用命令前必须先向用户展示将要执行的操作详情，并明确获得用户确认后才能执行**，不得跳过确认步骤：

  | 命令 | 风险说明 |
  |------|---------|
  | `meeting cancel` | 取消会议，不可恢复 |
  | `meeting update` | 修改会议信息（时间、主题等），影响所有参会人 |
  | `auth logout` | 清除本地登录凭证 |
  | `record permission-apply-commit` | 正式提交录制权限申请，会触发审批流程（必须先执行 `record permission-apply-prepare` 并向用户展示申请信息确认）|

  **确认流程**：
  1. 向用户展示即将执行的操作及关键信息（使用 `meeting_code` 会议号标识会议，不得展示 `meeting_id`）；
  2. 等待用户明确回复"确认"、"是"、"yes"等肯定指令；
  3. 收到确认后再执行命令；
  4. 若用户未明确确认或表示取消，则终止操作。

- **必填参数缺失时，必须向用户确认补充，禁止自行填充**：若执行命令所需的必填参数未由用户提供，**不得自行推断或填充默认值**，必须明确告知用户缺少哪些参数并请求补充，待用户提供后再执行命令。

## 响应处理规则

- **只展示关键信息**：在用户没有明确要求的前提下，仅展示与用户问题直接相关的核心字段，不得输出冗余字段。
- **禁止擅自聚合或排序**：未经用户要求，不得对返回结果进行任何聚合统计或排序操作，按原始结果如实呈现。

## 自动反馈规则

当 Agent 在使用 tmeet CLI 过程中遇到下列任一情况时，应识别并准备调用 `tmeet tshoot feedback` 将问题上报至平台，**但在实际执行上报前必须先向用户二次确认，获得明确同意后再执行**；详细参数与示例见 [`references/tmeet-tshoot.md`](references/tmeet-tshoot.md)。

### 触发条件（满足任一即上报）

| `--category` | 触发场景 |
|------|---------|
| `tool_not_found` | 用户想做某事，但 tmeet 当前**没有匹配的命令/子命令**可完成 |
| `tool_error` | 调用某个 tmeet 命令**返回了错误**（业务错误码 / 参数错误 / 执行异常） |
| `tool_inadequate` | 命令存在，但其**参数或能力无法满足**用户当前诉求 |
| `unexpected_result` | 命令调用成功，但**返回结果与用户/Agent 期望明显不一致** |
| `suggestion` | 在交互中识别到**通用性改进建议或新增能力提议** |

### 调用准则

- **必须二次确认后再上报**：识别到上述触发条件后，**先向用户展示将要反馈的内容**（包括 `--category`、`--intent`、`--actions-tried`、`--result` 等关键字段），并明确询问用户是否同意上报；**仅在收到用户明确确认（如"确认"、"是"、"yes"等肯定指令）后**才执行 `tmeet tshoot feedback`；若用户拒绝或未明确同意，则不得上报。上报完成后**简要告知用户**「已为您将该问题反馈至平台」。
- **不替代正常错误处理**：反馈仅用于告知平台，**不得用于绕过用户原始任务**。如仍有可执行的替代方案（如换一个命令、补充参数重试），应**先尝试解决**，无法解决再征询用户是否上报。
- **如实填写上下文**：`--intent` 必须如实写明用户的原始意图；`--actions-tried` 写明已尝试的命令；`--result` 写明阻塞点或错误信息；涉及具体命令时填入 `--tool-name`；有错误码时填入 `--error-code`。**严禁编造或填充无关内容**。
- **隐私脱敏强约束**：反馈内容中，**严禁透露用户姓名 / 电话 / 会议号 / 会议链接 / 会议主题 / 参会人**等涉及用户个人隐私的信息。如果必须引用相关内容辅助说明问题，**必须先进行打星号、加密等脱敏处理**（例如：姓名 `张三` → `张*`、手机号 `13800138000` → `138****8000`、会议号 `123456789` → `12****789`、会议主题 `Q2 项目复盘会` → `Q* 项目***会`）后再写入 `--intent` / `--actions-tried` / `--result` 等字段。
- **字符长度约束**：`--intent ≤ 200` / `--actions-tried ≤ 500` / `--result ≤ 500`，超长会被客户端直接拒绝，请精炼描述。
- **登录前置**：本命令依赖登录态，若用户尚未登录，先引导执行 `tmeet auth login`，登录成功后再发起反馈。
- **去重与节制**：同一用户会话中针对**同一问题**只上报**一次**，避免重复刷屏；不同问题分别独立上报。

### 典型示例

```bash
# 用户希望批量导出某月所有会议的智能纪要，但 tmeet 暂无批量命令
tmeet tshoot feedback \
  --category "tool_not_found" \
  --intent "批量导出 2026-04 整月所有会议的智能纪要" \
  --actions-tried "查阅 record list / record smart-minutes" \
  --result "未找到批量导出命令"
```

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `user config is empty` | 未登录 | 执行 `tmeet auth login` |
| `--start format error` | 时间格式不合法（如缺少时区） | 改用 `2026-03-12T14:00:00+08:00` 格式 |
| `--meeting-id is required` | 缺少必填参数 | 补充对应必填参数 |
| `user has been initialized` | 已登录，重复执行 login | 直接使用，或先 logout 再 login |