# tencent-connect/botgo fork 调研（2026-10）

## 范围与方法

- 数据源：GitHub forks API（通过代理访问），共 73 个 fork。
- 时间窗口：2026-08-20 ~ 2026-10-20（两个月）。
- 筛选条件：`pushed_at` 或 `updated_at` 在窗口内。
- 检查方式：下载默认分支源码，与 `tencent-connect/botgo` master 做 tree diff；重点看核心 SDK 代码，不看业务示例。
- 兼容性原则：**不破坏 SealDice 现有调用面；新能力优先做成可选包/可选接口。**

## 窗口内的 fork

| fork | pushed_at | 默认分支状态 | 结论 |
|---|---|---|---|
| afjcjsbx/botgo | 2026-10-15 | 与上游一致 | 无可汲取代码 |
| diablobear2026/botgo | 2026-10-12 | 只改 examples（ASR/TTS/COS/FAQ/silk/voice_reply） | 业务特化，不收 |
| SpellingDragon/botgo | 2026-10-09 | 与上游一致 | 无可汲取代码 |
| SHAWNHU0621/botgo | 2026-10-07 | 只改 `openapi/v1/openapi.go` | **已采纳：请求级 auth 修复** |
| HDU-Channal/botgo | 2026-09-29 | 只改 `openapi/v1/openapi.go` | **已采纳：同一修复** |
| WindowsSov8forUs/botgo-plus | 2026-09-26 | 大量改造 | 见下文，按候选清单处理 |
| cyqmq/botgo | 2026-09-18 | 与上游一致 | 无可汲取代码 |
| sealdice/botgo | 2026-09-16 | 我们的基线 | — |
| crazysarah/botgo | 2026-09-04 | 与上游一致 | 无可汲取代码 |
| wowzydld/botgo | 2024-12-27 pushed，2026-09-20 updated | 仅元数据更新 | 跳过 |
| usamikinoko/botgo | 2024-12-27 pushed，2026-09-02 updated | 仅元数据更新 | 跳过 |

> 说明：部分 fork 的 `updated_at` 很新但 `pushed_at` 是旧日期，通常只是 star/描述/分支元数据变化，没有代码更新。

## 已采纳

### 1. OnBeforeRequest 请求级 auth（SHAWNHU0621 / HDU-Channal）

上游 v0.2.1 的 `OnBeforeRequest` 同时修改了 `resty.Client`：

```go
c.SetAuthScheme(tk.TokenType)
c.SetAuthToken(tk.AccessToken)
```

这是 client 级可变状态，并发请求会互相覆盖，存在 data race。两个 fork 都改成只设置 request：

```go
r.SetAuthScheme(tk.TokenType)
r.SetAuthToken(tk.AccessToken)
```

我们已采纳，保留 request 级设置，删除 client 级设置。

### 2. Go 图片格式转换（botgo-plus 思路）

新增 `pkg/image`：

- 纯 Go，不依赖 ffmpeg
- 已是 png/jpg/gif 直接透传
- BMP 转 JPEG，其他支持格式转 PNG
- 内存内转换，不写 `data/cache`
- 限制 50MiB / 单边 10000px，防解压炸弹

这是 botgo-plus `pkg/image` 思路的重写版本，只保留 QQ 官方图片格式所需能力。

### 3. `dto.Message.Ret`

botgo-plus 的 `Message` 有：

```go
Ret int `json:"ret,omitempty"` // 超过主动限制返回 22009
```

已以增量字段方式加入，不影响现有 JSON。

### 4. 日志脱敏（review botgo-plus token 时发现）

修复了我们自己继承的日志泄密：

- token 请求不再打印 `clientSecret`
- token 响应不再打印 `access_token`
- WS Identify/Resume 不再打印完整 payload

## 候选清单（待一起确认）

| 优先级 | 候选 | 来源 | 价值 | 成本 | 备注 |
|---|---|---|---|---|---|
| P1 | `PostMessageMultipart` / `PostDirectMessageMultipart` | botgo-plus | 频道图片 multipart 发送，避开 base64 限制 | 低 | 建议放进可选 `FormFileAPI`，不动 `OpenAPI` |
| P2 | 论坛发帖 `PostForum` | botgo-plus `PostFourm` | 补论坛创建帖子能力 | 低 | 先核对当前官方 `/channels/{channel_id}/threads` 文档 |
| P2 | 去重 EventID 为空时使用 payload hash fallback | botgo-plus | 极少数无 EventID 事件的去重 | 低 | 用 xxhash/TTL cache，不照搬它的 sync.Map 全量存 payload |
| P3 | `pkg/mp4` 转码 | botgo-plus | 非 mp4 视频转码 | 中高 | 依赖 ffmpeg；需 context、超时、大小限制 |
| P3 | 音频适配 | botgo-plus / 官方新文档 | 新版本已支持 mp3/wav | 中 | **优先直接支持 mp3/wav，不再为旧版强上 silk** |
| — | `openapi/v2` 双实现 | botgo-plus | 无 | 高 | 域名仍是旧 `api.sgroup.qq.com`，拒绝 |
| — | Gin Webhook Server | botgo-plus | 无 | 高 | 校验顺序/op 12 ACK/去重/多账号都不如现有实现 |
| — | `token.BotToken/InitToken` | botgo-plus | 无 | 中 | 无 60s 提前量/退避/ForceRefresh，且日志泄密 |
| — | `sessions/multi` | botgo-plus | 无 | 中 | 仍是旧 panic/无退避模型 |
| — | 业务示例（ASR/TTS/COS/FAQ） | diablobear2026 | 业务特化 | — | 不属于 SDK 能力 |

## 兼容性红线

