package dedup

import (
	"testing"

	"github.com/sealdice/botgo/dto"
)

type memoryDeduper struct {
	seen map[string]bool
}

func (d *memoryDeduper) Seen(key string) bool { return d.seen[key] }
func (d *memoryDeduper) Mark(key string)      { d.seen[key] = true }
func (d *memoryDeduper) Unmark(key string)    { delete(d.seen, key) }

func TestKeysMessageEventUsesEventAndMessageID(t *testing.T) {
	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			EventID: "event-1",
			Type:    dto.EventGroupAtMessageCreate,
		},
		RawMessage: []byte(`{"id":"event-1","d":{"id":"msg-1","content":"hi"}}`),
	}
	keys := Keys(payload)
	if len(keys) != 2 || keys[0] != "event:event-1" || keys[1] != "msg:msg-1" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestKeysNonMessageEventUsesEventIDOnly(t *testing.T) {
	payload := &dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{
			EventID: "event-2",
			Type:    dto.EventGroupMemberAdd,
		},
		RawMessage: []byte(`{"id":"event-2","d":{"member_openid":"u1"}}`),
	}
	keys := Keys(payload)
	if len(keys) != 1 || keys[0] != "event:event-2" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestSeenAnyMessageIDDedup(t *testing.T) {
	d := &memoryDeduper{seen: map[string]bool{}}
	first := Keys(&dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{EventID: "event-1", Type: dto.EventGroupAtMessageCreate},
		RawMessage:    []byte(`{"id":"event-1","d":{"id":"msg-1"}}`),
	})
	second := Keys(&dto.WSPayload{
		WSPayloadBase: dto.WSPayloadBase{EventID: "event-99", Type: dto.EventGroupAtMessageCreate},
		RawMessage:    []byte(`{"id":"event-99","d":{"id":"msg-1"}}`),
	})
	MarkAll(d, first)
	if !SeenAny(d, second) {
		t.Fatal("same msg id with different event id should be duplicate")
	}
}
