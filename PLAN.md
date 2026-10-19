# sealdice/botgo 改造计划

> 基线：`github.com/sealdice/botgo v0.0.0-20260724133029-df949851f30b`
> 目标：用 `github.com/lxzan/gws` 替换 gorilla/websocket，修复 WS 生命周期与 token 续期，补全 Webhook，并选择性融合 Better-Bot-Go 的协议/生命周期能力。
> 原则：**公共 API 尽量不变；能加包就不改旧包；必须改的旧实现保持函数签名和行为兼容。**

---

## 0. 现状结论

### 0.1 这个 fork 实际改了什么

| 模块 | 相对官方 v0.2.1 的改动 |
|---|---|
| `token/token_source.go` | `expires_in` 支持 string/number |
| `websocket/client/client.go` | 增加 context、`ctx.Done()`、4009/9000 日志降噪 |
| `sessions/local/local.go` | 增加 ctx、`session.AppID`、4014 降级 |
| `sessions/remote/remote.go` | ctx 透传、Redis BRPop 使用调用方 ctx |
| `interaction/webhook/webhook.go` | 仅 import/注释变化，功能未改 |

### 0.2 必须解决的问题

**Token / WS**

- 刷新提前量只有 9s，官方建议 60s。
- 刷新失败每 1s 重试，连续 10 次 panic。
- 没有 `ForceRefresh/Invalidate`，4004/11244 可能复用旧 token。
- 11244 只刷新不重试当前请求。
- `saveSeq` 在 handler 处理前执行，失败会丢事件。
- Identify/Resume 失败直接 return，不重新排队。
- 4914/4915：local 热循环，remote 进程 panic。
- 4013/4014 未按官方错误码表处理；remote 没有 4014 降级。
- 连接失败没有退避，可能形成连接风暴。
- `getRefreshMilliSec` 使用包级 `rand.Rand`，多账号并发有数据竞争。
- `expires_in <= 0` 无保护。

**Webhook**

- `r.Body.Read` 可能短读。
- 先验签、后判断 op 13，可能拒掉官方 URL 校验请求。
- 验签失败/body 错误没有 400/401。
- 没有 `Content-Type`。
- 没有 panic 恢复。
- 没有事件去重。
- ACK 与业务处理同步，长命令可能超时。
- 没有 server、多账号、TLS、优雅关闭、请求限制。
- 没有 HTTP handler 测试。

---

## 1. 目标与非目标

### 1.1 目标

1. **gws 替换 gorilla/websocket**
   - 保持 `websocket.WebSocket` 接口不变。
   - 保持 `websocket/client` 包路径和 `Setup()` 不变。
   - 行为对齐现有实现：Identify/Resume/心跳/分片/事件队列/关闭。
   - 增加传输层 ping/pong、deadline、写超时、panic 恢复。

2. **修复 WS 生命周期与 token 续期**
   - 60s 刷新提前量。
   - 指数退避 + jitter，不 panic。
   - 强制刷新 + 11244 自动重试一次。
   - seq 处理成功后保存。
   - Identify/Resume 失败重新排队。
   - 按官方错误码表分类：Resume / Identify / Terminal / Transient。
   - 4914/4915 终止而不是 panic/热循环。
   - 4014 降级在 local/remote 一致。
   - 连接退避。

3. **Webhook 生产化**
   - 修 handler 的短读、op 13 顺序、状态码。
   - 增加 `Handler`/`Server`，支持多账号、TLS、优雅关闭。
   - 事件去重、panic 恢复、ACK 模式、请求限制。
   - 增加 handler/server 测试。

4. **选择性融合 BB**
   - 主动消息开关事件：`GROUP_MSG_RECEIVE/REJECT`、`C2C_MSG_RECEIVE/REJECT`。
   - `Check`/Observer 钩子。
   - Webhook server 的多账号/TLS/生命周期。
   - 正向 WS gateway（用 gws 重写，不照搬 BB 实现）。
   - `GetGroupMembers`（按官方 cursor 实现，不照搬 BB 的旧 POST 实现）。
   - `GetGroupInfo`、`GetBotState`、`GetGroupMember`。
   - `PostFormFileImage`、`PostFormFileReaderImage`。
   - `PostStreamMessage`（官方新接口，BB 也没有）。
   - `GROUP_JOIN_REQUEST` 事件。

### 1.2 非目标

- 不移植 BB 的 `AuthAcess`/`SendApi` 全局 token 模型。
- 不移植 BB 的 `websocket2` 客户端（比现有实现旧）。
- 不移植 BB 的 `webhook` 事件分发 if-chain（现有 `event.ParseAndHandle` 更完整）。
- 不移植 BB 的 `onebot.Frame`（不是标准 OneBot 协议）。
- 不移植 BB 的 Gin/logrus/rotatelogs 依赖。
- 不移植 BB 的 `WaitingFrames`（本身未实现 resolve）。
- 不改 `event.RegisterHandlers`、`event.RegisterHandlersByAppID` 的签名。
- 不改 `interaction/webhook.HTTPHandler` 的签名。

---

## 2. 兼容性策略

### 2.1 公共 API

保持以下 API 不变：

- `botgo.NewOpenAPI` / `NewSandboxOpenAPI`
- `botgo.NewSessionManager`
- `event.RegisterHandlers` / `RegisterHandlersByAppID`
- `websocket.WebSocket` 接口
- `websocket/client.Setup()`
- `interaction/webhook.HTTPHandler`
- `sessions/local.New` / `sessions/remote.New`
- `token.StartRefreshAccessToken` 签名

新增能力全部用**新包、新类型、新可选接口**实现。

### 2.2 Go 版本

- `gws` 要求 Go 1.20。
- `go.mod` 从 `go 1.16` 提升到 `go 1.20`。
- 这是本次唯一不可避免的兼容性变化，需要在 release note 中注明。

### 2.3 依赖

