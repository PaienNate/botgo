package openapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestRunHTTPFilterRecoversPanic(t *testing.T) {
	err := runHTTPFilter("panic-filter", func(_ *http.Request, _ *http.Response) error {
		panic("filter boom")
	}, nil, nil)
	if err == nil {
		t.Fatal("expected filter panic error")
	}
	if !strings.Contains(err.Error(), "filter boom") || !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("error should contain panic value and stack: %v", err)
	}
}
