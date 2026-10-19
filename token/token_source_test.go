package token

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetRefreshMilliSec(t *testing.T) {
	tests := []struct {
		name      string
		ttl       int64
		wantMinMS int64
		wantMaxMS int64
	}{
		{name: "normal", ttl: 7200, wantMinMS: 7135_000, wantMaxMS: 7140_000},
		{name: "short", ttl: 30, wantMinMS: 0, wantMaxMS: 0},
		{name: "zero", ttl: 0, wantMinMS: 1000, wantMaxMS: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRefreshMilliSec(tt.ttl)
			if got < tt.wantMinMS || got > tt.wantMaxMS {
				t.Fatalf("getRefreshMilliSec(%d) = %d, want [%d, %d]", tt.ttl, got, tt.wantMinMS, tt.wantMaxMS)
			}
		})
	}
}

func TestQQBotTokenSourceForceRefresh(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token",
			"expires_in":   "7200",
		})
	}))
	defer server.Close()

	oldURL := tokenURLOverride
	tokenURLOverride = server.URL
	defer func() { tokenURLOverride = oldURL }()

	source := NewQQBotTokenSource(&QQBotCredentials{AppID: "app", AppSecret: "secret"}).(*QQBotTokenSource)
	tk, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if tk.AccessToken != "token" {
		t.Fatalf("access token = %q", tk.AccessToken)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}

	// 缓存有效时不会重复请求
	if _, err := source.Token(); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}

	// ForceRefresh 必须绕过缓存
	if _, err := source.ForceRefresh(); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}

	// Invalidate 后下一次 Token 重新请求
	source.Invalidate()
	if _, err := source.Token(); err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Fatalf("hits = %d, want 3", hits)
	}
}

func TestQQBotTokenSourceInvalidExpiresIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token",
			"expires_in":   0,
		})
	}))
	defer server.Close()

	oldURL := tokenURLOverride
	tokenURLOverride = server.URL
	defer func() { tokenURLOverride = oldURL }()

	source := NewQQBotTokenSource(&QQBotCredentials{AppID: "app", AppSecret: "secret"}).(*QQBotTokenSource)
	if _, err := source.Token(); err == nil {
		t.Fatal("expected error for invalid expires_in")
	}
}

func TestStartRefreshAccessTokenStopsOnContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token",
			"expires_in":   7200,
		})
	}))
	defer server.Close()

	oldURL := tokenURLOverride
	tokenURLOverride = server.URL
	defer func() { tokenURLOverride = oldURL }()

	ctx, cancel := context.WithCancel(context.Background())
	source := NewQQBotTokenSource(&QQBotCredentials{AppID: "app", AppSecret: "secret"}).(*QQBotTokenSource)
	if err := StartRefreshAccessToken(ctx, source); err != nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(20 * time.Millisecond)
}

func TestGetTokenURL(t *testing.T) {
	const want = "https://api.bot.qq.com/app/getAppAccessToken"
	if got := getTokenURL(); got != want {
		t.Fatalf("getTokenURL() = %q, want %q", got, want)
	}
}
