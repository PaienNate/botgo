package v1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
	"github.com/sealdice/botgo/openapi/options"
)

func TestPostMessageMultipart(t *testing.T) {
	fileData := []byte("fake-jpeg-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			return
		}
		if got := r.FormValue("content"); got != "hi" {
			t.Errorf("content = %q", got)
		}
		if got := r.FormValue("msg_id"); got != "msg-1" {
			t.Errorf("msg_id = %q", got)
		}
		if got := r.FormValue("message_reference"); got != "" {
			t.Errorf("message_reference should be omitted when file_image is present, got %q", got)
		}
		file, header, err := r.FormFile("file_image")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		data, _ := io.ReadAll(file)
		if string(data) != string(fileData) {
			t.Errorf("file_image = %q, want %q", string(data), string(fileData))
		}
		if header.Filename == "" {
			t.Error("missing filename")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg-1","content":"hi"}`))
	}))
	defer server.Close()

	api := (&openAPI{}).Setup("app", &retryTokenSource{}, false).WithTimeout(3 * time.Second)
	msg, err := api.(openapi.FormFileAPI).PostMessageMultipart(context.Background(), "channel", &dto.MessageToCreate{
		Content:          "hi",
		MsgID:            "msg-1",
		MessageReference: &dto.MessageReference{MessageID: "REFIDX_ref"},
	}, fileData, options.WithURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.ID != "msg-1" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestPostMessageDropsMsgTypeForChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			return
		}
		if _, ok := body["msg_type"]; ok {
			t.Errorf("channel message should not contain msg_type: %#v", body)
		}
		if _, ok := body["markdown"]; !ok {
			t.Errorf("channel message should keep markdown: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer server.Close()

	api := (&openAPI{}).Setup("app", &retryTokenSource{}, false).WithTimeout(3 * time.Second)
	_, err := api.PostMessage(context.Background(), "channel", &dto.MessageToCreate{
		MsgType:  dto.MarkdownMsg,
		Markdown: &dto.Markdown{Content: "**hi**"},
	}, options.WithURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostForumThread(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body dto.ForumThreadToCreate
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
			return
		}
		if body.Title != "title" || body.Content != "content" || body.Format != dto.ForumFormatMarkdown {
			t.Errorf("unexpected body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"task-1","create_time":"123"}`))
	}))
	defer server.Close()

	api := (&openAPI{}).Setup("app", &retryTokenSource{}, false).WithTimeout(3 * time.Second)
	rsp, err := api.(openapi.ForumAPI).PostForumThread(context.Background(), "channel", &dto.ForumThreadToCreate{
		Title:   "title",
		Content: "content",
		Format:  dto.ForumFormatMarkdown,
	}, options.WithURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if rsp == nil || rsp.TaskID != "task-1" {
		t.Fatalf("unexpected response: %+v", rsp)
	}
}
