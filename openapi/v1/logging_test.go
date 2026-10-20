package v1

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/sealdice/botgo/dto"
	botlog "github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/openapi/options"
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

func captureLogs(t *testing.T) *captureLogger {
	t.Helper()
	old := botlog.DefaultLogger
	logger := &captureLogger{}
	botlog.SetLogger(logger)
	t.Cleanup(func() { botlog.SetLogger(old) })
	return logger
}

func TestSendFailureLogsAreVisibleAndActionable(t *testing.T) {
	logs := captureLogs(t)
	srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
		writeJSONBody(w, http.StatusOK, `{"err_code":40034006,"message":"消息内容违规"}`)
	})
	api, _ := newFallbackAPI(t)

	_, _ = api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "bad",
		MsgID:   "msg-1",
	}, options.WithURL(srv.server.URL))

	if !logs.contains("Warning", "请求返回错误") {
		t.Fatal("missing business error warning log")
	}
	if !logs.contains("Warning", "code=40034006") {
		t.Fatal("business error warning should contain code")
	}
	if !logs.contains("Error", "消息发送最终失败") {
		t.Fatal("missing final failure error log")
	}
	for _, want := range []string{"app=app", "target=group:group", "msg_id=msg-1", "code=40034006"} {
		if !logs.contains("Error", want) {
			t.Fatalf("final failure log missing %q", want)
		}
	}
}

func TestMarkdownFallbackLogsWarning(t *testing.T) {
	logs := captureLogs(t)
	srv := newSendTestServer(t, func(hit int32, w http.ResponseWriter) {
		if hit == 1 {
			writeJSONBody(w, http.StatusOK, `{"err_code":50056,"message":"不允许发送原生 markdown"}`)
			return
		}
		writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		MsgType:  dto.MarkdownMsg,
		Markdown: &dto.Markdown{Content: "**bold**"},
		MsgID:    "msg-1",
	}, options.WithURL(srv.server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if !logs.contains("Warning", "markdown 被拒绝，已自动降级为纯文本") {
		t.Fatal("missing markdown fallback warning")
	}
	if logs.contains("Error", "消息发送最终失败") {
		t.Fatal("successful fallback should not log final failure")
	}
}

func TestRedactLogBody(t *testing.T) {
	body := []byte(`{"id":"msg-1","content":"用户消息","markdown":{"content":"**secret**"},"file_data":"base64data","file_info":"file-info-secret","access_token":"access-token-secret"}`)
	got := redactLogBody(body)
	for _, secret := range []string{"用户消息", "secret", "base64data", "file-info-secret", "access-token-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted log still contains %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, `"id":"msg-1"`) || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("unexpected redacted log: %s", got)
	}
}
