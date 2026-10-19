package dto

// StreamMessageToCreate 流式发送单聊消息
type StreamMessageToCreate struct {
	InputMode   string `json:"input_mode,omitempty"`   // append（默认）/ replace
	InputState  int    `json:"input_state,omitempty"`  // 1=生成中，10=生成结束
	Index       int    `json:"index,omitempty"`        // 分片序号，从 0 递增
	ContentType string `json:"content_type,omitempty"` // text / markdown
	ContentRaw  string `json:"content_raw,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	MsgID       string `json:"msg_id,omitempty"`
	StreamMsgID string `json:"stream_msg_id,omitempty"`
	MsgSeq      int    `json:"msg_seq,omitempty"`
	IsWakeup    bool   `json:"is_wakeup,omitempty"`
}

// StreamMessageResp 流式消息响应
type StreamMessageResp struct {
	ID           string          `json:"id"`
	Timestamp    string          `json:"timestamp"`
	ExtInfo      *MessageExtInfo `json:"ext_info,omitempty"`
	RemainMsgLen int             `json:"remain_msg_len,omitempty"`
}

// MessageExtInfo 消息扩展信息
type MessageExtInfo struct {
	RefIdx string `json:"ref_idx,omitempty"`
}
