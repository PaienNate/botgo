package dto

import "strings"

// Message 消息结构体定义
type Message struct {
	// 消息ID
	ID string `json:"id"`
	// 子频道ID
	ChannelID string `json:"channel_id"`
	// 频道ID
	GuildID string `json:"guild_id"`
	// 群ID
	GroupID string `json:"group_id"`
	// 群OpenID
	GroupOpenID string `json:"group_openid,omitempty"`

	// 内容
	Content string `json:"content"`
	// 消息类型：0=纯文本，3=ARK卡片，103=引用消息
	MessageType int `json:"message_type,omitempty"`
	// 引用消息时，被引用消息的详细信息
	MsgElements []*MessageElement `json:"msg_elements,omitempty"`
	// 发送时间
	Timestamp Timestamp `json:"timestamp"`
	// 消息编辑时间
	EditedTimestamp Timestamp `json:"edited_timestamp"`
	// 是否@all
	MentionEveryone bool `json:"mention_everyone"`
	// 消息发送方
	Author *User `json:"author"`
	// 消息发送方Author的member属性，只是部分属性
	Member *Member `json:"member"`
	// 附件
	Attachments []*MessageAttachment `json:"attachments"`
	// 结构化消息 embeds
	Embeds []*Embed `json:"embeds"`
	// 消息中的提醒信息(@)列表
	Mentions []*User `json:"mentions"`
	// ark 消息
	Ark *Ark `json:"ark"`
	// ark 结构化数据（事件）
	ARKData *ARKData `json:"ark_data,omitempty"`
	// 私信消息
	DirectMessage bool `json:"direct_message"`
	// 子频道 seq，用于消息间的排序，seq 在同一子频道中按从先到后的顺序递增，不同的子频道之前消息无法排序
	SeqInChannel string `json:"seq_in_channel"`
	// 引用的消息
	MessageReference *MessageReference `json:"message_reference,omitempty"`
	// 私信场景下，该字段用来标识从哪个频道发起的私信
	SrcGuildID string `json:"src_guild_id"`
	// 上传富媒体文件后返回的文件信息。注意以群或者C2C消息上传后， 同类型可以重复使用，不同类型需要不能使用。
	FileInfo []byte `json:"file_info,omitempty"`
	// 上传富媒体文件后的有效期, 单位:秒 在有效期内可以重复使用。
	TTL uint `json:"ttl,omitempty"`
	// 消息场景描述
	MessageScene MessageScene `json:"message_scene,omitempty"`
	// Ret 发送结果返回码；例如超过主动消息限制会返回 22009。
	Ret int `json:"ret,omitempty"`
}

// Embed 结构
type Embed struct {
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Prompt      string                `json:"prompt"` // 消息弹窗内容，消息列表摘要
	Thumbnail   MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Fields      []*EmbedField         `json:"fields,omitempty"`
}

// MessageEmbedThumbnail embed 消息的缩略图对象
type MessageEmbedThumbnail struct {
	URL string `json:"url"`
}

// EmbedField Embed字段描述
type EmbedField struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// MessageElement 引用消息中的被引用消息元素。
type MessageElement struct {
	ID          string               `json:"id,omitempty"`
	MessageID   string               `json:"message_id,omitempty"`
	Content     string               `json:"content,omitempty"`
	Attachments []*MessageAttachment `json:"attachments,omitempty"`
}

// IsQuote 是否是引用消息（message_type=103）。
func (m *Message) IsQuote() bool {
	return m != nil && m.MessageType == 103
}

// QuotedElement 返回引用消息中的第一个被引用元素。
func (m *Message) QuotedElement() *MessageElement {
	if m == nil || len(m.MsgElements) == 0 {
		return nil
	}
	return m.MsgElements[0]
}

// MessageAttachment 附件定义
type MessageAttachment struct {
	URL         string `json:"url,omitempty"`
	FileName    string `json:"filename,omitempty"`
	Height      int    `json:"height,omitempty"`
	Size        int    `json:"size,omitempty"`
	Width       int    `json:"width,omitempty"`
	ContentType string `json:"content_type,omitempty"` // voice:语音, image/xxx: 图片 video/xxx: 视频
}

// MessageReactionUsers 消息表情表态用户列表
type MessageReactionUsers struct {
	Users  []*User `json:"users,omitempty"`
	Cookie string  `json:"cookie,omitempty"`
	IsEnd  bool    `json:"is_end,omitempty"`
}

// MessageScene 消息场景
type MessageScene struct {
	Source       string `json:"source,omitempty"`        // 消息来源, realtime_voice: 实时通话场景, ai_search: AI搜索 其它默认为AIO消息
	CallbackData string `json:"callback_data,omitempty"` // 回调数据
	// Ext 事件扩展信息，形如 ["msg_idx=REFIDX_xxx", "ref_msg_idx=REFIDX_yyy"]。
	// 引用回复时，message_reference.message_id 需要使用这里的 REFIDX_ 值，
	// 不能直接使用消息的 ROBOT1.0_ 开头 id。
	Ext []string `json:"ext,omitempty"`
}

// MsgIdx 返回当前消息自身的引用索引（REFIDX_...），用于出站引用回复。
func (s MessageScene) MsgIdx() string {
	return s.extValue("msg_idx")
}

// RefMsgIdx 返回被引用消息的索引（REFIDX_...）。
func (s MessageScene) RefMsgIdx() string {
	return s.extValue("ref_msg_idx")
}

func (s MessageScene) extValue(key string) string {
	prefix := key + "="
	for _, item := range s.Ext {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(item, prefix))
		}
	}
	return ""
}

// ResolvedReferenceID 返回可用于 message_reference.message_id 的引用 ID。
//
// 官方规则：
//   - 机器人自己发送的消息：可以直接使用 message_reference.message_id；
//   - 用户消息/非机器人消息：必须使用 message_scene.ext 中的 msg_idx（REFIDX_...），
//     message.id（ROBOT1.0_...）不能直接用于引用。
//
// 这里优先返回显式 REFIDX_，其次返回 msg_idx / ref_msg_idx，
// 最后才回退到原始 message_reference.message_id（仅建议用于机器人消息）。
func (m *Message) ResolvedReferenceID() string {
	if m == nil {
		return ""
	}
	if m.MessageReference != nil {
		id := strings.TrimSpace(m.MessageReference.MessageID)
		if strings.HasPrefix(id, "REFIDX_") {
			return id
		}
	}
	if id := m.MessageScene.MsgIdx(); id != "" {
		return id
	}
	if id := m.MessageScene.RefMsgIdx(); id != "" {
		return id
	}
	if m.MessageReference != nil {
		return strings.TrimSpace(m.MessageReference.MessageID)
	}
	return ""
}
