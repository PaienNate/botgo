package token

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

type panicTokenSource struct{}

func (panicTokenSource) Token() (*oauth2.Token, error) {
	panic("token source boom")
}

func TestStartRefreshAccessTokenRecoversTokenSourcePanic(t *testing.T) {
	err := StartRefreshAccessToken(context.Background(), panicTokenSource{})
	if err == nil {
		t.Fatal("expected token source panic error")
	}
	if !strings.Contains(err.Error(), "token source boom") || !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("error should contain panic value and stack: %v", err)
	}
}

func TestRunRefreshErrorHandlerRecoversPanic(t *testing.T) {
	// 不应把测试进程打崩。
	runRefreshErrorHandler(func(error) {
		panic("handler boom")
	}, "app", nil)
}

func TestQQBotTokenSourceTokenRecoversPanic(t *testing.T) {
	src := NewQQBotTokenSource(nil)
	_, err := src.Token()
	if err == nil {
		t.Fatal("expected token source panic error")
	}
	if !strings.Contains(err.Error(), "panic") || !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("error should contain panic value and stack: %v", err)
	}
}
