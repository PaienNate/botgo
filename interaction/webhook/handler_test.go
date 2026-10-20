package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestHandlerDedupByMessageID(t *testing.T) {
	appID := fmt.Sprintf("test-msg-dedup-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	handler := NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: appID, AppSecret: "secret"},
		Deduper:     NewTTLDeduper(time.Minute, 100),
	})
	for _, eventID := range []string{"event-1", "event-2"} {
		body := []byte(fmt.Sprintf(`{
			"id":%q,
			"op":0,
			"t":"GROUP_AT_MESSAGE_CREATE",
			"d":{
				"id":"msg-1",
				"content":"1d100",
				"group_openid":"group-1",
				"author":{"member_openid":"user-1"},
				"timestamp":"2024-10-15T16:33:15+08:00"
			}
		}`, eventID))
		req := signedRequest(t, body, "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}
	if called != 1 {
		t.Fatalf("handler called %d times, want 1", called)
	}
}

func TestHandlerMarksDedupBeforeDispatch(t *testing.T) {
	appID := fmt.Sprintf("test-inflight-%d", time.Now().UnixNano())
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			entered <- struct{}{}
			<-release
			return nil
		},
	))

	body := []byte(`{
		"id":"event-inflight",
		"op":0,
		"t":"GROUP_AT_MESSAGE_CREATE",
		"d":{
			"id":"msg-inflight",
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

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, signedRequest(t, body, "secret"))
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first handler did not start")
	}

	// 第一次处理还没结束时，平台重试同一条事件。此时应直接 ACK，不能再次进入业务处理。
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, signedRequest(t, body, "secret"))
		if rec.Code != http.StatusOK {
			t.Errorf("second status = %d, want 200", rec.Code)
		}
		var ackMsg ack
		if err := json.Unmarshal(rec.Body.Bytes(), &ackMsg); err != nil {
			t.Errorf("second ack unmarshal: %v", err)
		} else if ackMsg.Op != dto.HTTPCallbackAck || ackMsg.Data != 0 {
			t.Errorf("second ack = %+v, want op 12 d 0", ackMsg)
		}
	}()

	select {
	case <-secondDone:
	case <-time.After(500 * time.Millisecond):
		close(release)
		t.Fatal("duplicate in-flight event was not ACKed promptly")
	}

	close(release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}
	if got := atomic.LoadInt32(&called); got != 1 {
		t.Fatalf("handler called %d times, want 1", got)
	}
}

func TestHandlerUnmarksDedupOnDispatchFailure(t *testing.T) {
	appID := fmt.Sprintf("test-failure-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			if atomic.AddInt32(&called, 1) == 1 {
				return errors.New("boom")
			}
			return nil
		},
	))

	body := []byte(`{
		"id":"event-failure",
		"op":0,
		"t":"GROUP_AT_MESSAGE_CREATE",
		"d":{
			"id":"msg-failure",
			"content":"1d100",
			"group_openid":"group-1",
			"author":{"member_openid":"user-1"},
			"timestamp":"2024-10-15T16:33:15+08:00"
		}
	}`)
	handler := NewHandler(HandlerOptions{
		Credentials: &token.QQBotCredentials{AppID: appID, AppSecret: "secret"},
		Deduper:     NewTTLDeduper(time.Minute, 100),
		ErrorHandler: func(error) {
		},
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, signedRequest(t, body, "secret"))
	var ackMsg ack
	if err := json.Unmarshal(rec.Body.Bytes(), &ackMsg); err != nil {
		t.Fatal(err)
	}
	if ackMsg.Data != 1 {
		t.Fatalf("first ack = %+v, want d=1", ackMsg)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, signedRequest(t, body, "secret"))
	if err := json.Unmarshal(rec.Body.Bytes(), &ackMsg); err != nil {
		t.Fatal(err)
	}
	if ackMsg.Data != 0 {
		t.Fatalf("second ack = %+v, want d=0", ackMsg)
	}
	if got := atomic.LoadInt32(&called); got != 2 {
		t.Fatalf("handler called %d times, want 2", got)
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