- 新增 `github.com/lxzan/gws v1.10.2`。
- 新增 `github.com/cenkalti/backoff/v4 v4.3.0`：指数退避 + jitter，不再手写。
- 新增 `github.com/jellydator/ttlcache/v3 v3.1.0`：Webhook 事件去重 / TTL + 容量上限。
- 移除 `github.com/gorilla/websocket`（迁移完成后确认无其他引用）。
- 保持 `golang.org/x/net/proxy` 用于代理拨号（已有 indirect 依赖）。

### 2.4 库优先原则

能用库就不手写：

| 需求 | 选型 | 理由 |
|---|---|---|
| WebSocket | `lxzan/gws` | 高性能、事件驱动、内置 Broadcaster/Upgrader/Authorize/SessionStorage |
| 指数退避 | `cenkalti/backoff/v4` | 成熟、支持 jitter、`RetryNotify`、`PermanentError` |
| 去重/TTL | `jellydator/ttlcache/v3` | 泛型、TTL、容量上限、自动清理 |
| Token HTTP | `net/http` + 现有 `golang.org/x/oauth2` | 标准库 + 已有依赖，不引入额外 HTTP 客户端 |
| JSON | `encoding/json` | 标准库；无必要引入 sonic |
| 代理 | `golang.org/x/net/proxy` | 已有依赖；gws `NewDialer` 直接适配 |
| Webhook 路由 | `net/http.ServeMux` | 单路径 + `X-Bot-Appid` 已足够；不引入 Gin/chi |

**gws 高级 API 使用清单**

- `gws.NewClient` + `ClientOption.NewDialer`：客户端连接 + 代理。
- `gws.NewUpgrader` + `ServerOption.Authorize` + `NewSession`：Gateway 鉴权和连接级状态。
- `gws.Broadcaster`：Gateway 广播时只压缩一次，避免循环 `WriteAsync` 的重复压缩。
- `WriteAsync` / `WritevAsync`：Gateway 非阻塞写。
- `SetDeadline` / `SetReadDeadline` / `SetWriteDeadline`：心跳与写超时。
- `gws.Recovery`：handler panic 恢复。
- `gws.CloseError`：close code 分类。
- `gws.SessionStorage`：在连接上存 appID/clientID/订阅信息。

---

## 3. 工作流

### Workstream A：gws 客户端迁移

**文件**

- `websocket/client/client.go`：改为 gws 实现。
- `websocket/client/event.go`（新增）：`gws.Event` 适配器。
- `websocket/client/close.go`（新增）：close code 分类。
- `websocket/client/heartbeat.go`（新增）：传输层 ping/pong + deadline。
- `websocket/client/client_test.go`（新增）：假 QQ gateway 测试。
- `go.mod` / `go.sum`。

**设计**

1. 保持 `Client` 的公开方法不变：
   - `New(ctx, session)`
   - `Connect()`
   - `Identify()`
   - `Session()`
   - `Resume()`
   - `Listening()`
   - `Write(*dto.WSPayload)`
   - `Close()`

2. `Connect()`：
   - 使用 `gws.NewClient(handler, &gws.ClientOption{...})`。
   - `Addr: session.URL`。
   - `TlsConfig` 默认系统根证书。
   - `HandshakeTimeout: 10s`。
   - `ReadMaxPayloadSize` 设为合理上限（例如 8MB）。
   - `ParallelEnabled: false`，保持事件顺序。
   - `Recovery: gws.Recovery`，防 handler panic。
   - `NewDialer` 使用 `proxy.FromEnvironment()`，支持 `HTTP_PROXY/HTTPS_PROXY/NO_PROXY`。
   - 保存 `*gws.Conn`。

3. `Listening()`：
   - `go c.conn.ReadLoop()`。
   - `go c.listenMessageAndHandle()` 继续消费 `messageQueue`。
   - select：`ctx.Done()`、resumeSignal、closeChan、QQ 心跳 ticker、传输层 ping ticker。
   - `ctx.Done()` 时调用 `Close()`。

4. `OnMessage`：
   - `defer message.Close()`。
   - 只接受 `OpcodeText`；JSON 解析为 `dto.WSPayload`。
   - 设置 `RawMessage`、`Session`。
   - 投递到 `messageQueue`；队列满时阻塞，形成背压。
   - 解析失败只记日志，不关闭连接。

5. `OnClose`：
   - 将 `*gws.CloseError` 转为统一错误。
   - `close(messageQueue)` 只执行一次（`sync.Once`）。
   - 非阻塞发送到 `closeChan`。
   - 处理 `ErrConnClosed`、网络错误。

6. `OnPing` / `OnPong`：
   - 回复 `WritePong`。
   - 刷新 `SetDeadline(now + pingInterval + pingWait)`。

7. `Write()`：
   - 设置 `SetWriteDeadline(now + writeTimeout)`。
   - `WriteMessage(gws.OpcodeText, payload)`。
   - 失败时通知 `closeChan`。

8. `Close()`：
   - `WriteClose(1000, nil)`。
   - `NetConn().Close()`。
   - 停止 ticker。
   - 幂等（`sync.Once`）。

9. close code 兼容：
   - 增加 `isCloseError(err, codes...)` 和 `isUnexpectedCloseError(err, allowed...)`。
   - 从 `*gws.CloseError.Code` 取值。
   - 保持现有 `Listening` 的错误分类逻辑。

**测试**

- 启动本地 gws server 模拟 QQ gateway。
- 覆盖：
  - Hello → Identify → Ready。
  - 事件下发 → handler 收到。
  - 心跳。
  - Resume。
  - 4009 → Resume。
  - 4006/4007 → Identify。
  - 4914/4915 → Terminal。
  - ctx 取消。
  - 写超时/断开。
- `go test -race`。

---

### Workstream B：Token 续期修复

**文件**

- `token/token_source.go`
- `token/token_source_test.go`（新增）
- `openapi/v1/openapi.go`（11244 重试）
- `websocket/client/client.go`（4004 强制刷新）

**设计**

1. 新增可选接口：

```go
type RefreshableTokenSource interface {
    oauth2.TokenSource
    Invalidate()
    ForceRefresh() (*oauth2.Token, error)
}
```