- 不改 `websocket.WebSocket`、`event.RegisterHandlers*`、`interaction/webhook.HTTPHandler`、`token.StartRefreshAccessToken` 签名。
- 新 API 一律作为可选接口，不塞进 `openapi.OpenAPI`。
- 新包不进入默认依赖链；ffmpeg/大体积 codec 必须可选。
- SealDice 现有官方 QQ 适配器用到的符号必须保持可用。

## P1 / P2 详细评估（2026-10-20）

### P1：`PostMessageMultipart` / `PostDirectMessageMultipart`

**官方文档核对**

- `/channels/{channel_id}/messages` 明确支持：
  - `application/json`
  - `multipart/form-data`
  - multipart 专有字段：`file_image`
- 通用字段：`content`、`embed`、`ark`、`image`、`message_reference`、`msg_id`、`event_id`、`markdown`
- `/dms/{guild_id}/messages` 文档只写了 `application/json`，**没有** multipart。

**结论**

- `PostMessageMultipart`：**采纳**，作为可选 `FormFileAPI` 方法。
- `PostDirectMessageMultipart`：**不采纳**。官方 DM 文档没有 multipart；botgo-plus 的实现属于未文档化用法，不适合放进正式 SDK。
- 实现只发送官方文档列出的字段，不发送未文档化的 `msg_type` / `keyboard`。
- 如果同时传 `file_image`，不再发送 `image` 字段，避免二者冲突。

**对 SealDice 的影响**

- 当前 SealDice 的 `sendQQChannelMsgRaw` / `sendQQGuildDirectMsgRaw` 对 `ImageElement` 是空 case，还没有发频道图片。
- 所以这个方法目前是“SDK 可用、SealDice 未接线”，不会影响现有行为。
- 如果以后要让 SealDice 支持频道本地图片，优先用这个方法，而不是 base64。

### P2：论坛发帖 `PostForumThread`

**官方文档核对**

- 接口：`PUT /channels/{channel_id}/threads`
- 请求：`title` / `content` / `format`
- `format`：1=text，2=html，3=markdown，4=json
- 响应：`task_id` / `create_time`
- 限制：公域机器人暂不支持，仅私域机器人可用。

**结论**

- **采纳**，作为可选 `ForumAPI.PostForumThread`。
- 没有沿用 botgo-plus 的 `PostFourm` 拼写，使用 `PostForumThread`。
- 只做“发表帖子”；帖子列表/详情/删除暂不加入，避免一次扩展太多。

**对 SealDice 的影响**

- SealDice 当前没有处理论坛事件，也没有论坛发帖需求。
- 属于 SDK 能力补全，不影响现有官方 QQ 适配器。

### P2：EventID 为空时的 payload hash 去重 fallback

**评估**

- 官方 dispatch 事件基本都有 `id`，当前按 `EventID` 去重已经覆盖绝大多数场景。
- fallback 需要 hash 整个 `RawMessage`，并额外维护一套 key 规则；价值低。
- 对“没有 id 且 payload 完全相同”的合法事件，存在误判重复的风险。

**结论**

- **暂不实现**，保留在 backlog。
- 如果未来遇到真实的无 EventID 重复事件，再按 `xxhash(RawMessage)` + TTL cache 加。

### P3：媒体转码

- `pkg/image`：已采纳（纯 Go）。
- `pkg/mp4`：暂缓，依赖 ffmpeg，需要 context/超时/大小限制。
- `pkg/silk`：**不采纳旧版 silk 强转**。新版官方已支持 mp3/wav，音频优先直接支持 mp3/wav；只有确实需要 silk 时再做可选包。

## 本轮实现与验证

- 新增 `FormFileAPI.PostMessageMultipart`。
- 新增 `ForumAPI.PostForumThread`。
- 新增 `dto.ForumThreadToCreate` / `dto.ForumThreadResp` 与格式常量。
- 新增 multipart / forum 的 httptest 测试。
- `golangci-lint run ./...`：0 issues
- `go test -short ./...`：全绿
- `go vet ./...`：clean
- SealDice `go build ./...` + `go test ./dice -run OfficialQQ`：通过

## 追加：官方重复推送与去重策略（网上核对）

### 官方行为

- 官方明确说明：为保证可达，**相同 msg_id 可能重复推送**。
- `GROUP_AT_MESSAGE_CREATE` / `C2C_MESSAGE_CREATE` 文档都写了：
  > 为确保消息可达，相同 msg_id 可能重复推送，开发者需结合 msg_seq 做去重。
- AstrBot 的线上问题与修复（PR #6320）也确认：
  - QQ Webhook 超时后会重试，最多 3 次
  - 重试携带**相同顶层 id**
  - 不去重会导致同一条消息处理 2~3 次

### 结论

这不是“官方实现写错了”，而是官方语义是 **at-least-once**，去重责任在 SDK/业务侧。

botgo-plus 用 `payload.Data` 的 hash 去重，是一种**兜底策略**：

- 优点：不依赖事件类型、不依赖顶层 id 是否存在
- 缺点：
  - 对“无 id 且 payload 完全相同”的合法事件有误杀风险
  - 需要存完整 payload 或 hash，增加内存/清理逻辑
  - 没有利用官方更精确的 `msg_id` / `event id`

### 我们的改进

不采用 payload hash，而是使用**双 key 去重**：

- 顶层 `event id`
- 消息事件的 `d.id`（即官方说的 msg_id）

任一 key 命中即视为重复：

- Webhook 重试：顶层 id 相同，直接命中
- 同一条消息换 event id 重推：`d.id` 相同，仍然命中
- 不会因为两条无 id 但 payload 相同的合法事件被误杀

实现位于 `internal/dedup`，WS 与 Webhook 共用。
