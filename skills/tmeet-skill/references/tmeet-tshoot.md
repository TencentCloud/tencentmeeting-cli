# tshoot — 问题排查

## `tshoot log` — 导出本地日志

将本地日志打包为 zip 文件，输出到 `~/tmeet_ts_{datetime}.zip`，可用于问题排查。支持按时间范围过滤，不传时间参数则导出全部日志。

```bash
tmeet tshoot log [选项]
```

### 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--start` | string | 与 `--end` 同时使用 | — | 日志查询开始时间，ISO 8601，如 `2026-03-12T14:00+08:00` |
| `--end` | string | 与 `--start` 同时使用 | — | 日志查询结束时间，ISO 8601，如 `2026-03-12T15:00+08:00` |
| `--upload` | bool | 否 | `false` | 上传日志到服务器，需要登录 |

> `--start` 和 `--end` 必须同时传入或同时不传。

### 示例

```bash
# 导出全部日志
tmeet tshoot log

# 导出指定时间范围内的日志
tmeet tshoot log \
  --start "2026-04-10T00:00+08:00" \
  --end "2026-04-10T23:59+08:00"

# 导出日志并上传至服务器（需要登录）
tmeet tshoot log --upload
```

### 输出示例

```
output log saved to: ~/tmeet_ts_20260410_153000.zip
```

### 说明

- 日志文件存储在 `~/.tmeet/logs/` 目录下
- 输出的 zip 文件保存在用户主目录（`~/`）
- 若指定时间范围内无日志，输出提示 `choose time range has no log`
- `--upload` 用于将日志上传到服务器，需要先完成 `tmeet auth login` 登录授权