2. `QQBotTokenSource`：
   - `Invalidate()`：清空缓存。
   - `ForceRefresh()`：`Invalidate()` + `Token()`。
   - `Token()` 保持 `oauth2.TokenSource` 兼容。
   - singleflight 内二次检查缓存，避免重复请求。
   - `expires_in <= 0` 返回错误，不缓存。
   - 记录 trace ID。

3. `StartRefreshAccessToken`：
   - 60s 提前量。
   - 失败退避使用 `github.com/cenkalti/backoff/v4`：

```go
b := backoff.NewExponentialBackOff(
    backoff.WithInitialInterval(1*time.Second),
    backoff.WithMultiplier(2),
    backoff.WithMaxInterval(60*time.Second),
    backoff.WithRandomizationFactor(0.5),
    backoff.WithMaxElapsedTime(0), // 不停止，交给 ctx
)
```

   - 使用 `backoff.RetryNotify` + `context`，不要手写 1s 固定重试。
   - 不再 panic。
   - 新增可选错误回调：

```go
type RefreshErrorHandler func(err error)
func SetRefreshErrorHandler(h RefreshErrorHandler)
```

   - 保留原函数签名。

4. `getRefreshMilliSec`：
   - `expiresIn*1000 - 60000`。
   - jitter 用 `backoff` 的 randomization，或线程安全的 `math/rand.Int63n`。
   - `expiresIn <= 60` 时立即刷新。

5. OpenAPI 11244：
   - `handleError` 中检测 11244。
   - 如果 token source 支持 `RefreshableTokenSource`，调用 `ForceRefresh()`。
   - 对当前请求重试一次。
   - 实现方式：在 `openapi/v1/openapi.go` 增加 `doWithTokenRetry`，或使用 resty retry + 强制刷新钩子。

6. WS 4004：
   - 调用 `ForceRefresh()`，而不是 `Token()`。
   - 下次 Identify 使用新 token。

**测试**

- `httptest.Server` 模拟 token 接口：
  - 正常返回。
  - `expires_in` 为 string/number。
  - 100001 限流。
  - 100007/100016。
  - 11244。
  - `expires_in=0`。
- 断言刷新时间、退避、无 panic。
- `go test -race`。

---

### Workstream C：WS 生命周期修复

**文件**

- `sessions/manager/manager.go`
- `sessions/local/local.go`
- `sessions/remote/remote.go`
- `websocket/client/client.go`
- `dto/websocket.go`（可选：增加 `RetryCount`）

**设计**

1. 新增错误分类：

```go
type Action int
const (
    ActionResume Action = iota
    ActionIdentify
    ActionTerminal
    ActionTransient
)

func Classify(err error) Action
```

映射官方错误码：

| 错误码 | 动作 |
|---|---|
| 4009 | Resume |
| 4006/4007/4008/4900-4913 | Identify |
| 4004 | Identify + ForceRefresh |
| 4014 | Identify + 降级 intents（一次） |
| 4013 | Terminal |
| 4914/4915 | Terminal |
| 4001/4002/4010/4011/4012 | Terminal |
| 网络错误/1006 | Transient |

2. `local.newConnect`：
   - `Connect` 失败 → 重新排队 + 退避。
   - `Identify/Resume` 失败 → 重新排队 + 退避。
   - `Listening` 失败 → `Classify`：
     - Resume：保留 session ID，重新排队。
     - Identify：清空 ID/Seq，重新排队。
     - Terminal：调用错误回调，**不再排队**。
     - Transient：重新排队 + 退避。
   - 去掉 `panic`。
   - 保留 defer recover，但 recover 后不再无条件重排队；根据错误分类决定。

3. `remote.newConnect`：
   - 与 local 相同。
   - 增加 4014 降级。
   - 去掉 `panic`。
   - 确保 shard lock 在终止路径释放。

4. 退避：
   - 使用 `github.com/cenkalti/backoff/v4`，不要手写 1s/2s/4s。
   - 每个 shard 一个 `backoff.BackOff` 实例：
     ```go
     b := backoff.NewExponentialBackOff(
         backoff.WithInitialInterval(1*time.Second),
         backoff.WithMultiplier(2),
         backoff.WithMaxInterval(60*time.Second),
         backoff.WithRandomizationFactor(0.5),
         backoff.WithMaxElapsedTime(0),
     )
     ```
   - `RetryCount` 放在 `dto.Session`（`json:"-"`）或 manager 内部 map。
   - 成功 Identify/Resume 后 `b.Reset()`；或连接稳定 30s 后重置。
   - Terminal 错误不进入重试。

5. seq 保存与去重：
   - 这是最容易影响 SealDice 的行为变化。
   - 先加 WS 事件去重（`ttlcache`，key = `payload.EventID`，TTL 10min，容量 100k）。
   - 去重通过后：
     ```go
     if err := event.ParseAndHandle(payload); err != nil {
         // 不推进 seq；记录失败
     } else {
         c.saveSeq(payload.Seq)
     }
     ```
   - 如果去重关闭，则回退到旧行为（处理前保存 seq），避免重复执行骰子指令。
   - 提供选项：
     ```go
     type SeqPolicy int
     const (
         SeqBeforeHandle SeqPolicy = iota // 旧行为，at-most-once
         SeqAfterHandle                   // 新行为，at-least-once + dedup
     )
     ```
   - 默认：`SeqAfterHandle` + dedup；如果 `Dedup` 被禁用，默认自动回退到 `SeqBeforeHandle`。
   - 残余风险：进程重启后内存去重缓存为空；remote session 从 Redis resume 时可能重放少量旧事件。文档注明，必要时接持久化 dedup。

6. 错误回调：

```go
func SetErrorHandler(h func(appID string, err error, action Action))
```

   - 默认只记日志。
   - `ActionTerminal` 通知上层（SealDice 可以停止该 endpoint）。
   - 详细策略见「8.2 Terminal 错误策略」。

7. 资源释放：
   - Terminal 路径必须释放 shard lock、停止 ticker、关闭连接。
   - 不要 panic，不要让 goroutine 泄漏。
   - 其他 app 不受影响。

