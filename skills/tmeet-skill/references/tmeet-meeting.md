# tmeet meeting — 会议管理

> **前置条件：** 先执行 `tmeet auth login` 完成登录授权。

时间参数格式：`2026-03-12T14:00:00+08:00` 或 `2026-03-12T14:00+08:00`（必须包含时区）。

---

## create — 创建会议

```bash
# 创建普通会议（必填：主题、开始时间、结束时间）
tmeet meeting create \
  --subject "项目周会" \
  --start "2026-04-10T14:00:00+08:00" \
  --end "2026-04-10T15:00:00+08:00"

# 创建带密码的会议
tmeet meeting create \
  --subject "季度复盘" \
  --start "2026-04-10T14:00:00+08:00" \
  --end "2026-04-10T16:00:00+08:00" \
  --password "123456"

# 创建仅受邀成员可入会的会议，并开启等候室
tmeet meeting create \
  --subject "保密会议" \
  --start "2026-04-10T10:00:00+08:00" \
  --end "2026-04-10T11:00:00+08:00" \
  --join-type 2 \
  --waiting-room

# 创建每周周期性会议（按次数结束，共 10 次）
tmeet meeting create \
  --subject "每周站会" \
  --start "2026-04-10T09:30:00+08:00" \
  --end "2026-04-10T10:00:00+08:00" \
  --meeting-type 1 \
  --recurring-type 2 \
  --until-type 1 \
  --until-count 10

# 创建每天周期性会议（按日期结束）
tmeet meeting create \
  --subject "每日站会" \
  --start "2026-04-10T09:00:00+08:00" \
  --end "2026-04-10T09:30:00+08:00" \
  --meeting-type 1 \
  --recurring-type 0 \
  --until-type 0 \
  --until-date "2026-05-10T00:00:00+08:00"
```

### 参数

| 参数 | 必填 | 默认值 | 说明                                               |
|------|------|--------|--------------------------------------------------|
| `--subject <text>` | ✅ | — | 会议主题                                             |
| `--start <time>` | ✅ | — | 开始时间（ISO 8601，含时区）                               |
| `--end <time>` | ✅ | — | 结束时间（ISO 8601，含时区）                               |
| `--password <pwd>` | 否 | — | 会议密码（4~6 位数字）                                    |
| `--timezone <tz>` | 否 | — | 时区，如 `Asia/Shanghai`                             |
| `--meeting-type <n>` | 否 | `0` | 会议类型：`0`-普通，`1`-周期性                              |
| `--join-type <n>` | 否 | `0` | 入会限制：`1`-所有成员，`2`-仅受邀，`3`-仅企业内部                  |
| `--waiting-room` | 否 | `false` | 开启等候室                                            |
| `--recurring-type <n>` | 周期性时使用 | `0` | 重复类型：`0`-每天，`1`-每周一至五，`2`-每周，`3`-每两周，`4`-每月      |
| `--until-type <n>` | 周期性时使用 | `0` | 结束类型：`0`-按日期，`1`-按次数                             |
| `--until-count <n>` | 周期性时使用 | `7` | 重复次数（每天/每个工作日/每周最大 500，每两周/每月最大 50）              |
| `--until-date <date>` | 周期性按日期结束时使用 | — | 结束日期（ISO 8601，含时区，如 `2026-05-10T00:00:00+08:00`） |

---

## update — 更新会议

```bash
# 修改会议主题
tmeet meeting update --meeting-id "100000000" --subject "新主题"

# 修改时间
tmeet meeting update \
  --meeting-id "100000000" \
  --start "2026-04-10T15:00:00+08:00" \
  --end "2026-04-10T16:00:00+08:00"

# 修改入会限制并开启等候室
tmeet meeting update \
  --meeting-id "100000000" \
  --join-type 3 \
  --waiting-room

# 修改周期性会议（必须传 --meeting-type 1，否则会被当作普通会议处理）
tmeet meeting update \
  --meeting-id "100000000" \
  --meeting-type 1 \
  --subject "每周站会（新主题）" \
  --recurring-type 2 \
  --until-type 1 \
  --until-count 20
```

> ⚠️ **周期性会议注意**：修改周期性会议时，如果没有修改会议类型，**必须传 `--meeting-type 1`**，否则系统会将其修改为普通会议，导致周期规则丢失。

### 参数

