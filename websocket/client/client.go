// Package client 默认的 websocket client 实现。
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/lxzan/gws"
	"golang.org/x/net/proxy"
	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/event"
	"github.com/sealdice/botgo/internal/dedup"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/websocket"
)

const (
	// DefaultQueueSize 监听队列的缓冲长度
	DefaultQueueSize = 10000

	defaultHandshakeTimeout   = 10 * time.Second
	defaultReadMaxPayloadSize = 8 << 20 // 8 MiB
	defaultWriteTimeout       = 10 * time.Second
	defaultPingInterval       = 30 * time.Second
	defaultPingWait           = 10 * time.Second
)

// SeqPolicy 控制 seq 的保存时机。
type SeqPolicy int

const (
	// SeqPolicyDefault 使用默认策略（SeqAfterHandle，未开启去重时回退到 SeqBeforeHandle）。
	SeqPolicyDefault SeqPolicy = iota
	// SeqBeforeHandle 旧行为：处理前保存 seq，at-most-once，可能丢事件但不会重复执行。
	SeqBeforeHandle
	// SeqAfterHandle 新行为：处理成功后保存 seq，配合去重实现 at-least-once。
	SeqAfterHandle
)

// ClientOptions websocket 客户端选项。
type ClientOptions struct {
	// SeqPolicy 控制 seq 保存时机。默认 SeqAfterHandle。
	SeqPolicy SeqPolicy
	// DedupEnabled 是否开启事件去重。默认 true。
	DedupEnabled bool
	// DedupTTL 去重缓存 TTL。默认 10 分钟。
	DedupTTL time.Duration
	// DedupCapacity 去重缓存最大条目数。默认 100000。
	DedupCapacity uint64
}

var (
	optionsMu            sync.RWMutex
	defaultClientOptions = ClientOptions{
		SeqPolicy:     SeqAfterHandle,
		DedupEnabled:  true,
		DedupTTL:      10 * time.Minute,
		DedupCapacity: 100000,
	}
)

// SetDefaultOptions 设置新建客户端使用的默认选项。
// 建议在创建连接之前调用；运行期修改只影响之后创建的客户端。
func SetDefaultOptions(opts ClientOptions) {
	if opts.DedupTTL <= 0 {
		opts.DedupTTL = 10 * time.Minute
	}
	if opts.DedupCapacity == 0 {
		opts.DedupCapacity = 100000
	}
	if opts.SeqPolicy == SeqPolicyDefault {
		opts.SeqPolicy = SeqAfterHandle
	}
	if !opts.DedupEnabled {
		// 没有去重时不能安全地使用 after-handle，否则断线重放可能重复执行。
		opts.SeqPolicy = SeqBeforeHandle
	}
	optionsMu.Lock()
	defaultClientOptions = opts
	optionsMu.Unlock()
}

func currentOptions() ClientOptions {
	optionsMu.RLock()
	defer optionsMu.RUnlock()
	return defaultClientOptions
}

type messageDeduper struct {
	cache *ttlcache.Cache[string, struct{}]
	ttl   time.Duration
}

func newMessageDeduper(ttl time.Duration, capacity uint64) *messageDeduper {
	cache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](ttl),
		ttlcache.WithCapacity[string, struct{}](capacity),
	)
	go cache.Start()
	return &messageDeduper{cache: cache, ttl: ttl}
}

func (d *messageDeduper) Seen(eventID string) bool {
	if d == nil || eventID == "" {
		return false
	}
	return d.cache.Has(eventID)
}

func (d *messageDeduper) Mark(eventID string) {
	if d == nil || eventID == "" {
		return
	}
	d.cache.Set(eventID, struct{}{}, d.ttl)
}

func (d *messageDeduper) Unmark(eventID string) {
	if d == nil || eventID == "" {
		return
	}
	d.cache.Delete(eventID)
}

func (d *messageDeduper) Stop() {
	if d != nil && d.cache != nil {
		d.cache.Stop()
	}
}

// Setup 依赖注册
func Setup() {
	websocket.Register(&Client{})
}