8. `Start` 返回值：
   - Terminal 错误必须让 `SessionManager.Start` **返回 error**。
   - 这样 SealDice 现有的 `if startErr := pa.SessionManager.Start(...); startErr != nil` 分支会自动把 `ep.State = 3`、`ep.Enable = false`。
   - 不能只调用 `ErrorHandler` 而让 `Start` 永远阻塞，否则 UI 状态不会更新。
   - 实现：manager 增加 `fatalChan chan error`，`Start` select `sessionChan` / `ctx.Done()` / `fatalChan`。

**测试**

- 假 gateway 模拟各错误码。
- 断言 Resume/Identify/Terminal 行为。
- 断言不会 panic。
- 断言退避序列。
- `go test -race`。

---

### Workstream D：Webhook Handler 修复

**文件**

- `interaction/webhook/webhook.go`
- `interaction/webhook/handler.go`（新增）
- `interaction/webhook/webhook_test.go`（扩展）

**设计**

1. 保留 `HTTPHandler` 签名，内部改为：
   - `io.ReadAll` / `MaxBytesReader`。
   - 先解析 `dto.WSPayload`。
   - `op == 13`：解析 `plain_token`/`event_ts`，`GenValidationACK`，返回 200。
   - 其他：`signature.Verify`。
   - `op == 0`：`event.ParseAndHandle`，带 panic recover；成功 `GenDispatchACK(true)`，失败 `GenDispatchACK(false)`。
   - `op == 1`：`GenHeartbeatACK`。
   - 未知 op：400 或 200 空。
   - 设置 `Content-Type: application/json`。
   - 失败返回 400/401/500。

2. 新增 `Handler`：

```go
type Handler struct {
    Credentials *token.QQBotCredentials
    Deduper     Deduper
    MaxBodyBytes int64
    AckMode     AckMode
    ErrorHandler func(error)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

   - `AckModeSync`：默认，处理完再 ACK。
   - `AckModeAsync`：先 ACK，再投递 worker；失败只记日志（因为已经 ACK）。
   - `AckModeTimeout`：带超时处理；超时返回 `d=1`，可选后台继续。

3. 去重：

```go
type Deduper interface {
    Seen(eventID string) bool
}
```

   - 默认实现用 `github.com/jellydator/ttlcache/v3`：
     ```go
     cache := ttlcache.New[string, struct{}](
         ttlcache.WithTTL[string, struct{}](10*time.Minute),
         ttlcache.WithCapacity[string, struct{}](100_000),
     )
     ```
   - `Seen` 用 `Has` + `Set`；`cache.Start()` 在独立 goroutine 跑自动清理。
   - 按 `payload.EventID` 去重。
   - 重复事件直接 ACK 成功，不投递。

4. panic 恢复：
   - handler 包装 `recover`，返回 `GenDispatchACK(false)` 或 500。

**测试**

- 校验请求无签名 → 正确返回 `plain_token/signature`。
- 合法签名事件 → `{"op":12,"d":0}`。
- 非法签名 → 401。
- 重复 EventID → handler 只调用一次。
- panic handler → `d=1` / 500。
- 大 body → 413。
- 未知 op。
- `httptest` 全覆盖。

---

### Workstream E：Webhook Server

**文件**

- `interaction/webhook/server.go`（新增）
- `interaction/webhook/dedup.go`（新增）
- `interaction/webhook/server_test.go`（新增）

**设计**

```go
type CredentialsResolver func(appID string) (*token.QQBotCredentials, error)

type Server struct {
    Addr        string
    Path        string
    TLSConfig   *tls.Config
    Resolve     CredentialsResolver
    Deduper     Deduper
    MaxBodyBytes int64
    AckMode     AckMode
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
    ErrorHandler func(error)
}

