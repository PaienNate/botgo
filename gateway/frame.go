package gateway

import (
	"encoding/json"
	"time"

	"github.com/sealdice/botgo/dto"
)

func nowUnix() int64 { return time.Now().Unix() }

// FrameMode 控制推送给客户端的帧格式。
type FrameMode int

const (
	// FrameModeEnvelope 版本化信封，推荐默认值。
	FrameModeEnvelope FrameMode = iota
	// FrameModeRaw 直接推送官方 dto.WSPayload。
	FrameModeRaw
	// FrameModeBB 兼容 Better-Bot-Go 的 onebot.Frame 字段。
	FrameModeBB
)

// Frame 版本化信封。
type Frame struct {
	V       int         `json:"v,omitempty"`
	AppID   string      `json:"appid,omitempty"`
	Type    string      `json:"type,omitempty"` // hello / event / ack / error / pong
	ID      string      `json:"id,omitempty"`
	TS      int64       `json:"ts,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

// BBFrame 兼容 Better-Bot-Go 的字段结构。
type BBFrame struct {
	BotID   string          `json:"bot_id,omitempty"`
	Echo    string          `json:"echo,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Time    int64           `json:"time,omitempty"`
	SelfID  string          `json:"self_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Payload *dto.WSPayload  `json:"payload,omitempty"`
}

func marshalFrame(mode FrameMode, appID string, payload *dto.WSPayload) ([]byte, error) {
	switch mode {
	case FrameModeRaw:
		return json.Marshal(payload)
	case FrameModeBB:
		data, _ := json.Marshal(payload)
		return json.Marshal(BBFrame{
			BotID:   appID,
			OK:      true,
			Time:    nowUnix(),
			SelfID:  appID,
			Data:    data,
			Payload: payload,
		})
	default:
		return json.Marshal(Frame{
			V:       1,
			AppID:   appID,
			Type:    "event",
			ID:      payload.EventID,
			TS:      nowUnix(),
			Payload: payload,
		})
	}
}

func marshalHello(mode FrameMode, appID string) []byte {
	if mode != FrameModeEnvelope {
		return nil
	}
	data, _ := json.Marshal(Frame{
		V:     1,
		AppID: appID,
		Type:  "hello",
		TS:    nowUnix(),
	})
	return data
}
