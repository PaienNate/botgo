package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/event"
	"github.com/sealdice/botgo/interaction/signature"
	"github.com/sealdice/botgo/token"
)

func signedRequest(t *testing.T, body []byte, secret string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/qqbot", bytes.NewReader(body))
	req.Header.Set(signature.HeaderTimestamp, strconv.FormatInt(time.Now().Unix(), 10))
	sig, err := signature.Generate(secret, req.Header, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(signature.HeaderSig, sig)
	return req
}

func TestHandlerValidationWithoutSignature(t *testing.T) {
	body := []byte(`{"op":13,"d":{"plain_token":"plain","event_ts":"123456"}}`)
	req := httptest.NewRequest(http.MethodPost, "/qqbot", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: "app", AppSecret: "secret"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rsp dto.WHValidationRsp
	if err := json.Unmarshal(rec.Body.Bytes(), &rsp); err != nil {
		t.Fatal(err)
	}
	if rsp.PlainToken != "plain" || rsp.Signature == "" {
		t.Fatalf("unexpected validation response: %+v", rsp)
	}
}

func TestHandlerDispatchACKAndDedup(t *testing.T) {
	appID := fmt.Sprintf("test-app-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	body := []byte(`{
		"id":"event-1",
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
	handler := NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: appID, AppSecret: "secret"},
		Deduper:     NewTTLDeduper(time.Minute, 100),
	})

	for i := 0; i < 2; i++ {
		req := signedRequest(t, body, "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var ackMsg ack
		if err := json.Unmarshal(rec.Body.Bytes(), &ackMsg); err != nil {
			t.Fatal(err)
		}
		if ackMsg.Op != dto.HTTPCallbackAck || ackMsg.Data != 0 {
			t.Fatalf("unexpected ack: %+v", ackMsg)
		}
	}
	if called != 1 {
		t.Fatalf("handler called %d times, want 1", called)
	}
}

func TestHandlerInvalidSignature(t *testing.T) {
	body := []byte(`{"id":"event-1","op":0,"t":"GROUP_AT_MESSAGE_CREATE","d":{}}`)
	req := signedRequest(t, body, "wrong-secret")
	rec := httptest.NewRecorder()
	NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: "app", AppSecret: "secret"},
	}).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandlerPanicReturnsRetryACK(t *testing.T) {
	appID := fmt.Sprintf("test-panic-%d", time.Now().UnixNano())
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			panic("boom")
		},
	))

	body := []byte(`{
		"id":"event-panic",
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
	req := signedRequest(t, body, "secret")
	rec := httptest.NewRecorder()
	NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: appID, AppSecret: "secret"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var ackMsg ack
	if err := json.Unmarshal(rec.Body.Bytes(), &ackMsg); err != nil {
		t.Fatal(err)
	}
	if ackMsg.Op != dto.HTTPCallbackAck || ackMsg.Data != 1 {
		t.Fatalf("unexpected panic ack: %+v", ackMsg)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/qqbot", nil)
	rec := httptest.NewRecorder()
	NewHandler(HandlerOptions{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