func NewServer(opts ServerOptions) *Server
func (s *Server) Handler() http.Handler
func (s *Server) ListenAndServe() error
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error
func (s *Server) Shutdown(ctx context.Context) error
```

- 从 `X-Bot-Appid` 或路径 `/qqbot/{appid}` 解析 appid。
- 解析失败 → 400/401。
- 校验 header 与 resolver 返回的 credentials 一致。
- 支持 HTTPS。
- 支持优雅关闭。
- 端口限制在文档中说明（官方只允许 80/443/8080/8443）。
- 可观测性：记录 traceID、事件类型、耗时、结果。

---

### Workstream F：BB 协议/生命周期融合

#### F1. 事件钩子

- `event/event.go`：
  - `RegisterFilter(func(*dto.WSPayload) bool)`：返回 false 则丢弃。
  - `RegisterObserver(func(*dto.WSPayload))`：dispatch 前后观察。
  - 保持 `ParseAndHandle` 签名不变。
- 用于 gateway 转发、指标、日志、去重前置。

#### F2. 主动消息事件

- `dto/group_event.go`：
  - `WSGroupMsgReceiveData`
  - `WSGroupMsgRejectData`
- `dto/c2c.go` 或新文件：
  - `WSC2CMsgReceiveData`
  - `WSC2CMsgRejectData`
- `event/register.go`：新增 handler 类型和注册。
- `event/event.go`：新增 map 项。
- `dto/websocket_event.go`：新增 event type，加入 `IntentGroupMessages`（1<<25）。

#### F3. 群 API（按官方文档，不照搬 BB）

- `GetGroupMembers(ctx, groupOpenID, cursor string) (*dto.GroupMembersResp, error)`
  - `GET /v2/groups/{group_openid}/members?cursor=...`
  - 返回 `members[]`、`next_cursor`。
- `GetGroupMember(ctx, groupOpenID, memberOpenID string)`.
- `GetGroupInfo(ctx, groupOpenID)`.
- `GetBotState(ctx, groupOpenID)`.

#### F4. 消息/文件 API

- `PostFormFileImage` / `PostFormFileReaderImage`（频道图片表单上传）。
- `PostStreamMessage`（`POST /v2/users/{user_openid}/stream_messages`）。
- `GROUP_JOIN_REQUEST` 事件。

#### F5. 正向 WS Gateway（可选包）

- 新包 `gateway/`。
- 直接使用 gws 高级 API：
  - `gws.NewUpgrader` + `ServerOption`。
  - `ServerOption.Authorize`：鉴权，不用自己写握手。
  - `ServerOption.NewSession` + `SessionStorage`：存 appID/clientID/订阅。
  - `gws.Broadcaster`：广播只压缩一次。
  - `WriteAsync` / `WritevAsync`：非阻塞写。
  - `SetWriteDeadline`：慢客户端保护。
- `Hub` 按 appID 管理客户端。
- `Broadcast(appID, payload)`。
- 鉴权：`Authorization: Bearer <token>` 或 query token。
- 帧格式：见「8.1 Gateway 帧格式」。
- 背压：每客户端有界队列；慢客户端丢弃/关闭。
- 生命周期：ping/pong、write deadline、优雅关闭。
- 与 `event.RegisterObserver` 集成。
- 不移植 BB 的 `WaitingFrames`。

---

### Workstream G：SealDice 集成

**现状**

SealDice 只有两个文件直接依赖 botgo：

- `dice/platform_adapter_official_qq.go`
- `dice/platform_adapter_official_qq_test.go`

使用的 API：

- `qqbot.SetLogger`
- `qqtoken.NewQQBotTokenSource`
- `qqtoken.StartRefreshAccessToken`
- `qqbot.NewOpenAPI`
- `qqbot.NewSessionManager`
- `SessionManager.Start`
- `event.RegisterHandlersByAppID`
- `event.ParseAndHandle`
- `signature.Verify`
- `Api.WS` / `PostMessage` / `PostGroupMessage` / `PostC2CMessage` / `PostGroupFile` / `PostDirectMessage` / `CreateDirectMessage` / `PutInteraction` / `Transport`

这些签名全部保持不变。

**需要确认/处理的三点**

1. **Terminal 错误**
   - `SessionManager.Start` 必须返回 terminal error。
   - SealDice 现有代码会据此设置 `ep.State = 3`、`ep.Enable = false`。
   - 不需要改 SealDice 适配层。

2. **seq/去重**
   - 默认 `SeqAfterHandle` + dedup。
   - 如果去重关闭，回退 `SeqBeforeHandle`，避免重复执行骰子指令。
   - SealDice 不需要改，但必须做“断线重放不重复执行”的集成测试。

3. **Webhook**
   - SealDice 的 `handleWebhookCallback` 是适配层自己实现的，不走 SDK 的 `interaction/webhook.HTTPHandler`。
   - SDK 修好 webhook 不会自动修好 SealDice 的 webhook。
   - 需要一个小补丁：把 `handleWebhookCallback` 改为使用新的 `webhook.Handler`，或手动补 op 13 / dedup / panic recover。
   - 这是 **recommended**，不是编译必需。

**依赖兼容性**

- SealDice 已经通过 `pineutil/evsocket` 间接依赖 `gws v1.9.1`。
- 已在临时 worktree 验证：升级到 `gws v1.10.2` 后 `go build ./dice` 和 OfficialQQ 测试通过。
- `cenkalti/backoff/v4`、`jellydator/ttlcache/v3` 目前不在 SealDice 依赖图中，新增无冲突。
- `gorilla/websocket` 在 SealDice 里还有其他适配器使用，移除 botgo 的 gorilla 依赖不会影响它们。
- SealDice 使用 Go 1.25.14，满足 Go 1.20 要求。

**验证**

- [ ] 临时 worktree：`replace github.com/sealdice/botgo => ../botgo`。
- [ ] `go build ./dice`。
- [ ] `go test ./dice -run OfficialQQ`。
- [ ] 假 gateway 断线重放测试：确认骰子指令不重复执行。
- [ ] 假 webhook op 13 / 签名 / ACK 测试。
- [ ] 沙箱机器人 24h canary。

---

## 4. 里程碑

### M0：基线

- [x] 跑通 `go test ./...`（Redis 测试可跳过）。
- [x] 建立假 gateway / 假 webhook 测试骨架。
- [x] 确认 gws v1.10.2 API。

### M1：gws 客户端替换

- [x] `websocket/client` 内部改为 gws。
- [x] 保持 `websocket.WebSocket` 接口和 `Setup()` 不变。
- [x] 基础连接/Identify/Ready/事件/心跳测试通过。
- [x] 移除 gorilla 依赖。

### M2：Token + WS 生命周期

- [x] 60s 刷新、退避、无 panic。
- [x] ForceRefresh + 11244 重试。
- [x] WS 事件去重（ttlcache）。
- [x] seq 保存时机 + SeqPolicy。
- [x] `Start` 返回 terminal error。
- [x] Identify/Resume 失败重排队。
- [x] 错误码分类 + Terminal 处理。
- [x] 4014 降级 local/remote 一致。
- [x] 连接退避。
- [x] `go test -race`。

### M3：Webhook Handler

- [x] `io.ReadAll`、op 13 顺序、状态码。
- [x] panic 恢复、去重、ACK 模式。
- [x] handler 测试。

### M4：Webhook Server

- [x] 多账号 resolver。
- [x] TLS、优雅关闭、请求限制。
- [x] server 测试。

### M5：BB 融合

- [x] Observer/Filter。
- [x] 主动消息事件。
- [x] 群 API / 文件 API / Stream API。
- [x] Gateway 包。
- [x] `GROUP_JOIN_REQUEST`。

### M6：集成与回归

- [x] 假 QQ gateway 端到端测试。
- [x] 假 webhook 端到端测试。
- [x] SealDice `replace` 构建与官方 QQ 测试。
- [ ] SealDice 断线重放测试（骰子指令不重复执行）。
- [ ] SealDice webhook 适配层切换到新 `webhook.Handler`（recommended）。
- [ ] 沙箱 canary。
- [ ] 文档更新。

---

## 5. 测试策略

### 单元测试

- token：刷新、退避、过期、错误码、并发。
- WS：close code 分类、seq、重连、ctx 取消。
- webhook：签名、校验、ACK、去重、panic、限制。
- gateway：广播、鉴权、背压、关闭。
- event：过滤器、观察者、新事件。

### 集成测试

- 本地 gws server 模拟 QQ gateway：
  - Hello/Identify/Ready。
  - 事件推送。
  - Heartbeat。
  - Resume。
  - 4009/4006/4914。
- `httptest` 模拟 webhook：
  - op 13 校验。
  - 签名事件。
  - 重复事件。
  - 慢 handler。
- SealDice：
  - `replace github.com/sealdice/botgo => ../botgo`。
  - 跑官方 QQ 适配测试。

### 质量门禁

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `golangci-lint`（如项目已有配置）
- 公共 API 兼容性检查（编译一个最小消费者）。

---

## 6. 风险与对策

| 风险 | 对策 |
|---|---|
| gws 与 gorilla 的 close code 语义不同 | 增加 close code 兼容层 + 测试 |
| gws 回调并发/顺序不同 | `ParallelEnabled: false`，保持事件队列顺序 |
| gws 要求 Go 1.20 | release note 注明；SealDice 已是 1.25 |
| 移除 gorilla 影响下游 | 公共 API 不暴露 gorilla，风险低 |
| 异步 ACK 改变语义 | 默认同步，异步 opt-in |
| 去重内存增长 | `ttlcache` TTL + `WithCapacity` 上限 |
| Terminal 错误停止 manager | 通过错误回调让上层决定是否重启 |
| 退避导致恢复慢 | `backoff/v4` 初始 1s、上限 60s、可配置 |
| 多账号共享 token source | 每个 app 一个 token source，测试并发 |
| 4014 降级不完整 | 记录降级后的 intent，二次失败则 Terminal |
| 新增依赖过多 | 只引入 gws、backoff、ttlcache 三个运行时依赖 |

---

## 7. 验收标准

- [x] `websocket.WebSocket`、`event.RegisterHandlers`、`interaction/webhook.HTTPHandler` 等 SealDice 使用的公共 API 签名不变（`OpenAPI` 接口有新增方法，对实现者是可加性破坏）。
- [x] `go.mod` 提升 Go 版本，增加 gws/backoff/ttlcache，移除 gorilla。
- [x] 假 gateway 覆盖 Identify/Resume/心跳/事件/4009/4006/4914。
- [x] token 刷新有 60s 提前量、退避、无 panic、ForceRefresh。
- [x] webhook op 13 无签名也能正确校验。
- [x] webhook 合法事件返回 op 12 ACK。
- [x] webhook 去重、panic、状态码、请求限制测试通过。
- [ ] `go test -race ./...` 通过。
- [x] SealDice 使用 `replace` 可构建并通过官方 QQ 相关测试。
- [ ] 文档更新：迁移说明、Go 版本、行为变化、配置项。

---

## 8. 关键设计说明

### 8.1 Gateway 帧格式

**“Gateway 帧格式”指的是：SDK 作为 WebSocket 服务端，把 QQ 事件推给外部客户端时，WebSocket 文本帧里那段 JSON 的结构。**

它不是腾讯的协议，也不是 QQ 官方 payload 的替代，而是 SDK 自己定义的“转发信封”。

当前有三类可选方案：

#### 方案 A：原始官方 payload

直接转发 `dto.WSPayload`：

```json
{
  "id": "event_id",
  "op": 0,
  "s": 42,
  "t": "GROUP_AT_MESSAGE_CREATE",
  "d": { "content": "1d100", "...": "..." }
}
```

优点：

- 与官方文档完全一致，客户端可直接复用官方 DTO。
- 不增加任何翻译层。
- 调试时和 QQ 后台日志一致。

缺点：

- 多账号时不知道事件来自哪个 appID。
- 没有版本号、控制帧、错误帧的扩展位。
- 客户端要自己推断连接对应的账号。

#### 方案 B：版本化信封（推荐）

```json
{
  "v": 1,
  "appid": "123456",
  "type": "event",
  "id": "event_id",
  "ts": 1784276800,
  "payload": {
    "id": "event_id",
    "op": 0,
    "s": 42,
    "t": "GROUP_AT_MESSAGE_CREATE",
    "d": { "...": "..." }
  }
}
```

控制帧：

```json
{"v":1,"type":"hello","appid":"123456","heartbeat_interval":30000}
{"v":1,"type":"ack","id":"event_id"}
{"v":1,"type":"error","code":"...","message":"..."}
```

优点：

- 多账号天然可区分。
- 有版本号，后续可演进。
- 控制帧、错误帧、心跳帧有统一位置。
- 客户端仍然可以直接读 `payload`，复用官方 DTO。

缺点：

- 客户端需要多解一层信封。
- 与 BB 的 `onebot.Frame` 不兼容。

#### 方案 C：兼容 BB 的 `onebot.Frame`

BB 的帧长这样：

```json
{
  "bot_id": "123456",
  "echo": "1",
  "ok": true,
  "time": 1784276800,
  "self_id": "123456",
  "data": "...",
  "payload": { "...": "..." }
}
```

优点：

- 可以直接复用 BB 的 `Bot-Client-Go` 客户端。

缺点：

- 字段重复（`data` + `payload`）。
- `echo/ok` 是请求-响应语义，但 BB 的 `WaitingFrames` 实际没有 resolve，属于半成品。
- 名字叫 `onebot`，但不是标准 OneBot 协议，容易误导。
- 不建议作为默认格式。

#### 推荐

- 默认使用 **方案 B：版本化信封**。
- 提供 `FrameMode` 选项：
  - `FrameModeEnvelope`（默认）
  - `FrameModeRaw`（只发官方 payload）
  - `FrameModeBB`（兼容 BB，仅在有明确客户端需求时启用）
- 入站客户端帧只支持最小控制集：
  - `{"type":"ping"}`
  - `{"type":"subscribe","appids":["..."]}`
  - `{"type":"ack","id":"..."}`（可选）
- 入站业务帧不转发到 QQ，避免把 gateway 做成任意 RPC。

### 8.2 Terminal 错误策略

**Terminal 的定义：继续用同样的凭据、同样的 intent、同样的配置重连，也不会成功的错误。**

继续重连的后果：

- 对平台：形成无效连接风暴，可能触发限流/封禁。
- 对进程：当前实现会 panic。local 被 recover 后又把 session 放回队列，形成热循环；remote 没有 recover，直接崩进程。
- 对业务：骰子整体掉线，其他正常账号也被拖累。

#### 错误分类与动作

| 错误码 | 含义 | 动作 | 是否 Terminal |
|---|---|---|---|
| 4009 | session 过期 | Resume | 否 |
| 4006 | session 无效 | Identify | 否 |
| 4007 | seq 错误 | Identify | 否 |
| 4008 | 发送过快 | Resume / Identify + 退避 | 否 |
| 4004 | 鉴权失败 | ForceRefresh + Identify 一次 | 一次失败后 Terminal |
| 4013 | 无效 intent | 停止，等待配置修正 | 是 |
| 4014 | intent 无权限 | 降级 intent + Identify 一次 | 一次失败后 Terminal |
| 4914 | 机器人已下架/只允许沙箱 | 停止，等待环境修正 | 是 |
| 4915 | 机器人被封禁 | 停止，等待解封 | 是 |
| 4001/4002 | 协议错误 | 停止 | 是 |
| 4010/4011/4012 | shard/分片/版本错误 | 停止 | 是 |
| 4900-4913 | 内部错误 | Identify + 退避 | 否 |
| 网络错误/1006 | 连接断开 | 退避重连 | 否 |

#### 推荐行为

1. **Terminal 时停止“该 app 的 manager”**，不是只停当前 shard。
   - 原因：4914/4915/4013/4014 是账号级/配置级错误，不是某个 shard 的问题。
   - 继续重试其他 shard 只会放大无效连接。
   - SealDice 每个 endpoint 有自己的 manager/token source，停止该 manager 不会影响其他 endpoint。

2. **不 panic。**
   - 释放 shard lock、停止 ticker、关闭连接。
   - 调用 `ErrorHandler(appID, err, ActionTerminal)`。
   - 进程保持存活，其他 app 继续工作。

3. **保留可重启能力。**
   - manager 进入 `Stopped` 状态。
   - 上层修正配置后，重新 `New`/`Start` 即可。
   - 可选提供 `Done() <-chan error` 或 `LastError()` 供上层观察。

4. **4014 特殊处理。**
   - 第一次：从 intent 中移除已知高风险位（`IntentGroupMembers`、`IntentEnterAIO` 等），重新 Identify。
   - 第二次仍然 4014：Terminal。
   - 降级后的 intent 要记录到日志，方便定位。

5. **4004 特殊处理。**
   - 先 `ForceRefresh()` 拿新 token。
   - 重新 Identify 一次。
   - 仍 4004：Terminal（可能是 app secret 错/机器人被封）。

6. **Transient 退避。**
   - 使用 `cenkalti/backoff/v4`。
   - 初始 1s，最大 60s，jitter 0.5。
   - 成功 Identify/Resume 后 `Reset()`。

#### 为什么不是“只停当前 shard”

- 4009/4006/4007/4008/4900-4913 是连接/session 级错误，停 shard 重连即可。
- 4013/4014/4914/4915 是账号/配置级错误，所有 shard 都会失败。
- 只停 shard 会导致其余 shard 继续撞墙，日志刷屏，且无法真正恢复。
- 因此：**连接级错误按 shard 处理，账号级错误按 app 处理。**

---

## 9. 需要确认的决策

1. **gws 版本**：`v1.10.2`。已确认。
2. **Go 版本**：升到 1.20。已确认。
3. **ACK 默认模式**：建议默认同步（保持现有行为），异步 opt-in。待确认。
4. **Gateway 帧格式**：建议默认 `FrameModeEnvelope`，保留 `FrameModeRaw`；`FrameModeBB` 仅按需启用。待确认。
5. **Terminal 错误策略**：建议停止该 app 的 manager，不 panic，可重启；连接级错误按 shard 处理。待确认。
6. **是否保留 gorilla**：直接移除。已确认。

---

## 10. 备注

- 这个仓库是从 Go module proxy 拉取的源码快照，`git` 历史从导入提交开始。
- 原始仓库地址：`https://github.com/sealdice/botgo`。
- 当前基线测试除 Redis 依赖外通过。

