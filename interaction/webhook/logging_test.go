package webhook

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/event"
	botlog "github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/token"
)

type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) add(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, level+" "+msg)
}

func (l *captureLogger) Debug(v ...interface{}) { l.add("Debug", fmt.Sprint(v...)) }
func (l *captureLogger) Info(v ...interface{})  { l.add("Info", fmt.Sprint(v...)) }
func (l *captureLogger) Warn(v ...interface{})  { l.add("Warning", fmt.Sprint(v...)) }
func (l *captureLogger) Error(v ...interface{}) { l.add("Error", fmt.Sprint(v...)) }

func (l *captureLogger) Debugf(format string, v ...interface{}) {
	l.add("Debug", fmt.Sprintf(format, v...))
}
func (l *captureLogger) Infof(format string, v ...interface{}) {
	l.add("Info", fmt.Sprintf(format, v...))
}
func (l *captureLogger) Warnf(format string, v ...interface{}) {
	l.add("Warning", fmt.Sprintf(format, v...))
}
func (l *captureLogger) Errorf(format string, v ...interface{}) {
	l.add("Error", fmt.Sprintf(format, v...))
}
func (l *captureLogger) Sync() error { return nil }

func (l *captureLogger) contains(level, substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.HasPrefix(line, level+" ") && strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

func TestWebhookDispatchFailureLogsContext(t *testing.T) {
	old := botlog.DefaultLogger
	logger := &captureLogger{}
	botlog.SetLogger(logger)
	t.Cleanup(func() { botlog.SetLogger(old) })

	appID := fmt.Sprintf("test-log-%d", time.Now().UnixNano())
	var called int32
	event.RegisterHandlersByAppID(appID, event.GroupATMessageEventHandler(
		func(_ *dto.WSPayload, _ *dto.WSGroupATMessageData) error {
			if atomic.AddInt32(&called, 1) == 1 {
				return errors.New("handler boom")
			}
			return nil
		},
	))

	body := []byte(`{
		"id":"event-log",
		"op":0,
		"t":"GROUP_AT_MESSAGE_CREATE",
		"d":{
			"id":"msg-log",
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

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, signedRequest(t, body, "secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	for _, want := range []string{"事件处理失败", "app=" + appID, "event_id=event-log", "type=GROUP_AT_MESSAGE_CREATE", "handler boom"} {
		if !logger.contains("Error", want) {
			t.Fatalf("webhook failure log missing %q", want)
		}
	}
}
