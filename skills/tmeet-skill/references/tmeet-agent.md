# tmeet agent — 子账号管理

> **前置条件：**
> - 所有 `agent` 命令都必须先执行 `tmeet auth login` 完成主账号登录授权。
> - 订阅 `subscribe_role=agent` 的事件（如 `event consume meeting.asr-push`）同样要求本地已通过 `agent create` 创建过子账号。

## 目录

- [概念与用途](#概念与用途)
- [create — 创建子账号](#create--创建子账号)
- [delete — 删除子账号](#delete--删除子账号)
- [token — 刷新子账号 Token](#token--刷新子账号-token)
- [list — 列出子账号](#list--列出子账号)
- [get — 查询单个子账号详情](#get--查询单个子账号详情)
- [常见错误](#常见错误)
- [参考](#参考)

## 概念与用途

子账号（agent）是在主账号之下、由 CLI 独立持有 `access_token` / `refresh_token` 的一类受限身份。它有两类用途：

1. **生命周期管理**（`create` / `delete` / `token` / `list` / `get`）：用主账号身份在服务端开通 / 注销一个子账号，并把子账号凭证加密保存到本地；`list` / `get` 为**只读**操作，仅读取本地已保存的子账号信息，不产生远端调用。
2. **订阅子账号类实时事件**（`event consume`）：部分 EventKey 的 `subscribe_role` 为 `agent`（如实时转写推送 `meeting.asr-push`），**只能由子账号订阅**。订阅前本地必须已通过 `agent create` 创建子账号，否则订阅失败。

**当前限制**：同一主账号下**只能存在一个**子账号。`create` 在已有子账号时会直接报错，需先 `delete` 才能重建。

> ⚠️ `agent create` / `agent delete` / `agent token` 均为写操作，**执行前必须按 [SKILL.md 「安全规则」](../SKILL.md#安全规则)中「以下命令操作必须二次确认」的表格与确认流程向用户展示并获得明确确认**；`agent delete` 另有非交互环境下必须带 `--force` 的硬要求，同见该表。

---

## create — 创建子账号

> ⚠️ **写操作（在主账号下开通子账号并把凭证落盘到本机）：执行前必须按 [SKILL.md 「安全规则」](../SKILL.md#安全规则)中「以下命令操作必须二次确认」向用户展示并获得明确确认。**

在当前已登录的主账号下创建一个子账号。同一主账号下若已存在子账号，命令会拒绝重复创建并展示已有子账号信息（必须先 `agent delete` 之后才能再创建）。

```bash
tmeet agent create
```

### 参数

无参数。

### 输出

以纯文本方式打印多行 `key: value`（**非 JSON**，不走 `{trace_id, message, data}` 信封，也不受 `--format` / `--compact` 影响）。

成功创建：

```
Agent created successfully
  AgentId:    <agent open_id>
  CreateTime: <ISO 8601 时间>
```

子账号已存在：以**非零退出码**返回错误 `agent already exists`，并打印已有子账号的 `AgentId` 与 `CreateTime`。此时**不要**自行改调 `agent delete` 重建——须先向用户说明"已存在子账号"，询问是复用现有子账号还是删除后重建。

---

## delete — 删除子账号

> ⚠️ **高风险不可逆写操作（服务端删除子账号 + 清除本地凭证，该子账号一切请求立即失效，正在进行的 `subscribe_role=agent` 订阅会中断）：执行前必须按 [SKILL.md 「安全规则」](../SKILL.md#安全规则)中「以下命令操作必须二次确认」向用户展示 `AgentId` / `CreateTime` 并获得明确确认；非交互环境下必须带 `--force` 才会真正执行，且严禁未获确认就带 `--force`。**

> ⚠️ **判定成功必须看输出内容，不能只看退出码**：只有出现 `Agent deleted successfully` 才算删除成功；出现 `已取消删除操作` 表示**未删除**（退出码同样为 0）。严禁凭退出码 0 就向用户报告"已删除"。

删除当前主账号下的子账号，并清空本地保存的子账号凭证。

```bash
# Agent 场景：已获得用户明确确认后，带 --force 执行
tmeet agent delete --agent-id <AgentID> --force
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--agent-id <id>` | ✅ | — | 要删除的 Agent ID；必须与本地保存的一致，可用 `agent list` 查看 |
| `--force` | — | `false` | 跳过 CLI 自己的 stdin 二次确认。**在非交互环境下必须显式传入**，否则命令会静默取消；传入前必须已获得用户确认 |

### 输出

成功删除：

```
Agent deleted successfully
  AgentId:    <agent open_id>
  CreateTime: <ISO 8601 时间>
```

未确认（非交互环境下不带 `--force` 的必然结果）：

```
已取消删除操作
```

> 该输出的**退出码为 0**，但**删除未发生**。若已获得用户确认，补上 `--force` 重新执行。

本地未创建过子账号，或 `--agent-id` 与本地配置不匹配：返回错误 `agent not found`，**无任何远端调用**（不会误删他人子账号）。

---

## token — 刷新子账号 Token

> ⚠️ **写操作（重新签发子账号凭证并覆盖本地，旧 `access_token` 立即失效）：执行前必须按 [SKILL.md 「安全规则」](../SKILL.md#安全规则)中「以下命令操作必须二次确认」向用户说明该影响并获得明确确认。**

为已存在的子账号重新签发一对 `access_token` / `refresh_token`，并覆盖本地凭证。

**触发条件由 CLI 自行判断，不受调用方控制**：命令先检查本地 `refresh_token` 是否仍在有效期内 ——

- **未过期** → 直接提示"refresh_token 未过期，无需刷新"并展示剩余有效期，**不调用服务端接口、不改动任何凭证**，退出码 0；
- **已过期** → 才向服务端签发新的 token 对并覆盖本地凭证。

> **因此本命令无法用于"主动轮换尚未过期的 Token"** —— 该场景下命令是空操作。若用户诉求是强制轮换，如实告知这一限制；若确需更换凭证，只能走 `agent delete` + `agent create`（`delete` 属不可逆操作，须按 [SKILL.md 「安全规则」](../SKILL.md#安全规则)二次确认）。
>
> 适用场景：子账号 `refresh_token` 已过期导致订阅或调用失败后的恢复。

```bash
tmeet agent token
```

### 参数

无参数。**注意**：本命令不接受 `--agent-id`，始终作用于本地已保存的那个子账号。

### 输出

refresh_token 未过期时（**空操作**）：

```
Agent 的 refresh_token 未过期，无需刷新
  AgentId:             <agent open_id>
  RefreshTokenExpires: <ISO 8601 时间>
  剩余有效期:          29天23小时58分钟
```

refresh_token 已过期，刷新成功：

```
Agent token refreshed successfully
  AgentId:             <agent open_id>
  RefreshTokenExpires: <ISO 8601 时间>
```

本地未创建过子账号：返回错误 `agent not found`。

> **禁止输出凭证**：本命令签发的 `access_token` / `refresh_token` 不会被打印，Agent 也**严禁**从本地配置读取或向用户回显任何 token 明文（见 [SKILL.md «安全规则»](../SKILL.md)）。

---

## list — 列出子账号

**只读操作**，读取本地配置展示，不产生远端调用。当前仅支持一个子账号，后续可能扩展为多个。

```bash
tmeet agent list
```

### 参数

无参数。

### 输出

存在子账号时，以带序号的列表形式展示：

```
当前主账号下的 agent 列表（共 1 个）：

  [1] AgentId: <agent open_id>
      CreateTime:          <ISO 8601 时间>
      AccessTokenExpires:  <ISO 8601 时间>
      RefreshTokenExpires: <ISO 8601 时间>
```

本地无子账号时：

```
当前主账号下没有子账号（agent），请通过 `tmeet agent create` 创建
```

> 该提示**不代表**应当立即执行 `agent create` —— 创建属写操作，须先征得用户确认。

---

## get — 查询单个子账号详情

**只读操作**，读取本地配置展示，不产生远端调用。

```bash
tmeet agent get --agent-id <AgentID>
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--agent-id <id>` | ✅ | — | Agent ID |

### 输出

匹配成功时：

```
Agent 详情：

  AgentId: <agent open_id>
  CreateTime:          <ISO 8601 时间>
  AccessTokenExpires:  <ISO 8601 时间>
  RefreshTokenExpires: <ISO 8601 时间>
```

本地未创建过子账号或 `--agent-id` 与本地配置不匹配：返回错误 `agent not found`。

---

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `agent already exists` | 当前主账号下已存在子账号（仅支持一个） | 向用户说明并询问：复用现有子账号，或确认后 `agent delete` 再 `agent create` |
| `agent not found` | 本地未创建过子账号，或 `--agent-id` 与本地配置不匹配 | 先用 `agent list` 核对 AgentId；确实没有则征得确认后 `agent create` |
| `--agent-id is required` | `agent delete` / `agent get` 缺少必填参数 | 补充 `--agent-id`，可通过 `agent list` 查看当前 AgentId |
| 输出 `已取消删除操作`，退出码 0 | 非交互环境下未带 `--force`，CLI 读 stdin 得到 EOF 判定为未确认 | **删除未发生**。先向用户确认，再带 `--force` 重试 |
| `Agent 的 refresh_token 未过期，无需刷新` | refresh_token 仍在有效期内，命令为空操作 | 无需处理，token 仍然有效；该命令无法强制轮换未过期的 Token |
| `event consume` 订阅 `subscribe_role=agent` 事件失败 | 本地未创建子账号，或子账号 refresh_token 已过期 | `agent list` 核对状态；无子账号则确认后 `create`，已过期则 `agent token` |
| `user config is empty` | 主账号未登录 | 先执行 `tmeet auth login` |

## 参考

- [tmeet](../SKILL.md) — 全部命令概览与安全规则
- [tmeet-auth](tmeet-auth.md) — 主账号登录授权（所有 `agent` 命令的前置条件）
- [tmeet-event](tmeet-event.md) — 实时事件订阅（`subscribe_role=agent` 的事件需先创建子账号）
- [tmeet-meeting](tmeet-meeting.md) — 会议管理（`join-as-agent` / `leave-as-agent` 以子账号身份入会）
