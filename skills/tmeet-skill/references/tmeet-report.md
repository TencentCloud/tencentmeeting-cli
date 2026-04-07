# tmeet report — 会议报告

> **前置条件：** 先执行 `tmeet auth login` 完成登录授权。

时间参数格式：`2026-03-12T14:00:00+08:00` 或 `2026-03-12T14:00+08:00`（必须包含时区）。

---

## participants — 获取参会人列表

```bash
# 获取会议参会人列表
tmeet report participants --meeting-id "100000000"

# 分页获取（每页 50 条，从第 50 条开始）
tmeet report participants \
  --meeting-id "100000000" \
  --pos 50 \
  --size 50

# 获取周期性会议某个子会议的参会人
tmeet report participants \
  --meeting-id "100000000" \
  --sub-meeting-id "200000001"

# 按时间范围过滤参会人
tmeet report participants \
  --meeting-id "100000000" \
  --start "2026-04-10T14:00:00+08:00" \
  --end "2026-04-10T15:00:00+08:00"
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--meeting-id <id>` | ✅ | — | 会议 ID |
| `--sub-meeting-id <id>` | 否 | — | 子会议 ID（周期性会议的某场） |
| `--pos <n>` | 否 | `0` | 分页起始位置 |
| `--size <n>` | 否 | `20` | 每页数量，最大 `100` |
| `--start <time>` | 否 | — | 查询起始时间（ISO 8601，含时区） |
| `--end <time>` | 否 | — | 查询结束时间（ISO 8601，含时区） |

---

## waiting-room-log — 获取等候室成员列表

```bash
# 获取等候室成员列表
tmeet report waiting-room-log --meeting-id "100000000"

# 分页获取
tmeet report waiting-room-log \
  --meeting-id "100000000" \
  --page 2 \
  --page-size 50
```

### 参数

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `--meeting-id <id>` | ✅ | — | 会议 ID |
| `--page <n>` | 否 | `1` | 页码 |
| `--page-size <n>` | 否 | `20` | 每页数量 |

---

## 常见错误

| 错误现象 | 原因 | 解决方案 |
|---------|------|---------|
| `--meeting-id is required` | 缺少必填参数 | 补充 `--meeting-id` |
| `--start format error` | 时间格式不合法（如缺少时区） | 改用 `2026-03-12T14:00:00+08:00` 格式 |

## 参考

- [tmeet](../SKILL.md) — 全部命令概览
- [tmeet-meeting](tmeet-meeting.md) — 会议管理
- [tmeet-record](tmeet-record.md) — 录制管理
