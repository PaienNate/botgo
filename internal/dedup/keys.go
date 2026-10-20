// Package dedup 提供 QQ 官方事件去重 key 的构造与批量操作。
//
// 官方明确说明消息可能重复推送（相同 msg_id），因此消息事件需要同时使用：
//   - 顶层事件 id（event id）
//   - 消息体 d.id（msg_id）
//
// 任一 key 命中都视为重复。
package dedup

import (
	"github.com/tidwall/gjson"

	"github.com/sealdice/botgo/dto"
)

// Deduper 去重器需要实现的最小接口。
type Deduper interface {
	Seen(key string) bool
	Mark(key string)
	Unmark(key string)
}

// Keys 返回一个事件的全部去重 key。空切片表示不参与去重。
func Keys(payload *dto.WSPayload) []string {
	if payload == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if payload.EventID != "" {
		keys = append(keys, "event:"+payload.EventID)
	}
	if isMessageEvent(payload.Type) && len(payload.RawMessage) > 0 {
		if msgID := gjson.GetBytes(payload.RawMessage, "d.id").String(); msgID != "" {
			keys = append(keys, "msg:"+msgID)
		}
	}
	return keys
}

func isMessageEvent(t dto.EventType) bool {
	switch t {
	case dto.EventMessageCreate,
		dto.EventAtMessageCreate,
		dto.EventDirectMessageCreate,
		dto.EventGroupAtMessageCreate,
		dto.EventGroupMessageCreate,
		dto.EventC2CMessageCreate:
		return true
	default:
		return false
	}
}

// SeenAny 任一 key 命中即返回 true。
func SeenAny(d Deduper, keys []string) bool {
	if d == nil || len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if d.Seen(key) {
			return true
		}
	}
	return false
}

// MarkAll 标记全部 key。
func MarkAll(d Deduper, keys []string) {
	if d == nil {
		return
	}
	for _, key := range keys {
		d.Mark(key)
	}
}

// UnmarkAll 取消全部 key。
func UnmarkAll(d Deduper, keys []string) {
	if d == nil {
		return
	}
	for _, key := range keys {
		d.Unmark(key)
	}
}
