package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/openapi/options"
)

type sendTestServer struct {
	server   *httptest.Server
	hits     int32
	mu       sync.Mutex
	requests []map[string]any
}

func newSendTestServer(t *testing.T, respond func(hit int32, w http.ResponseWriter)) *sendTestServer {
	t.Helper()
	s := &sendTestServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.requests = append(s.requests, body)
		s.mu.Unlock()
		hit := atomic.AddInt32(&s.hits, 1)
		respond(hit, w)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *sendTestServer) request(i int) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[i]
}

func writeJSONBody(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func newFallbackAPI(t *testing.T) (*openAPI, *retryTokenSource) {
	t.Helper()
	ts := &retryTokenSource{}
	api := (&openAPI{}).Setup("app", ts, false).WithTimeout(2 * time.Second).(*openAPI)
	return api, ts
}

func TestOpenAPIBusinessErrorOnHTTP200(t *testing.T) {
	srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
		writeJSONBody(w, http.StatusOK, `{"err_code":40034006,"message":"消息内容违规"}`)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "bad",
		MsgID:   "msg-1",
	}, options.WithURL(srv.server.URL))
	if err == nil {
		t.Fatal("expected business error")
	}
	if got := errs.Error(err).Code(); got != 40034006 {
		t.Fatalf("error code = %d, want 40034006", got)
	}
	if got := atomic.LoadInt32(&srv.hits); got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
}

func TestOpenAPIBusinessErrorStringCode(t *testing.T) {
	srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
		writeJSONBody(w, http.StatusOK, `{"err_code":"40034005","message":"回复消息msg_id已过期"}`)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "hello",
		MsgID:   "msg-1",
	}, options.WithURL(srv.server.URL))
	if err == nil {
		t.Fatal("expected business error")
	}
	if got := errs.Error(err).Code(); got != 40034005 {
		t.Fatalf("error code = %d, want 40034005", got)
	}
}

func TestOpenAPIMarkdownFallback(t *testing.T) {
	srv := newSendTestServer(t, func(hit int32, w http.ResponseWriter) {
		if hit == 1 {
			writeJSONBody(w, http.StatusOK, `{"err_code":50056,"message":"不允许发送原生 markdown"}`)
			return
		}
		writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
	})
	api, _ := newFallbackAPI(t)

	msg, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		MsgType:  dto.MarkdownMsg,
		Markdown: &dto.Markdown{Content: "**bold**"},
		MsgID:    "msg-1",
	}, options.WithURL(srv.server.URL))
	if err != nil {
		t.Fatalf("PostGroupMessage returned error: %v", err)
	}
	if msg.ID != "msg-2" {
		t.Fatalf("msg id = %q, want msg-2", msg.ID)
	}
	if got := atomic.LoadInt32(&srv.hits); got != 2 {
		t.Fatalf("hits = %d, want 2", got)
	}

	first := srv.request(0)
	if _, ok := first["markdown"]; !ok {
		t.Fatalf("first request should contain markdown: %#v", first)
	}
	if first["msg_id"] != "msg-1" {
		t.Fatalf("first msg_id = %#v, want msg-1", first["msg_id"])
	}
	firstSeq, ok := first["msg_seq"].(float64)
	if !ok || firstSeq == 0 {
		t.Fatalf("first msg_seq missing: %#v", first)
	}

	second := srv.request(1)
	if _, ok := second["markdown"]; ok {
		t.Fatalf("fallback request should not contain markdown: %#v", second)
	}
	if second["content"] != "**bold**" {
		t.Fatalf("fallback content = %#v, want **bold**", second["content"])
	}
	if v, ok := second["msg_type"]; ok && v != float64(dto.TextMsg) {
		t.Fatalf("fallback msg_type = %#v, want %d or omitted", v, dto.TextMsg)
	}
	if second["msg_id"] != "msg-1" {
		t.Fatalf("fallback should keep msg_id, got %#v", second["msg_id"])
	}
	secondSeq, _ := second["msg_seq"].(float64)
	if secondSeq != firstSeq {
		t.Fatalf("markdown fallback should keep msg_seq, got %v then %v", firstSeq, secondSeq)
	}
}

