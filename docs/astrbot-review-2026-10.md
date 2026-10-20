# AstrBot QQ 官方适配器源码审阅（2026-10）

审阅对象：`AstrBotDevs/AstrBot` 的 `qqofficial`（WebSocket）与 `qqofficial_webhook` 适配器。

目标：只吸收能提升 SDK 兼容性、稳定性和错误可观测性的做法，不引入 AstrBot 的业务层逻辑。

## 结论

AstrBot 的 QQ 官方适配器有几处值得借鉴的“防御性”处理，尤其是：

1. 平台返回 HTTP 200 但 body 里带 `err_code` 时，不能当成功。
2. 被动回复 `msg_id` 过期/越权时，可以降级为主动消息。
3. markdown 被平台拒绝时，降级为纯文本。
4. 同一条消息多次回复必须使用不同 `msg_seq`。
5. `message_reference.message_id` 对用户消息不能直接用 `ROBOT1.0_`，要用 `message_scene.ext` 里的 `REFIDX_`。
6. Webhook 必须先去重再处理，且重试期间不能并发执行业务 handler。

## 已吸收

### 1. HTTP 200 业务错误识别

文件：`openapi/v1/openapi.go`

官方错误体示例：

```json
{
  "err_code": 40034005,
  "message": "回复消息msg_id已过期",
  "trace_id": "..."
}
```

部分旧接口使用 `code`。此前 SDK 只按 HTTP 状态码判断，HTTP 200 + `err_code` 会被当成成功，导致：

- 发消息失败但调用方以为成功；
- 媒体上传返回空/错误时继续使用无效 `file_info`；
- `msg_id` 过期时没有机会降级。

现在 `OnAfterResponse` 会解析 `err_code`/`code`（兼容数字和字符串），非 0 时返回 `errs.New(code, message, traceID)`。

### 2. 空响应检测

文件：`openapi/v1/message.go`

AstrBot 对 `APIReturnNoneError` 做重试。我们新增：

- `decodeMessage` / `decodeMedia` / `decodeStreamMessage`
- 空 body 或 `null` 返回 `errs.ErrEmptyResponse`
- `options.WithRetryOnTransient()` 可选重试一次

默认不自动重试，因为“响应丢失但消息已送达”时重试可能造成重复发送。

### 3. 自动补充 msg_seq

文件：`openapi/v1/message.go`

官方要求：相同 `msg_id + msg_seq` 重复发送会失败（`40054005`）。

现在 `PostGroupMessage` / `PostC2CMessage` 会在：

- `MsgID` 或 `EventID` 非空；
- `MsgSeq == 0`；

时自动补充一个进程内唯一、时间种子的 `msg_seq`。不会修改调用方传入的结构体。

遇到 `40054005` 时，还会自动换一个 `msg_seq` 重试一次。

### 4. Markdown 降级

文件：`openapi/v1/message.go`

官方错误码包括：

- `50056` 不允许发送 markdown
- `40034011` 无效 markdown
- `40034127` 无 markdown 模板权限
- `304036` 旧接口 markdown 权限

现在如果消息带 `Markdown` 且平台拒绝，会自动：

- 清除 `Markdown`
- 用 `Markdown.Content` 填充 `Content`
- `MsgType` 从 2 降为 0
- 保留 `msg_id`/`msg_seq` 重试一次

这是默认行为，因为不改变消息类型/配额，只是格式降级。

### 5. 被动消息降级为主动消息（可选）

文件：`openapi/options/options.go`、`openapi/v1/message.go`

官方被动回复错误码：

- `40034005` msg_id 已过期
- `40034024` msg_id 无效或越权
- `40034025` event_id 无效
- `40034026` event_id 已过期
- `40034027` 事件不支持回复
- `40034128` 被动回复时间/次数超限
- 旧接口 `304103` / `304026` / `304027` / `304028`

调用方可以传：

```go
options.WithProactiveFallback()
```

SDK 会去掉 `msg_id`/`event_id` 后按主动消息重试一次。

默认关闭：主动消息有独立频控/日限额，调用方通常需要自己排队。SDK 不应在调用方不知情时绕过其配额控制。

### 6. Webhook 处理中就去重

文件：`interaction/webhook/handler.go`

此前同步 ACK 模式下，只有处理成功后才标记去重。平台超时重试可能与第一次处理并发，导致同一条消息执行两次。

现在：

- 收到事件后先 `MarkAll`，再处理；
- 处理失败时 `UnmarkAll`，让平台可以重试；
- 处理期间到达的重复事件直接 ACK 成功，不再进入业务 handler。

AstrBot 也是先写 `_seen_event_ids` 再 `func(msg)`。

### 7. 引用回复 ID 解析

文件：`dto/message.go`

新增 `MessageScene.Ext []string`、`MsgIdx()`、`RefMsgIdx()`、`Message.ResolvedReferenceID()`；
同时补上 `Message.MessageType`、`Message.MsgElements`、`MessageElement`、`IsQuote()`、`QuotedElement()`，
避免引用消息的 `message_type=103` / `msg_elements` 在结构化解析时丢失。