| 参数 | 必填 | 默认值 | 说明                                               |
|------|------|--------|--------------------------------------------------|
| `--meeting-id <id>` | ✅ | — | 会议 ID                                            |
| `--subject <text>` | 否 | — | 新会议主题                                            |
| `--start <time>` | 否 | — | 新开始时间（ISO 8601，含时区）                              |
| `--end <time>` | 否 | — | 新结束时间（ISO 8601，含时区）                              |
| `--password <pwd>` | 否 | — | 新会议密码（4~6 位数字）                                   |
| `--timezone <tz>` | 否 | — | 新时区                                              |
| `--meeting-type <n>` | **周期性会议时必填** | `0` | 会议类型：`0`-普通，`1`-周期性                              |
| `--join-type <n>` | 否 | `0` | 入会限制：`1`-所有成员，`2`-仅受邀，`3`-仅企业内部                  |
| `--waiting-room` | 否 | `false` | 开启等候室                                            |
| `--recurring-type <n>` | 周期性时使用 | `0` | 重复类型：`0`-每天，`1`-每周一至五，`2`-每周，`3`-每两周，`4`-每月      |
| `--until-type <n>` | 周期性时使用 | `0` | 结束类型：`0`-按日期，`1`-按次数                             |
| `--until-count <n>` | 周期性时使用 | `7` | 重复次数（每天/每个工作日/每周最大 500，每两周/每月最大 50）              |
| `--until-date <date>` | 周期性按日期结束时使用 | — | 结束日期（ISO 8601，含时区，如 `2026-05-10T00:00:00+08:00`） |

---

## cancel — 取消会议

> ⚠️ **写操作，执行前请确认用户意图。**

```bash
# 取消普通会议
tmeet meeting cancel --meeting-id "100000000"

# 取消周期性会议的某个子会议
tmeet meeting cancel \
  --meeting-id "100000000" \
  --sub-meeting-id "200000001"

# 取消整场周期性会议
tmeet meeting cancel \
  --meeting-id "100000000" \
  --meeting-type 1
```

### 参数

| 参数 | 必填 | 默认值 | 说明                                   |
|------|------|--------|--------------------------------------|
| `--meeting-id <id>` | ✅ | — | 会议 ID                                |
| `--sub-meeting-id <id>` | 否 | — | 子会议 ID（取消周期性会议的某场时使用）                |
| `--meeting-type <n>` | 否 | `0` | `0`-普通会议，`1`-周期性会议；取消整场周期性会议时必须传 `1` |

---

## get — 获取会议详情

```bash
# 通过会议 ID 查询（优先级更高）
tmeet meeting get --meeting-id "100000000"

# 通过会议码查询
tmeet meeting get --meeting-code "123456789"
```

### 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `--meeting-id <id>` | 二选一 | 会议 ID（优先级高于会议码） |
| `--meeting-code <code>` | 二选一 | 会议码 |

> `--meeting-id` 和 `--meeting-code` 必须提供其中一个。

---

## list — 获取会议列表

```bash
# 查询所有会议列表（不限时间范围）
tmeet meeting list

# 按时间范围查询
tmeet meeting list \
  --start "2026-04-01T00:00:00+08:00" \
  --end "2026-04-30T23:59:59+08:00"

# 展示所有子会议
tmeet meeting list --show-all-sub 1
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--start <time>` | 否 | — | 查询起始时间（ISO 8601，含时区） |
| `--end <time>` | 否 | — | 查询结束时间（ISO 8601，含时区） |
| `--show-all-sub <n>` | 否 | `0` | 展示所有子会议：`0`-不展示，`1`-展示 |

---

## list-ended — 获取已结束会议列表

```bash
# 查询所有已结束会议
tmeet meeting list-ended

# 按时间范围查询已结束会议
tmeet meeting list-ended \
  --start "2026-04-01T00:00:00+08:00" \
  --end "2026-04-30T23:59:59+08:00"

# 分页查询
tmeet meeting list-ended \
  --start "2026-04-01T00:00:00+08:00" \
  --end "2026-04-30T23:59:59+08:00" \
  --page 2 \
  --page-size 20
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--start <time>` | 否 | — | 查询起始时间（ISO 8601，含时区） |
| `--end <time>` | 否 | — | 查询结束时间（ISO 8601，含时区） |
| `--page <n>` | 否 | `1` | 页码，从 `1` 开始 |
| `--page-size <n>` | 否 | `10` | 每页数量，最大 `20` |

---

## invitees-list — 获取会议受邀者

```bash
# 获取会议受邀者列表
tmeet meeting invitees-list --meeting-id "100000000"

# 分页获取（从第 20 条开始）
tmeet meeting invitees-list --meeting-id "100000000" --pos 20
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--meeting-id <id>` | ✅ | — | 会议 ID |
| `--pos <n>` | 否 | `0` | 分页起始位置 |

---

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `--subject is required` | 缺少必填参数 | 补充 `--subject` |
| `--start format error` | 时间格式不合法（如缺少时区） | 改用 `2026-03-12T14:00:00+08:00` 格式 |
| `--meeting-id is required` | 缺少必填参数 | 补充 `--meeting-id` |

## 参考

- [tmeet](../SKILL.md) — 全部命令概览
- [tmeet-record](tmeet-record.md) — 录制管理
- [tmeet-report](tmeet-report.md) — 会议报告
