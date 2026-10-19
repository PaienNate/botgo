// Package gateway 提供一个基于 gws 的正向 WebSocket gateway：
// SDK 把 QQ 事件推送给连接上来的外部客户端。
package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/lxzan/gws"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/event"
)

const (
	defaultPingInterval = 30 * time.Second
	defaultPingWait     = 10 * time.Second
)

// HubOptions gateway 配置。
type HubOptions struct {
	// FrameMode 推送帧格式，默认 FrameModeEnvelope。
	FrameMode FrameMode
	// Authorize 自定义鉴权。返回 appID 和是否允许连接。
	// 默认从 X-Bot-Appid header 或 ?appid= 读取。
	Authorize    func(r *http.Request) (appID string, ok bool)
	PingInterval time.Duration
	PingWait     time.Duration
}

type clientState struct {
	conn  *gws.Conn
	appID string
	done  chan struct{}
	once  sync.Once
}

func (c *clientState) close() {
	c.once.Do(func() { close(c.done) })
}

// Hub 管理 gateway 客户端。
type Hub struct {
	opts     HubOptions
	upgrader *gws.Upgrader

	mu      sync.RWMutex
	clients map[string]map[*clientState]struct{}
}

// NewHub 创建 gateway Hub。
func NewHub(opts HubOptions) *Hub {
	if opts.FrameMode < FrameModeEnvelope || opts.FrameMode > FrameModeBB {
		opts.FrameMode = FrameModeEnvelope
	}
	if opts.Authorize == nil {
		opts.Authorize = defaultAuthorize
	}
	if opts.PingInterval <= 0 {
		opts.PingInterval = defaultPingInterval
	}
	if opts.PingWait <= 0 {
		opts.PingWait = defaultPingWait
	}

	h := &Hub{
		opts:    opts,
		clients: make(map[string]map[*clientState]struct{}),
	}
	h.upgrader = gws.NewUpgrader(&hubHandler{hub: h}, &gws.ServerOption{
		ParallelEnabled: false,
		Recovery:        gws.Recovery,
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			appID, ok := h.opts.Authorize(r)
			if !ok || appID == "" {
				return false
			}
			session.Store("appid", appID)
			return true
		},
	})
	return h
}

// Handler 返回 HTTP handler。
func (h *Hub) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		socket, err := h.upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		go socket.ReadLoop()
	})
}

// Broadcast 把事件推送给指定 appID 的所有客户端。
func (h *Hub) Broadcast(appID string, payload *dto.WSPayload) {
	if payload == nil {
		return
	}
	data, err := marshalFrame(h.opts.FrameMode, appID, payload)
	if err != nil {
		return
	}
	h.broadcastData(appID, data)
}

// BroadcastAll 把事件推送给所有客户端。
func (h *Hub) BroadcastAll(appID string, payload *dto.WSPayload) {
	if payload == nil {
		return
	}
	data, err := marshalFrame(h.opts.FrameMode, appID, payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	all := make([]*clientState, 0)
	for _, clients := range h.clients {
		for c := range clients {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	h.broadcastClients(all, data)
}

func (h *Hub) broadcastData(appID string, data []byte) {
	h.mu.RLock()
	clients := make([]*clientState, 0, len(h.clients[appID]))
	for c := range h.clients[appID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()
	h.broadcastClients(clients, data)
}

func (h *Hub) broadcastClients(clients []*clientState, data []byte) {
	if len(clients) == 0 {
		return
	}
	b := gws.NewBroadcaster(gws.OpcodeText, data)
	defer func() { _ = b.Close() }()
	for _, c := range clients {
		_ = b.Broadcast(c.conn, nil)
	}
}

// Observe 注册一个事件观察者，把收到的 QQ 事件自动转发给对应 appID 的客户端。
// 返回取消函数。
func (h *Hub) Observe() func() {
	return event.RegisterObserver(func(payload *dto.WSPayload) {
		if payload == nil || payload.Session == nil || payload.Session.AppID == "" {
			return
		}
		h.Broadcast(payload.Session.AppID, payload)
	})
}

// Close 关闭所有客户端。ctx 用于兼容优雅关闭接口。
func (h *Hub) Close(_ context.Context) error {
	h.mu.Lock()
	all := make([]*clientState, 0)
	for _, clients := range h.clients {
		for c := range clients {
			all = append(all, c)
		}
	}
	h.mu.Unlock()

	for _, c := range all {
		c.close()
		_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = c.conn.WriteClose(1000, nil)
		_ = c.conn.NetConn().Close()
	}

	h.mu.Lock()
	h.clients = make(map[string]map[*clientState]struct{})
	h.mu.Unlock()
	return nil
}

func (h *Hub) register(appID string, conn *gws.Conn) *clientState {
	c := &clientState{
		conn:  conn,
		appID: appID,
		done:  make(chan struct{}),
	}
	h.mu.Lock()
	if h.clients[appID] == nil {
		h.clients[appID] = make(map[*clientState]struct{})
	}
	h.clients[appID][c] = struct{}{}
	h.mu.Unlock()
	conn.Session().Store("client", c)

	if hello := marshalHello(h.opts.FrameMode, appID); hello != nil {
		_ = conn.WriteMessage(gws.OpcodeText, hello)
	}

	go h.pingLoop(c)
	return c
}

func (h *Hub) unregister(c *clientState) {
	if c == nil {
		return
	}
	c.close()
	h.mu.Lock()
	if clients := h.clients[c.appID]; clients != nil {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.clients, c.appID)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) pingLoop(c *clientState) {
	ticker := time.NewTicker(h.opts.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.conn.WritePing(nil); err != nil {
				_ = c.conn.NetConn().Close()
				return
			}
		}
	}
}

type hubHandler struct {
	hub *Hub
}

func (h *hubHandler) OnOpen(socket *gws.Conn) {
	appID := ""
	if v, ok := socket.Session().Load("appid"); ok {
		appID, _ = v.(string)
	}
	if appID == "" {
		_ = socket.WriteClose(1008, []byte("missing appid"))
		return
	}
	_ = socket.SetDeadline(time.Now().Add(h.hub.opts.PingInterval + h.hub.opts.PingWait))
	h.hub.register(appID, socket)
}

func (h *hubHandler) OnClose(socket *gws.Conn, _ error) {
	if v, ok := socket.Session().Load("client"); ok {
		if c, ok := v.(*clientState); ok {
			h.hub.unregister(c)
		}
	}
}

func (h *hubHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
	_ = socket.SetDeadline(time.Now().Add(h.hub.opts.PingInterval + h.hub.opts.PingWait))
}

func (h *hubHandler) OnPong(socket *gws.Conn, _ []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.hub.opts.PingInterval + h.hub.opts.PingWait))
}

func (h *hubHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer func() { _ = message.Close() }()
	if message.Opcode != gws.OpcodeText {
		return
	}
	var frame Frame
	if err := json.Unmarshal(message.Bytes(), &frame); err != nil {
		return
	}
	switch frame.Type {
	case "ping":
		data, _ := json.Marshal(Frame{V: 1, Type: "pong", TS: nowUnix()})
		_ = socket.WriteMessage(gws.OpcodeText, data)
	case "ack":
		// 客户端确认，当前不需要额外处理。
	}
}

func defaultAuthorize(r *http.Request) (string, bool) {
	appID := r.Header.Get("X-Bot-Appid")
	if appID == "" {
		appID = r.URL.Query().Get("appid")
	}
	return appID, appID != ""
}