// New 新建一个连接对象
func (c *Client) New(ctx context.Context, session dto.Session) websocket.WebSocket {
	opts := currentOptions()
	seqPolicy := opts.SeqPolicy
	if seqPolicy == SeqPolicyDefault {
		seqPolicy = SeqAfterHandle
	}
	if !opts.DedupEnabled {
		seqPolicy = SeqBeforeHandle
	}
	client := &Client{
		messageQueue:        make(messageChan, DefaultQueueSize),
		session:             &session,
		closeChan:           make(closeErrorChan, 10),
		heartBeatTicker:     time.NewTicker(60 * time.Second), // 先给一个默认 ticker，在收到 hello 包之后，会 reset
		transportPingTicker: time.NewTicker(defaultPingInterval),
		ctx:                 ctx,
		queueDone:           make(chan struct{}),
		writeTimeout:        defaultWriteTimeout,
		pingInterval:        defaultPingInterval,
		pingWait:            defaultPingWait,
		seqPolicy:           seqPolicy,
	}
	if opts.DedupEnabled {
		client.deduper = newMessageDeduper(opts.DedupTTL, opts.DedupCapacity)
	}
	client.handler = &gwsEventHandler{client: client}
	return client
}

// Client websocket 连接客户端
type Client struct {
	version             int
	conn                *gws.Conn
	handler             *gwsEventHandler
	messageQueue        messageChan
	session             *dto.Session
	user                *dto.WSUser
	closeChan           closeErrorChan
	heartBeatTicker     *time.Ticker // 用于维持 QQ 协议的定时心跳
	transportPingTicker *time.Ticker // 用于维持 WebSocket 传输层 ping/pong
	ctx                 context.Context
	queueDone           chan struct{}
	queueDoneOnce       sync.Once
	closeOnce           sync.Once
	tickerMu            sync.Mutex // 保护 ticker 的 Reset/Stop，OnMessage 与 Listening/Close 可能跨协程
	writeTimeout        time.Duration
	pingInterval        time.Duration
	pingWait            time.Duration
	seqPolicy           SeqPolicy
	deduper             *messageDeduper
}

type messageChan chan *dto.WSPayload
type closeErrorChan chan error

// Connect 连接到 websocket
func (c *Client) Connect() error {
	if c.session.URL == "" {
		return errs.ErrURLInvalid
	}

	//nolint:bodyclose // WebSocket 101 响应体由升级后的连接接管
	conn, _, err := gws.NewClient(c.handler, &gws.ClientOption{
		Addr:               c.session.URL,
		HandshakeTimeout:   defaultHandshakeTimeout,
		ReadMaxPayloadSize: defaultReadMaxPayloadSize,
		ParallelEnabled:    false,
		Recovery:           gws.Recovery,
		Logger:             gwsLogger{},
		NewDialer: func() (gws.Dialer, error) {
			return proxy.FromEnvironment(), nil
		},
	})
	if err != nil {
		log.Errorf("%s, connect err: %v", c.session, err)
		return err
	}
	c.conn = conn
	log.Infof("%s, url %s, connected", c.session, c.session.URL)
	return nil
}

// Listening 开始监听，会阻塞进程，内部会从事件队列不断的读取事件，解析后投递到注册的 event handler。
// 定时心跳也在这里维护。
func (c *Client) Listening() error {
	defer c.Close()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				websocket.PanicHandler(r, c.session)
				c.signalClose(fmt.Errorf("read loop panic: %v", r))
			}
		}()
		c.conn.ReadLoop()
	}()
	go c.listenMessageAndHandle()

	// transportPingTicker 在 New 中创建，Stop 统一由 Close 持锁执行。

	// 接收 resume signal
	resumeSignal := make(chan os.Signal, 1)
	if websocket.ResumeSignal >= syscall.SIGHUP {
		signal.Notify(resumeSignal, websocket.ResumeSignal)
		defer signal.Stop(resumeSignal)
	}

	for {
		select {
		case <-c.ctx.Done():
			return nil
		case <-resumeSignal: // 使用信号量控制连接立即重连
			log.Infof("%s, received resumeSignal signal", c.session)
			return errs.ErrNeedReConnect
		case err := <-c.closeChan:
			return c.classifyCloseError(err)
		case <-c.heartBeatTicker.C:
			log.Debugf("%s listened heartBeat", c.session)
			heartBeatEvent := &dto.WSPayload{
				WSPayloadBase: dto.WSPayloadBase{
					OPCode: dto.WSHeartbeat,
				},
				Data: c.session.LastSeq,
			}
			// 不处理错误，Write 内部会处理，如果发生发包异常，会通知主协程退出
			_ = c.Write(heartBeatEvent)
		case <-c.transportPingTicker.C:
			// WebSocket 传输层心跳；控制帧也必须显式设置写 deadline，
			// 否则会沿用上一次业务 Write 留下的已过期 deadline。
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
			_ = c.conn.WritePing(nil)
		}
	}
}

