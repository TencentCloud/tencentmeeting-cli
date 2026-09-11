# 命令参考

[返回 README](../README.md) | [English](./command_en.md)

---

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

### minutes — 元宝纪要

#### `minutes search` — 搜索元宝纪要

按关键词、时间范围搜索元宝纪要。所有过滤参数均为可选，可任意组合。

```bash
tmeet minutes search [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--query` | string | ❌ | — | 搜索关键词，最多 50 字 |
| `--start` | string | ❌ | — | 搜索时间下界（ISO 8601，如 `2026-03-12T14:00+08:00`） |
| `--end` | string | ❌ | — | 搜索时间上界（ISO 8601，如 `2026-03-12T14:00+08:00`） |
| `--page-token` | string | ❌ | — | 分页游标，首页不传；翻页时传入上一次响应的 `next_page_token` |
| `--page-size` | int | ❌ | `20` | 每页大小，默认 20，最大 50 |

**示例：**

```bash
# 按关键词搜索
tmeet minutes search --query "季度目标"

# 按时间范围搜索
tmeet minutes search \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# 关键词 + 时间范围组合搜索
tmeet minutes search \
  --query "项目评审" \
  --start "2026-04-01T00:00+08:00" \
  --end "2026-04-30T23:59+08:00"

# 翻下一页
tmeet minutes search \
  --query "项目评审" \
  --page-token "<next_page_token>" --page-size 20
```

---

#### `minutes get` — 查询元宝纪要详情

通过纪要 ID 或会议 ID 查询元宝纪要详情。`--minute-id`、`--meeting-id` 二选一。

```bash
tmeet minutes get (--minute-id <纪要ID> | --meeting-id <会议ID>) [选项]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--minute-id` | string | 二选一 | — | 纪要唯一标识 |
| `--meeting-id` | string | 二选一 | — | 会议 ID（周期级），需配合 `--sub-meeting-id` 定位实例 |
| `--sub-meeting-id` | string | — | — | 子会议 ID（周期会议实例）；非周期会议不传 |
| `--overview` | bool | — | `true` | 是否获取会议概览 |
| `--summary-points` | bool | — | `true` | 是否获取要点 |
| `--todos` | bool | — | `true` | 是否获取待办 |
| `--short-summary` | bool | — | `false` | 是否获取滚动总结历史序列 |
| `--page-token` | string | — | — | 分页游标，当一个 `meeting_id` 返回多份纪要时可用 |
| `--page-size` | int | — | `10` | 每页大小，默认 10，最大 30 |

**示例：**

```bash
# 按纪要 ID 查询
tmeet minutes get --minute-id "minute_abc123"

# 按会议 ID 查询
tmeet minutes get --meeting-id "6953553464429888300"

# 按会议 ID + 子会议 ID 查询（周期性会议）
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --sub-meeting-id "100001"

# 仅获取概览和待办，不获取要点
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --summary-points=false

# 翻下一页（当一个会议有多份纪要时）
tmeet minutes get \
  --meeting-id "6953553464429888300" \
  --page-token "<next_page_token>" --page-size 10
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

### app — 应用信息

管理当前 CLI 应用的接入信息（首页地址、会中打开布局、SDK 名称）。

#### `app get` — 获取应用信息

查询当前 CLI 应用的接入信息。

```bash
tmeet app get
```

> 无参数。

---

#### `app set` — 设置应用信息

设置当前 CLI 应用的接入信息。至少需要指定 `--homepage` / `--layout-style` / `--sdk-name` 中的一个，仅传入的字段会被更新。

```bash
tmeet app set [options]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|:----:|--------|------|
| `--homepage` | string | 三选一 | — | 应用首页 URL；必须使用 `http://` 或 `https://`；`https://` 时该地址需拥有**被本机信任**的 SSL/TLS 证书，否则会中打开会被拦截；长度上限 **200 个字符**。<br/>⚠️ 腾讯会议客户端 **3.45.10 之前的版本不支持打开 `http://` 页面**，仅支持 `https://`；3.45.10 及以后版本两者均支持。若无法确认目标用户的客户端版本，建议优先使用 `https://` |
| `--layout-style` | string | 三选一 | 服务端默认 `sidebar` | 会中打开布局：`sidebar`（窄侧边栏）\| `wide_sidebar`（宽侧边栏）\| `popout`（独立弹窗）。**未传该参数时 CLI 不下发该字段**：若服务端当前无值则由服务端置为 `sidebar`，若已有值则保持不变 |
| `--sdk-name` | string | 三选一 | — | 应用 SDK 名称；长度上限 **20（按显示宽度计算：ASCII 计 1，非 ASCII 如中文计 2）** |

**示例：**

```bash
# 设置应用首页地址
tmeet app set --homepage "https://example.com"

# 设置会中打开布局为独立弹窗
tmeet app set --layout-style popout

# 设置 SDK 名称
tmeet app set --sdk-name "my-sdk"

# 同时设置多个字段
tmeet app set --homepage "https://example.com" --layout-style wide_sidebar --sdk-name "my-sdk"
```

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
