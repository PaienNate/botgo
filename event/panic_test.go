package event

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
)

func TestParseAndHandleRecoversHandlerPanic(t *testing.T) {
	appID := fmt.Sprintf("event-panic-%d", time.Now().UnixNano())
	RegisterHandlersByAppID(appID, GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			panic("handler boom")
		},
	))

	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode:  dto.WSDispatchEvent,
			Type:    dto.EventGroupAtMessageCreate,
			EventID: "event-panic",
		},
		RawMessage: []byte(`{"d":{"id":"msg-1","content":"hi"}}`),
		Session:    &dto.Session{AppID: appID},
	}

	err := ParseAndHandle(payload)
	if !IsPanic(err) {
		t.Fatalf("error = %v, want PanicError", err)
	}
	if !strings.Contains(err.Error(), "handler boom") || !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("panic error should include value and stack: %v", err)
	}
}

func TestParseAndHandleContainsFilterPanic(t *testing.T) {
	appID := fmt.Sprintf("filter-panic-%d", time.Now().UnixNano())
	var called int32
	RegisterHandlersByAppID(appID, GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	cancel := RegisterFilter(func(_ *dto.WSPayload) bool {
		panic("filter boom")
	})
	defer cancel()

	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode: dto.WSDispatchEvent,
			Type:   dto.EventGroupAtMessageCreate,
		},
		RawMessage: []byte(`{"d":{"id":"msg-1"}}`),
		Session:    &dto.Session{AppID: appID},
	}
	if err := ParseAndHandle(payload); err != nil {
		t.Fatalf("filter panic should be contained, got %v", err)
	}
	if got := atomic.LoadInt32(&called); got != 0 {
		t.Fatalf("handler called %d times, want 0 after filter panic", got)
	}
}

func TestParseAndHandleContainsObserverPanic(t *testing.T) {
	appID := fmt.Sprintf("observer-panic-%d", time.Now().UnixNano())
	var called int32
	RegisterHandlersByAppID(appID, GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			atomic.AddInt32(&called, 1)
			return nil
		},
	))

	cancel := RegisterObserver(func(_ *dto.WSPayload) {
		panic("observer boom")
	})
	defer cancel()

	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			OPCode: dto.WSDispatchEvent,
			Type:   dto.EventGroupAtMessageCreate,
		},
		RawMessage: []byte(`{"d":{"id":"msg-1"}}`),
		Session:    &dto.Session{AppID: appID},
	}
	if err := ParseAndHandle(payload); err != nil {
		t.Fatalf("observer panic should be contained, got %v", err)
	}
	if got := atomic.LoadInt32(&called); got != 1 {
		t.Fatalf("handler called %d times, want 1", got)
	}
}
