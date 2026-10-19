package webhook

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sealdice/botgo/token"
)

func TestServerValidationAndUnknownAppID(t *testing.T) {
	server := NewServer(ServerOptions{
		Resolve: func(appID string) (*token.QQBotCredentials, error) {
			if appID == "app" {
				return &token.QQBotCredentials{AppID: "app", AppSecret: "secret"}, nil
			}
			return nil, errors.New("unknown appid")
		},
	})
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body := `{"op":13,"d":{"plain_token":"plain","event_ts":"123456"}}`
	req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Bot-Appid", "app")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	rspBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, string(rspBody))
	}
	if !strings.Contains(string(rspBody), `"plain_token":"plain"`) {
		t.Fatalf("unexpected body: %s", string(rspBody))
	}

	req, err = http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Bot-Appid", "unknown")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown appid status = %d, want 401", resp.StatusCode)
	}
}