官方规则：

- 机器人自己发的消息：可以直接用 `message_reference.message_id`；
- 用户消息/非机器人消息：必须用 `message_scene.ext` 里的 `msg_idx=REFIDX_...`；
- `message.id`（`ROBOT1.0_...`）不能直接用于引用。

AstrBot 专门为此写了 `_extract_self_ref_idx`。SDK 现在把原始字段保留并提供解析方法，调用方可直接使用。

### 8. 频道消息清理 `msg_type`

文件：`openapi/v1/message.go`、`openapi/v1/direct_message.go`

`msg_type` 是群聊/单聊 v2 字段，频道消息和私信频道接口不使用。AstrBot 在频道发送前执行 `payload.pop("msg_type", None)`。

现在 `PostMessage` / `PostDirectMessage` 会克隆消息并清空 `MsgType`，避免把 v2 字段发给频道接口。

### 9. file_image 与 message_reference 互斥

文件：`openapi/v1/message.go`

botpy 明确提示：`file_image` 与 `message_reference` 不能同时传入，优先保留 `file_image`。

`PostMessageMultipart` 现在在带 `fileImageData` 时不再写入 `message_reference`。

### 10. 签名错误不再被吞掉

文件：`interaction/signature/interaction.go`

`Verify` / `Generate` 在缺少 `X-Signature-Timestamp` 时现在返回明确 error，而不是继续用 nil content 验签。

## 暂不吸收（候选）

### A. 大于 10MB 的分片上传

AstrBot 实现了完整的 QQ 分片上传：

- `POST /v2/groups/{group_openid}/upload_prepare`
- `PUT` 预签名 URL 上传分片
- `POST /v2/groups/{group_openid}/upload_part_finish`
- `POST /v2/groups/{group_openid}/files` 合并
- 计算 `md5`、`sha1`、前 10,002,432 字节 `md5_10m`
- 重试 `40093001`，识别日配额 `40093002`
- 支持 C2C/群、分片 index 从 0 或 1 开始、并发上限、重试超时

这是完整功能而非小修补，建议作为可选 `ChunkedFileAPI` 单独实现，避免把现有 `PostGroupFile` 改复杂。

### B. 流式消息 writer

AstrBot 对 C2C 流式做了较多处理：

- 中间分片 `state=1`，结束分片 `state=10`
- 结束分片 content 必须以 `\n` 结尾
- 分片节流 1 秒
- 收到 `break` 时先结束当前流，再开新流
- 富媒体时降级为非流式

我们目前提供低层 `PostStreamMessage`，由调用方控制 state/index/节流。后续可以增加 `StreamWriter` 可选助手。

### C. Webhook 快速 ACK + 有界队列

AstrBot 使用 `ConnectionSession(max_async=1)`，收到事件后快速 ACK，再由单 worker 串行处理。

我们保留了同步 ACK 默认值，避免改变旧行为；同时用“处理中标记”消除了并发重复执行。若后续要默认快速 ACK，需要增加有界队列和优雅关闭，否则高并发下 goroutine 数量不可控。

### D. 媒体拆分与附件类型映射

AstrBot 会：

- 按“一条消息最多一个媒体”拆分消息链；
- 根据 `content_type`/扩展名映射 image/voice/video/file；
- 给无 scheme 的附件 URL 补 `https://`；
- 入站语音转 wav。

SDK 的 `MessageToCreate` 本身只有一个 `Media` 字段，SealDice 也已有拆分逻辑；附件映射属于消费侧。暂不在 SDK 内重复实现。

### E. 群信息兜底

AstrBot 在群信息接口无权限时，返回入站消息里已有的 `Group` 基本信息。

这是业务层兜底，SDK 的 `GetGroupInfo` 应保持“返回错误”，由调用方决定是否使用事件里的缓存信息。

### F. 表情标签解析

AstrBot 会把 `<faceType=...>` 中的 base64 `ext` 解码成 `[表情:xxx]`。

这是展示层逻辑，和协议健壮性无关，暂不加入 SDK。

### G. Webhook ACK 字段差异

网上存在多种 ACK 写法：

- 官方 payload 结构：`{"op":12,"d":0}`（我们当前使用）
- AstrBot：`{"opcode":12}`
- 部分博客：`{"op_code":12,"d":{"event_id":"...","status":0,"message":"success"}}`

目前没有证据表明标准结构会被拒绝，多个 Go/Rust SDK 也使用 `op/d`。在拿到平台明确结论前不修改，避免为了一个未证实的差异破坏兼容性。

## 验证

- `golangci-lint run ./...`：0 issues
- `go vet ./...`：clean
- `go test -short ./...`：全绿
- SealDice 临时 worktree：`go build ./...` 通过，`go test ./dice -run OfficialQQ` 通过，仅有 go.mod/go.sum 变化