func TestOpenAPIDuplicateMsgSeqRetry(t *testing.T) {
	srv := newSendTestServer(t, func(hit int32, w http.ResponseWriter) {
		if hit == 1 {
			writeJSONBody(w, http.StatusOK, `{"err_code":40054005,"message":"消息被去重，请检查请求msgseq"}`)
			return
		}
		writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "hello",
		MsgID:   "msg-1",
		MsgSeq:  7,
	}, options.WithURL(srv.server.URL))
	if err != nil {
		t.Fatalf("PostGroupMessage returned error: %v", err)
	}
	if got := atomic.LoadInt32(&srv.hits); got != 2 {
		t.Fatalf("hits = %d, want 2", got)
	}
	firstSeq, _ := srv.request(0)["msg_seq"].(float64)
	secondSeq, _ := srv.request(1)["msg_seq"].(float64)
	if firstSeq != 7 {
		t.Fatalf("first msg_seq = %v, want 7", firstSeq)
	}
	if secondSeq == firstSeq || secondSeq == 0 {
		t.Fatalf("retry msg_seq = %v, want different non-zero value", secondSeq)
	}
}

func TestOpenAPIProactiveFallbackOptIn(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
			writeJSONBody(w, http.StatusOK, `{"err_code":40034005,"message":"回复消息msg_id已过期"}`)
		})
		api, _ := newFallbackAPI(t)
		_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
			Content: "hello",
			MsgID:   "msg-1",
		}, options.WithURL(srv.server.URL))
		if err == nil {
			t.Fatal("expected error without proactive fallback")
		}
		if got := atomic.LoadInt32(&srv.hits); got != 1 {
			t.Fatalf("hits = %d, want 1", got)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		srv := newSendTestServer(t, func(hit int32, w http.ResponseWriter) {
			if hit == 1 {
				writeJSONBody(w, http.StatusOK, `{"err_code":40034005,"message":"回复消息msg_id已过期"}`)
				return
			}
			writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
		})
		api, _ := newFallbackAPI(t)
		_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
			Content: "hello",
			MsgID:   "msg-1",
		}, options.WithURL(srv.server.URL), options.WithProactiveFallback())
		if err != nil {
			t.Fatalf("PostGroupMessage returned error: %v", err)
		}
		if got := atomic.LoadInt32(&srv.hits); got != 2 {
			t.Fatalf("hits = %d, want 2", got)
		}
		first := srv.request(0)
		second := srv.request(1)
		if first["msg_id"] != "msg-1" {
			t.Fatalf("first msg_id = %#v, want msg-1", first["msg_id"])
		}
		if _, ok := second["msg_id"]; ok {
			t.Fatalf("proactive fallback should remove msg_id: %#v", second)
		}
		if second["content"] != "hello" {
			t.Fatalf("proactive fallback content = %#v", second["content"])
		}
	})
}

func TestOpenAPIEmptyResponseRetryOptIn(t *testing.T) {
	srv := newSendTestServer(t, func(hit int32, w http.ResponseWriter) {
		if hit == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "hello",
	}, options.WithURL(srv.server.URL), options.WithRetryOnTransient())
	if err != nil {
		t.Fatalf("PostGroupMessage returned error: %v", err)
	}
	if got := atomic.LoadInt32(&srv.hits); got != 2 {
		t.Fatalf("hits = %d, want 2", got)
	}
}

func TestOpenAPIEmptyResponseWithoutRetry(t *testing.T) {
	srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
		w.WriteHeader(http.StatusOK)
	})
	api, _ := newFallbackAPI(t)

	_, err := api.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{
		Content: "hello",
	}, options.WithURL(srv.server.URL))
	if err == nil {
		t.Fatal("expected empty response error")
	}
	if got := errs.Error(err).Code(); got != errs.CodeEmptyResponse {
		t.Fatalf("error code = %d, want %d", got, errs.CodeEmptyResponse)
	}
	if got := atomic.LoadInt32(&srv.hits); got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
}

func TestOpenAPIAutoFillsMsgSeqForPassiveReply(t *testing.T) {
	srv := newSendTestServer(t, func(_ int32, w http.ResponseWriter) {
		writeJSONBody(w, http.StatusOK, `{"id":"msg-2"}`)
	})
	api, _ := newFallbackAPI(t)

	msg := &dto.MessageToCreate{Content: "hello", MsgID: "msg-1"}
	if _, err := api.PostGroupMessage(context.Background(), "group", msg,
		options.WithURL(srv.server.URL)); err != nil {
		t.Fatal(err)
	}
	if msg.MsgSeq != 0 {
		t.Fatalf("caller message was mutated: %+v", msg)
	}
	seq, _ := srv.request(0)["msg_seq"].(float64)
	if seq == 0 {
		t.Fatalf("request should contain auto-filled msg_seq: %#v", srv.request(0))
	}
}
