package local

import (
	"context"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/websocket"
)

type testTokenSource struct{}

func (testTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "token", TokenType: "QQBot", Expiry: time.Now().Add(time.Hour)}, nil
}

type terminalWS struct {
	session *dto.Session
}

func (w *terminalWS) New(_ context.Context, session dto.Session) websocket.WebSocket {
	return &terminalWS{session: &session}
}
func (w *terminalWS) Connect() error  { return nil }
func (w *terminalWS) Identify() error { return nil }
func (w *terminalWS) Resume() error   { return nil }
func (w *terminalWS) Session() *dto.Session {
	return w.session
}
func (w *terminalWS) Listening() error {
	return errs.New(errs.CodeConnCloseCantIdentify, "bot banned")
}
func (w *terminalWS) Write(*dto.WSPayload) error { return nil }
func (w *terminalWS) Close()                     {}

func TestChanManagerTerminalErrorReturns(t *testing.T) {
	old := websocket.ClientImpl
	websocket.Register(&terminalWS{})
	defer func() { websocket.ClientImpl = old }() //nolint:reassign // 测试恢复全局状态

	m := New()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	intent := dto.IntentGuildAtMessage
	apInfo := &dto.WebsocketAP{
		URL:    "ws://example.invalid",
		Shards: 1,
		SessionStartLimit: dto.SessionStartLimit{
			Remaining:      1,
			MaxConcurrency: 2,
		},
	}
	err := m.Start(ctx, apInfo, testTokenSource{}, &intent)
	if err == nil {
		t.Fatal("expected terminal error")
	}
	if got := errs.Error(err).Code(); got != errs.CodeConnCloseCantIdentify {
		t.Fatalf("error code = %d, want %d", got, errs.CodeConnCloseCantIdentify)
	}
}