// Write 往 ws 写入数据
func (c *Client) Write(message *dto.WSPayload) error {
	m, _ := json.Marshal(message)
	if message.OPCode == dto.WSIdentity || message.OPCode == dto.WSResume {
		// Identify/Resume 的 payload 内含 access_token，不能原文写日志。
		log.Infof("%s write %s message (token redacted)", c.session, dto.OPMeans(message.OPCode))
	} else {
		log.Infof("%s write %s message, %v", c.session, dto.OPMeans(message.OPCode), string(m))
	}

	if c.conn == nil {
		return net.ErrClosed
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	if err := c.conn.WriteMessage(gws.OpcodeText, m); err != nil {
		log.Errorf("%s WriteMessage failed, %v", c.session, err)
		c.signalClose(err)
		return err
	}
	return nil
}

// Resume 重连
func (c *Client) Resume() error {
	token, err := c.session.TokenSource.Token()
	if err != nil {
		log.Errorf("[resume] get access token failed:%s", err)
		return err
	}
	payload := &dto.WSPayload{
		Data: &dto.WSResumeData{
			Token:     token.AccessToken,
			SessionID: c.session.ID,
			Seq:       c.session.LastSeq,
		},
	}
	payload.OPCode = dto.WSResume // 内嵌结构体字段，单独赋值
	return c.Write(payload)
}

// Identify 对一个连接进行鉴权，并声明监听的 shard 信息
func (c *Client) Identify() error {
	// 避免传错 intent
	if c.session.Intent == 0 {
		c.session.Intent = dto.IntentGuilds
	}
	tk, err := c.session.TokenSource.Token()
	if err != nil {
		log.Errorf("[resume] get access token failed:%s", err)
		return err
	}
	payload := &dto.WSPayload{
		Data: &dto.WSIdentityData{
			Token:   fmt.Sprintf("%s %s", tk.TokenType, tk.AccessToken),
			Intents: c.session.Intent,
			Shard: []uint32{
				c.session.Shards.ShardID,
				c.session.Shards.ShardCount,
			},
		},
	}
	payload.OPCode = dto.WSIdentity
	return c.Write(payload)
}

// Close 关闭连接
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.tickerMu.Lock()
		if c.transportPingTicker != nil {
			c.transportPingTicker.Stop()
		}
		if c.heartBeatTicker != nil {
			c.heartBeatTicker.Stop()
		}
		c.tickerMu.Unlock()
		if c.conn != nil {
			_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
			_ = c.conn.WriteClose(1000, nil)
			_ = c.conn.NetConn().Close()
		}
		c.deduper.Stop()
		c.signalClose(nil)
	})
}

// Session 获取client的session信息
func (c *Client) Session() *dto.Session {
	return c.session
}

func (c *Client) signalClose(err error) {
	if err == nil {
		err = errs.ErrNeedReConnect
	}
	c.queueDoneOnce.Do(func() {
		close(c.queueDone)
	})
	select {
	case c.closeChan <- err:
	default:
		log.Debugf("%s closeChan is full, drop close error: %v", c.session, err)
	}
}

func (c *Client) classifyCloseError(err error) error {
	if err == nil {
		return nil
	}

	// 关闭连接的错误码 https://bot.q.qq.com/wiki/develop/api/gateway/error/error.html
	if !isUnexpectedCloseError(err, 4009, 9000) {
		log.Debugf("%s Listening stop. err is %v", c.session, err)
	} else {
		log.Errorf("%s Listening stop. err is %v", c.session, err)
	}

	// 把 close error 规范化，保留官方错误码，交给 sessions/manager 分类。
	if code, ok := closeCode(err); ok {
		switch code {
		case errs.WSCodeBackendBotOffline, errs.WSCodeBackendBotBanned:
			// 机器人被下架/封禁，不能 identify
			err = errs.New(errs.CodeConnCloseCantIdentify, err.Error())
		case errs.WSCodeBackendAuthenticationFail:
			// accessToken 过期/失效，强制刷新后重试一次
			forceRefreshToken(c.session.TokenSource)
			err = errs.New(int(code), err.Error())
		case errs.WSCodeBackendSessionTimeOut:
			// 4009: session time out，允许 resume
			err = errs.New(int(code), err.Error())
		case errs.WSCodeBackendRateLimit:
			// 4008: 可以 resume 或 identify，交给上层退避
			err = errs.New(int(code), err.Error())
		case errs.WSCodeBackendInvalidIntents, errs.WSCodeBackendDisallowdIntents:
			// 4013/4014: intent 问题，不能直接重试
			err = errs.New(int(code), err.Error())
		case errs.WSCodeBackendUnknownOpCode, errs.WSCodeBackendDecodeError,
			errs.WSCodeBackendInvalidShard, errs.WSCodeBackendShardingRequired,
			errs.WSCodeBackendInvalidAPIVersion:
			// 协议/配置错误，重连也不会成功
			err = errs.New(int(code), err.Error())
		default:
			if isUnexpectedCloseError(err) {
				err = errs.New(errs.CodeConnCloseCantResume, err.Error())
			}
		}
	}

	if event.DefaultHandlers.ErrorNotify != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("[QQBot][ws] ErrorNotify panic: %v", event.NewPanicError("ErrorNotify", r))
				}
			}()
			event.DefaultHandlers.ErrorNotify(err)
		}()
	}
	return err
}

