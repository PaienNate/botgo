package local

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/websocket"
)

// authFailWS 每次连接都返回 4004，用于验证“重试一次后 Terminal”。
type authFailWS struct {
	session *dto.Session
}

var authFailCount int32

func (w *authFailWS) New(_ context.Context, session dto.Session) websocket.WebSocket {
	return &authFailWS{session: &session}
}
func (w *authFailWS) Connect() error  { return nil }
func (w *authFailWS) Identify() error { return nil }
func (w *authFailWS) Resume() error   { return nil }
func (w *authFailWS) Session() *dto.Session {
	return w.session
}
func (w *authFailWS) Listening() error {
	atomic.AddInt32(&authFailCount, 1)
	return errs.New(errs.WSCodeBackendAuthenticationFail, "auth failed")
}
func (w *authFailWS) Write(*dto.WSPayload) error { return nil }
func (w *authFailWS) Close()                     {}

func TestChanManagerAuthFailRetriesOnceThenTerminal(t *testing.T) {
	old := websocket.ClientImpl
	websocket.Register(&authFailWS{})
	defer func() { websocket.ClientImpl = old }() //nolint:reassign // 测试恢复全局状态

	m := New()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	intent := dto.IntentGuildAtMessage
	apInfo := &dto.WebsocketAP{
		URL:    "ws://example.invalid",
		Shards: 1,
		SessionStartLimit: dto.SessionStartLimit{
			Remaining:      10,
			MaxConcurrency: 10,
		},
	}
	err := m.Start(ctx, apInfo, testTokenSource{}, &intent)
	if err == nil {
		t.Fatal("expected terminal error")
	}
	if got := errs.Error(err).Code(); got != errs.WSCodeBackendAuthenticationFail {
		t.Fatalf("error code = %d, want %d", got, errs.WSCodeBackendAuthenticationFail)
	}
	// 第一次 Listening + 强刷后重试一次 = 2 次，之后必须 Terminal。
	if n := atomic.LoadInt32(&authFailCount); n != 2 {
		t.Fatalf("Listening called %d times, want 2", n)
	}
}
