package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lxzan/gws"
	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/event"
)

type staticTokenSource struct{}

func (staticTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "QQBot",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

type fakeGateway struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	conn   *gws.Conn
}

func newFakeGateway(t *testing.T) *fakeGateway {
	t.Helper()
	g := &fakeGateway{t: t}
	upgrader := gws.NewUpgrader(g, &gws.ServerOption{})
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		socket, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		go socket.ReadLoop()
	}))
	return g
}

func (g *fakeGateway) wsURL() string {
	return "ws" + strings.TrimPrefix(g.server.URL, "http")
}

func (g *fakeGateway) Close() {
	g.server.Close()
}

func (g *fakeGateway) OnOpen(socket *gws.Conn) {
	g.mu.Lock()
	g.conn = socket
	g.mu.Unlock()
	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = socket.WriteMessage(gws.OpcodeText, []byte(`{"op":10,"d":{"heartbeat_interval":1000}}`))
	}()
}

func (g *fakeGateway) OnClose(_ *gws.Conn, _ error) {}

func (g *fakeGateway) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (g *fakeGateway) OnPong(_ *gws.Conn, _ []byte) {}

func (g *fakeGateway) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer func() { _ = message.Close() }()
	payload := &dto.WSPayload{}
	if err := json.Unmarshal(message.Bytes(), payload); err != nil {
		g.t.Errorf("gateway unmarshal: %v", err)
		return
	}
	switch payload.OPCode {
	case dto.WSIdentity:
		_ = socket.WriteMessage(gws.OpcodeText, []byte(`{"op":0,"s":1,"t":"READY","d":{"version":1,"session_id":"session-1","user":{"id":"bot","username":"bot","bot":true},"shard":[0,1]}}`))
	case dto.WSResume:
		_ = socket.WriteMessage(gws.OpcodeText, []byte(`{"op":0,"s":2,"t":"RESUMED","d":""}`))
	case dto.WSHeartbeat:
		_ = socket.WriteMessage(gws.OpcodeText, []byte(`{"op":11}`))
	default:
		// 其他 opcode 在测试中不处理
	}
}

func (g *fakeGateway) send(payload string) {
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		g.t.Fatal("gateway connection is nil")
	}
	if err := conn.WriteMessage(gws.OpcodeText, []byte(payload)); err != nil {
		g.t.Fatalf("gateway send: %v", err)
	}
}

func (g *fakeGateway) closeWith(code uint16) {
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		g.t.Fatal("gateway connection is nil")
	}
	_ = conn.WriteClose(code, []byte("test close"))
}

func newTestClient(t *testing.T, gw *fakeGateway, appID string) (*Client, context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := (&Client{}).New(ctx, dto.Session{
		URL:         gw.wsURL(),
		TokenSource: staticTokenSource{},
		Intent:      dto.IntentGuildAtMessage,
		Shards:      dto.ShardConfig{ShardID: 0, ShardCount: 1},
		AppID:       appID,
	}).(*Client)
	if err := c.Connect(); err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := c.Identify(); err != nil {
		cancel()
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- c.Listening() }()
	return c, cancel, errCh
}

func TestClientIdentifyAndEvent(t *testing.T) {
	gw := newFakeGateway(t)
	defer gw.Close()

	appID := fmt.Sprintf("client-test-%d", time.Now().UnixNano())
	var received int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&received, 1)
			return nil
		},
	))

	readyCh := make(chan struct{})
	oldReady := event.DefaultHandlers.Ready
	event.DefaultHandlers.Ready = func(_ *dto.WSPayload, _ *dto.WSReadyData) {
		close(readyCh)
	}
	defer func() { event.DefaultHandlers.Ready = oldReady }()

	_, cancel, errCh := newTestClient(t, gw, appID)
	defer cancel()

	select {
	case <-readyCh:
	case <-time.After(10 * time.Second):
		t.Fatal("ready timeout")
	}

	gw.send(`{
		"id":"event-1",
		"op":0,
		"s":2,
		"t":"GROUP_AT_MESSAGE_CREATE",
		"d":{
			"id":"msg-1",
			"content":"1d100",
			"group_openid":"group-1",
			"author":{"member_openid":"user-1"},
			"timestamp":"2024-10-15T16:33:15+08:00"
		}
	}`)

	deadline := time.Now().Add(10 * time.Second)
	for atomic.LoadInt32(&received) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if received != 1 {
		t.Fatalf("received = %d, want 1", received)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Listening returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Listening did not stop")
	}
}