func (c *Client) listenMessageAndHandle() {
	defer func() {
		// panic，一般是由于业务自己实现的 handle 不完善导致
		// 打印日志后，关闭这个连接，进入重连流程
		if err := recover(); err != nil {
			websocket.PanicHandler(err, c.session)
			c.signalClose(fmt.Errorf("panic: %v", err))
		}
	}()

	for {
		select {
		case payload := <-c.messageQueue:
			c.handlePayload(payload)
		case <-c.queueDone:
			// 排空队列中已经收到的事件，保持与旧实现一致。
			for {
				select {
				case payload := <-c.messageQueue:
					c.handlePayload(payload)
				default:
					log.Infof("%s message queue is closed", c.session)
					return
				}
			}
		}
	}
}

func (c *Client) handlePayload(payload *dto.WSPayload) {
	if payload == nil {
		return
	}
	// 去重：同时使用顶层 event id 和消息体 d.id（msg_id）。
	dedupKeys := dedup.Keys(payload)
	// 最后一道保险：即使去重、ready 回调或日志以外的代码 panic，也只丢弃当前事件，
	// 不再让读循环退出、整条连接重连。
	defer func() {
		if r := recover(); r != nil {
			websocket.PanicHandler(r, c.session)
			dedup.MarkAll(c.deduper, dedupKeys)
			if c.seqPolicy == SeqAfterHandle {
				c.saveSeq(payload.Seq)
			}
		}
	}()
	if dedup.SeenAny(c.deduper, dedupKeys) {
		log.Debugf("%s duplicate event %s, skip", c.session, payload.EventID)
		c.saveSeq(payload.Seq)
		return
	}

	if c.seqPolicy == SeqBeforeHandle {
		c.saveSeq(payload.Seq)
	}

	// ready 事件需要特殊处理
	if payload.Type == "READY" {
		c.readyHandler(payload)
		if c.seqPolicy == SeqAfterHandle {
			c.saveSeq(payload.Seq)
		}
		return
	}

	// 解析具体事件，并投递给业务注册的 handler
	if err := event.ParseAndHandle(payload); err != nil {
		if event.IsPanic(err) {
			// 处理器 panic 往往是确定性的，重放只会再次 panic；记录后丢弃该事件，
			// 保证连接和后续消息不受影响。
			log.Errorf("%s handler panic, event dropped: %v", c.session, err)
			dedup.MarkAll(c.deduper, dedupKeys)
			if c.seqPolicy == SeqAfterHandle {
				c.saveSeq(payload.Seq)
			}
			return
		}
		log.Errorf("%s parseAndHandle failed, %v", c.session, err)
		// SeqAfterHandle 时不推进 seq，等 resume 重放；
		// 由于没有 Mark，重放不会被去重跳过。
		return
	}

	dedup.MarkAll(c.deduper, dedupKeys)
	if c.seqPolicy == SeqAfterHandle {
		c.saveSeq(payload.Seq)
	}
}

func (c *Client) saveSeq(seq uint32) {
	if seq > 0 {
		c.session.LastSeq = seq
	}
}

// isHandleBuildIn 内置的事件处理，处理那些不需要业务方处理的事件
// return true 的时候说明事件已经被处理了
func (c *Client) isHandleBuildIn(payload *dto.WSPayload) bool {
	switch payload.OPCode {
	case dto.WSHello: // 接收到 hello 后需要开始发心跳
		c.startHeartBeatTicker(payload.RawMessage)
	case dto.WSHeartbeatAck: // 心跳 ack 不需要业务处理
	case dto.WSReconnect: // 达到连接时长，需要重新连接，此时可以通过 resume 续传原连接上的事件
		c.signalClose(errs.ErrNeedReConnect)
	case dto.WSInvalidSession: // 无效的 sessionLog，需要重新鉴权
		c.signalClose(errs.ErrInvalidSession)
	default:
		return false
	}
	return true
}

