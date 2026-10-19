package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lxzan/gws"

	"github.com/sealdice/botgo/dto"
)

type testClientHandler struct {
	mu       sync.Mutex
	messages []Frame
	opened   chan struct{}
}

func (h *testClientHandler) OnOpen(_ *gws.Conn) {
	select {
	case <-h.opened:
	default:
		close(h.opened)
	}
}

func (h *testClientHandler) OnClose(_ *gws.Conn, _ error) {}

func (h *testClientHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (h *testClientHandler) OnPong(_ *gws.Conn, _ []byte) {}

func (h *testClientHandler) OnMessage(_ *gws.Conn, message *gws.Message) {
	defer func() { _ = message.Close() }()
	var frame Frame
	if err := json.Unmarshal(message.Bytes(), &frame); err != nil {
		return
	}
	h.mu.Lock()
	h.messages = append(h.messages, frame)
	h.mu.Unlock()
}

func (h *testClientHandler) waitFor(t *testing.T, typ string) Frame {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		for _, frame := range h.messages {
			if frame.Type == typ {
				h.mu.Unlock()
				return frame
			}
		}
		h.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for frame type %q", typ)
	return Frame{}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub(HubOptions{})
	defer func() { _ = hub.Close(context.Background()) }()

	server := httptest.NewServer(hub.Handler())
	defer server.Close()

	handler := &testClientHandler{opened: make(chan struct{})}
	conn, _, err := gws.NewClient(handler, &gws.ClientOption{
		Addr: "ws" + strings.TrimPrefix(server.URL, "http") + "?appid=app-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	go conn.ReadLoop()
	defer func() { _ = conn.NetConn().Close() }()

	select {
	case <-handler.opened:
	case <-time.After(3 * time.Second):
		t.Fatal("client open timeout")
	}

	hub.Broadcast("app-1", &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode:  dto.WSDispatchEvent,
			Type:    dto.EventGroupAtMessageCreate,
			EventID: "event-1",
			Seq:     7,
		},
	})
	frame := handler.waitFor(t, "event")
	if frame.AppID != "app-1" || frame.ID != "event-1" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}
