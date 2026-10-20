package dto

import (
	"encoding/json"
	"testing"
)

func TestMessageSceneMsgIdx(t *testing.T) {
	var msg Message
	body := `{"id":"ROBOT1.0_x","message_scene":{"ext":["msg_idx=REFIDX_self","ref_msg_idx=REFIDX_ref"]}}`
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		t.Fatal(err)
	}
	if got := msg.MessageScene.MsgIdx(); got != "REFIDX_self" {
		t.Fatalf("MsgIdx = %q, want REFIDX_self", got)
	}
	if got := msg.MessageScene.RefMsgIdx(); got != "REFIDX_ref" {
		t.Fatalf("RefMsgIdx = %q, want REFIDX_ref", got)
	}
	if got := msg.ResolvedReferenceID(); got != "REFIDX_self" {
		t.Fatalf("ResolvedReferenceID = %q, want REFIDX_self", got)
	}
}

func TestResolvedReferenceIDPrefersExplicitREFIDX(t *testing.T) {
	msg := &Message{
		MessageReference: &MessageReference{MessageID: "REFIDX_explicit"},
		MessageScene:     MessageScene{Ext: []string{"msg_idx=REFIDX_self"}},
	}
	if got := msg.ResolvedReferenceID(); got != "REFIDX_explicit" {
		t.Fatalf("ResolvedReferenceID = %q, want REFIDX_explicit", got)
	}
}

func TestResolvedReferenceIDPrefersRefMsgIdxOverRawID(t *testing.T) {
	msg := &Message{
		MessageReference: &MessageReference{MessageID: "ROBOT1.0_quoted"},
		MessageScene:     MessageScene{Ext: []string{"ref_msg_idx=REFIDX_quoted"}},
	}
	if got := msg.ResolvedReferenceID(); got != "REFIDX_quoted" {
		t.Fatalf("ResolvedReferenceID = %q, want REFIDX_quoted", got)
	}
}

func TestResolvedReferenceIDFallsBackToRawID(t *testing.T) {
	msg := &Message{MessageReference: &MessageReference{MessageID: "ROBOT1.0_bot"}}
	if got := msg.ResolvedReferenceID(); got != "ROBOT1.0_bot" {
		t.Fatalf("ResolvedReferenceID = %q, want ROBOT1.0_bot", got)
	}
}

func TestMessageQuoteFields(t *testing.T) {
	var msg Message
	body := `{
		"id":"ROBOT1.0_current",
		"message_type":103,
		"msg_elements":[{
			"id":"ROBOT1.0_quoted",
			"content":"被引用的内容",
			"attachments":[{"url":"https://example.com/a.png","content_type":"image/png"}]
		}],
		"message_scene":{"ext":["msg_idx=REFIDX_current","ref_msg_idx=REFIDX_quoted"]}
	}`
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		t.Fatal(err)
	}
	if !msg.IsQuote() {
		t.Fatal("IsQuote = false, want true")
	}
	quoted := msg.QuotedElement()
	if quoted == nil {
		t.Fatal("QuotedElement = nil")
	}
	if quoted.ID != "ROBOT1.0_quoted" || quoted.Content != "被引用的内容" {
		t.Fatalf("unexpected quoted element: %+v", quoted)
	}
	if len(quoted.Attachments) != 1 || quoted.Attachments[0].ContentType != "image/png" {
		t.Fatalf("unexpected attachments: %+v", quoted.Attachments)
	}
}