---

## 11. 实施结果（本地）

### 已完成

- gws 替换 gorilla/websocket，`websocket.WebSocket` 接口和 `Setup()` 保持不变。
- Token：60s 提前量、`backoff/v4` 指数退避、去掉 panic、`Invalidate/ForceRefresh`、11244 重试。
- WS 生命周期：seq 策略 + `ttlcache` 去重、Identify/Resume 重排队、错误码分类、Terminal 返回 error、4014 降级、连接退避。
- Webhook：`io.ReadAll`、op 13 先于验签、状态码、panic 恢复、去重、ACK 模式、`Handler`/`Server`。
- BB 融合：Observer/Filter、主动消息事件、群 API、FormFile/Stream API、`GROUP_JOIN_REQUEST`、Gateway。
- 测试：token、WS client、webhook handler/server、manager classify、local terminal、gateway broadcast。
- `go test -race`（非 Redis 包）通过。
- SealDice 临时 worktree：
  - `replace github.com/sealdice/botgo => /home/pinenut/GolandProjects/botgo`
  - `go build ./...` 通过
  - `go test ./dice` 通过（约 82s）
  - `go test ./dice -run OfficialQQ` 通过
  - **SealDice 没有任何 `.go` 源码改动，只有 `go.mod`/`go.sum` 依赖更新。**

