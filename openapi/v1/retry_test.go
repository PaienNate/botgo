package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi/options"
)

type retryTokenSource struct {
	forceRefreshCount int32
}

func (t *retryTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "token",
		TokenType:   "QQBot",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

func (t *retryTokenSource) ForceRefresh() (*oauth2.Token, error) {
	atomic.AddInt32(&t.forceRefreshCount, 1)
	return t.Token()
}

func TestOpenAPIRetriesOnceOnTokenExpired(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":11244,"message":"token expired"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg-1","content":"ok"}`))
	}))
	defer server.Close()

	tokenSource := &retryTokenSource{}
	api := (&openAPI{}).Setup("app", tokenSource, false).WithTimeout(3 * time.Second)
	msg, err := api.PostMessage(context.Background(), "channel", &dto.MessageToCreate{Content: "hi"},
		options.WithURL(server.URL))
	if err != nil {
		t.Fatalf("PostMessage returned error: %v", err)
	}
	if msg == nil || msg.ID != "msg-1" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("server hits = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&tokenSource.forceRefreshCount); got != 1 {
		t.Fatalf("force refresh count = %d, want 1", got)
	}
}