func TestHandlePayloadDedup(t *testing.T) {
	appID := fmt.Sprintf("dedup-test-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	c := (&Client{}).New(context.Background(), dto.Session{AppID: appID}).(*Client)
	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode:  dto.WSDispatchEvent,
			Type:    dto.EventGroupAtMessageCreate,
			EventID: "event-1",
			Seq:     2,
		},
		RawMessage: []byte(`{
			"id":"event-1",
			"op":0,
			"s":2,
			"t":"GROUP_AT_MESSAGE_CREATE",
			"d":{
				"id":"msg-1",
				"content":"1d100",
				"group_openid":"group-1",
				"author":{"member_openid":"user-1"},
				"timestamp":"2024-10-15T16:33:15+08:00"
			}
		}`),
		Session: c.session,
	}

	c.handlePayload(payload)
	c.handlePayload(payload)

	if called != 1 {
		t.Fatalf("handler called %d times, want 1", called)
	}
	if c.session.LastSeq != 2 {
		t.Fatalf("LastSeq = %d, want 2", c.session.LastSeq)
	}
}

func TestHandlePayloadDedupByMessageID(t *testing.T) {
	appID := fmt.Sprintf("dedup-msg-test-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	c := (&Client{}).New(context.Background(), dto.Session{AppID: appID}).(*Client)
	raw := []byte(`{
		"op":0,
		"t":"GROUP_AT_MESSAGE_CREATE",
		"d":{
			"id":"msg-1",
			"content":"1d100",
			"group_openid":"group-1",
			"author":{"member_openid":"user-1"},
			"timestamp":"2024-10-15T16:33:15+08:00"
		}
	}`)
	for _, eventID := range []string{"event-1", "event-2"} {
		payload := &dto.WSPayload{
			WSPayloadBase: dto.WSPayloadBase{
				OPCode:  dto.WSDispatchEvent,
				Type:    dto.EventGroupAtMessageCreate,
				EventID: eventID,
				Seq:     2,
			},
			RawMessage: raw,
			Session:    c.session,
		}
		c.handlePayload(payload)
	}
	if called != 1 {
		t.Fatalf("handler called %d times, want 1", called)
	}
}

func TestClientCloseCodeMapping(t *testing.T) {
	tests := []struct {
		name     string
		close    uint16
		wantCode int
	}{
		{name: "4009 resume", close: 4009, wantCode: 4009},
		{name: "4006 identify", close: 4006, wantCode: errs.CodeConnCloseCantResume},
		{name: "4914 terminal", close: 4914, wantCode: errs.CodeConnCloseCantIdentify},
		{name: "4014 disallowed", close: 4014, wantCode: errs.WSCodeBackendDisallowdIntents},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeGateway(t)
			defer gw.Close()
			appID := fmt.Sprintf("client-close-%d", time.Now().UnixNano())

			readyCh := make(chan struct{})
			oldReady := event.DefaultHandlers.Ready
			event.DefaultHandlers.Ready = func(_ *dto.WSPayload, _ *dto.WSReadyData) {
				select {
				case <-readyCh:
				default:
					close(readyCh)
				}
			}
			defer func() { event.DefaultHandlers.Ready = oldReady }()

			_, cancel, errCh := newTestClient(t, gw, appID)
			defer cancel()

			select {
			case <-readyCh:
			case <-time.After(10 * time.Second):
				t.Fatal("ready timeout")
			}

			gw.closeWith(tt.close)
			select {
			case err := <-errCh:
				if err == nil {
					t.Fatal("expected error")
				}
				if got := errs.Error(err).Code(); got != tt.wantCode {
					t.Fatalf("error code = %d, want %d (err: %v)", got, tt.wantCode, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Listening did not stop")
			}
		})
	}
}