### 未完成

- SealDice 断线重放集成测试（骰子指令不重复执行）。
- SealDice webhook 适配层切换到新 `webhook.Handler`（不改 SealDice 代码则无法自动生效）。
- 真实 QQ 沙箱 24h canary。
- 文档/release note。

### 补充修复：Redis 多实例 session 序列化

- `dto.Session.TokenSource` 增加 `json:"-"`。
- `sessions/remote.consume` 在反序列化后从本地配置注入 `TokenSource` 和 `AppID`。
- 原因：原实现会把 `TokenSource` 序列化成 `{}`，反序列化时报
  `cannot unmarshal object into Go struct field Session.TokenSource of type oauth2.TokenSource`，
  导致 remote manager 实际无法消费 session。
- 新增 `dto/session_test.go` 覆盖该行为。

### 第二轮 review 修复（含文档核对）

1. **`go vet` 重复 JSON tag（dto/message_media.go）——已核对官方文档并修复**
   - 根因：`MediaMessage` 同时嵌入 `Message`（含 `file_info`/`ttl`）和 `Media`（含 `file_info`/`ttl`），
     合成结构里 JSON key 冲突。该类型全仓库无人使用，属于官方遗留死代码。
   - 修复：删除 `MediaMessage`；`go vet ./...` 现在完全干净。
   - 顺带按 2026-10 官方文档补齐富媒体 DTO：
     - `MessageMediaToCreate` 增加 `upload_id`（分片上传合并）、`file_name`。
     - `Media` 增加 `id`（srv_send_msg=true 时返回）、`raw_url`（分片上传的预签名下载链接）。
   - 文档核对结论：官方事件体（如 GROUP_AT_MESSAGE_CREATE）目前**不含** `file_info`/`ttl`，
     `Message` 上这两个字段是官方 SDK v0.2.1 的遗留声明，因带 `omitempty` 无害，保留不动。

