package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type testTokenSource struct{}

func (testTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "token", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestSessionJSONOmitsTokenSource(t *testing.T) {
	session := Session{
		ID:          "id",
		URL:         "wss://example.invalid",
		TokenSource: testTokenSource{},
		Intent:      1,
		LastSeq:     2,
		Shards:      ShardConfig{ShardID: 0, ShardCount: 1},
		AppID:       "app",
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "TokenSource") {
		t.Fatalf("session JSON should omit TokenSource: %s", string(data))
	}

	var decoded Session
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TokenSource != nil {
		t.Fatal("decoded TokenSource should be nil")
	}
	if decoded.AppID != "app" || decoded.Shards.ShardID != 0 || decoded.LastSeq != 2 {
		t.Fatalf("unexpected decoded session: %+v", decoded)
	}
}