// startHeartBeatTicker 启动定时心跳
func (c *Client) startHeartBeatTicker(message []byte) {
	helloData := &dto.WSHelloData{}
	if err := event.ParseData(message, helloData); err != nil {
		log.Errorf("%s hello data parse failed, %v, message %v", c.session, err, message)
		return
	}
	if helloData.HeartbeatInterval <= 0 {
		log.Errorf("%s invalid heartbeat_interval: %d", c.session, helloData.HeartbeatInterval)
		return
	}
	// 根据 hello 的回包，重新设置心跳的定时器时间
	c.tickerMu.Lock()
	c.heartBeatTicker.Reset(time.Duration(helloData.HeartbeatInterval) * time.Millisecond)
	c.tickerMu.Unlock()
}

// readyHandler 针对ready返回的处理，需要记录 sessionID 等相关信息
func (c *Client) readyHandler(payload *dto.WSPayload) {
	readyData := &dto.WSReadyData{}
	if err := event.ParseData(payload.RawMessage, readyData); err != nil {
		log.Errorf("%s parseReadyData failed, %v, message %v", c.session, err, payload.RawMessage)
		return
	}
	c.version = readyData.Version
	// 基于 ready 事件，更新 session 信息
	c.session.ID = readyData.SessionID
	if len(readyData.Shard) >= 2 {
		c.session.Shards.ShardID = readyData.Shard[0]
		c.session.Shards.ShardCount = readyData.Shard[1]
	}
	c.user = &dto.WSUser{
		ID:       readyData.User.ID,
		Username: readyData.User.Username,
		Bot:      readyData.User.Bot,
	}
	// 调用自定义的 ready 回调
	if event.DefaultHandlers.Ready != nil {
		event.DefaultHandlers.Ready(payload, readyData)
	}
}

type gwsEventHandler struct {
	client *Client
}

func (h *gwsEventHandler) OnOpen(socket *gws.Conn) {
	h.refreshReadDeadline(socket)
}

func (h *gwsEventHandler) OnClose(_ *gws.Conn, err error) {
	h.client.signalClose(err)
}

func (h *gwsEventHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetWriteDeadline(time.Now().Add(h.client.writeTimeout))
	_ = socket.WritePong(payload)
	h.refreshReadDeadline(socket)
}

func (h *gwsEventHandler) OnPong(socket *gws.Conn, _ []byte) {
	h.refreshReadDeadline(socket)
}

func (h *gwsEventHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	h.refreshReadDeadline(socket)
	defer func() { _ = message.Close() }()

	if message.Opcode != gws.OpcodeText {
		return
	}

	c := h.client
	raw := message.Bytes()
	payload := &dto.WSPayload{}
	if err := json.Unmarshal(raw, payload); err != nil {
		log.Errorf("%s json failed, %v", c.session, err)
		return
	}
	payload.RawMessage = append([]byte(nil), raw...)
	payload.Session = c.session
	log.Infof("%s receive %s message, %s", c.session, dto.OPMeans(payload.OPCode), string(raw))

	// 处理内置的一些事件，如果处理成功，则这个事件不再投递给业务
	if c.isHandleBuildIn(payload) {
		return
	}

	select {
	case c.messageQueue <- payload:
	case <-c.queueDone:
	}
}

// refreshReadDeadline 只刷新读 deadline。写操作由 Write 按次设置，避免 QQ 心跳
// 在 45~60 秒的间隔下被连接建立时的全局 deadline 提前截断。
func (h *gwsEventHandler) refreshReadDeadline(socket *gws.Conn) {
	if socket == nil {
		return
	}
	_ = socket.SetReadDeadline(time.Now().Add(h.client.pingInterval*2 + h.client.pingWait))
}

type gwsLogger struct{}

func (gwsLogger) Error(v ...any) {
	log.Error(v...)
}

func closeCode(err error) (uint16, bool) {
	var closeErr *gws.CloseError
	if errors.As(err, &closeErr) {
		return closeErr.Code, true
	}
	return 0, false
}

func isUnexpectedCloseError(err error, allowed ...int) bool {
	code, ok := closeCode(err)
	if !ok {
		return false
	}
	if code == 1000 || code == 1001 {
		return false
	}
	for _, c := range allowed {
		if int(code) == c {
			return false
		}
	}
	return true
}

func forceRefreshToken(tokenSource oauth2.TokenSource) {
	if refreshable, ok := tokenSource.(interface {
		ForceRefresh() (*oauth2.Token, error)
	}); ok {
		_, _ = refreshable.ForceRefresh()
		return
	}
	_, _ = tokenSource.Token()
}