2. **ticker 数据竞争**：新增 `tickerMu` 保护 `heartBeatTicker.Reset/Stop` 与 `transportPingTicker.Stop`；
   transport ticker 改在 `New` 创建，消除 `Listening` 写路径。

3. **4004 重试一次后 Terminal**：`dto.Session` 新增 `AuthRetryCount`（`json:"-"`），
   local/remote 对 4004 首次强刷重试、二次 Terminal；连接稳定 30s 后清零。
   新增 `sessions/local/auth_retry_test.go` 验证恰好重试一次。

4. **全局选项线程安全**：`websocket/client.SetDefaultOptions`、`token.SetRefreshErrorHandler`
   改为 RWMutex 保护的读写。

5. **`OpenAPI` 接口兼容性**：撤销对 `OpenAPI`/`MessageAPI` 的方法新增。
   `GroupAPI` / `FormFileAPI` / `StreamMessageAPI` 改为独立可选接口，
   使用方类型断言获取；具体实现侧加编译期断言。第三方 `openapi.OpenAPI` 实现者不再被破坏。

### golangci-lint 配置适配与清理

- 引入 `sealdice-core` 的 `.golangci.toml`（golangci-lint v2 格式）。
- 适配项：
  - `goimports.local-prefixes` → `github.com/sealdice/botgo`
  - `[run] go = '1.20'`、`timeout = '10m'`
  - 关闭 `testpackage`（botgo 测试需要同包访问）
  - 关闭 `gochecknoinits`（SDK 用 init 做注册）
  - 关闭 `copyloopvar` / `intrange`（要求 Go 1.22+）
  - 移除 `math/rand$` 的 depguard 规则（Go 1.20 无 math/rand/v2）
  - `exhaustive.check` 修正为 `['switch', 'map']`
  - 为历史公开命名/控制台输出增加定向 exclusion（`errs.Err`、`ErrorNotOk`、`log/console.go`）
- 修复了全部 lint 问题（原 89 个）：
  - errorlint `%v` → `%w`
  - `errs.Error` 改用 `errors.As`
  - `redis.Nil` 改用 `errors.Is`
  - 删除未使用的 resource 常量与 `isCloseError`
  - `ioutil` → `io`
  - 删除死代码 `MediaMessage`，补齐富媒体 DTO 新字段
  - gocritic assignOp、gofmt/goimports、testifylint、unconvert、perfsprint 等
  - 必要的 `//nolint:...` 均带解释（bodyclose/reassign/nilerr/exhaustive）
- 验证：
  - `golangci-lint run ./...` → **0 issues**
  - `go vet ./...` → clean
  - `go test -short ./...` → 全绿
  - `go test -race -short <核心包>` → 全绿
  - SealDice `go build ./...` + `go test ./dice -run OfficialQQ` → 通过

---

## 12. QQ 官方 changelog 对齐（2026-08 ~ 2026-09）

### 20260916

- `/v2/generate_url_link`：新增 `ShareAPI.GenerateURLLink`，请求 `callback_data`，响应按新文档使用 `data.url`。
- `ark_type`：新增 `dto.ARKData` 与 `ArkTypePicture = "picture"`，并补齐 tuwen/feed/miniapp/map/contact_card/video_share/music_together 常量。

### 20260903

- `GroupAPI.GetGroupMembers`：GET + cursor/next_cursor。
- `GroupAPI.GetGroupMember`：GET 单个成员。
- `GroupManageAPI.BatchRemoveMembers`：POST 批量移除。
- `GroupManageAPI.GetGroupBlacklist`：GET 黑名单查询。
- `GroupManageAPI.UpdateGroupBlacklist`：POST add/del 黑名单。

### 20260812

- `MenuAPI.GetMenu` / `PutMenu`：GET/PUT `/v2/menu`。
- `PanelAPI`：List/Create/Get/Update/Delete/UpdateTarget，覆盖 `/v2/panels` 与 `/v2/panels/{panel_id}`。

### 20260810

- 域名统一：`constant.APIDomain` 与 `TokenDomain` 均为 `https://api.bot.qq.com`；sandbox 保持官方 `https://sandbox.api.sgroup.qq.com`。
- Markdown：`dto.Markdown.ForceVerifyImageResource`。
- 群禁言：`GroupManageAPI.GetGroupMuteSetting` / `SetGroupMute`。
- 入群申请：`JoinRequestAPI.ListJoinRequests` / `ApproveJoinRequest`。
- 入群自动审批策略：`JoinApprovalStrategyAPI` 的列表/创建/修改/删除/执行/白名单六个方法。
- 用户入群申请事件：`EventGroupJoinRequest` + `WSGroupJoinRequestData` + handler + intent。

新接口全部以**可选接口**提供，不修改现有 `OpenAPI`，避免破坏第三方实现者。
