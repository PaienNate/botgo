package dto

// GroupMember 群成员信息
type GroupMember struct {
	MemberOpenID string `json:"member_openid"` // 成员 OpenID
	Username     string `json:"username"`      // 用户昵称
	MemberRole   string `json:"member_role"`   // member / owner / admin
	Bot          bool   `json:"bot"`           // 是否机器人
	JoinedAt     string `json:"joined_at"`     // 入群时间戳（RFC3339）
	UnionOpenID  string `json:"union_openid"`  // 统一标识
}

// GetGroupMembersResp 群成员列表响应
type GetGroupMembersResp struct {
	Members    []*GroupMember `json:"members"`
	NextCursor string         `json:"next_cursor"` // 空串表示已到末页
}

// GroupInfo 群基本信息
type GroupInfo struct {
	GroupOpenID     string   `json:"group_openid"`
	GroupName       string   `json:"group_name"`
	GroupFingerMemo string   `json:"group_finger_memo"`
	GroupClassText  string   `json:"group_class_text"`
	GroupTags       []string `json:"group_tags"`
	GroupMemberNum  int      `json:"group_member_num"`
}

// GroupBotState 机器人群内状态
type GroupBotState struct {
	MemberOpenID      string `json:"member_openid"`
	JoinedAt          string `json:"joined_at"`
	AllowProactiveMsg bool   `json:"allow_proactive_msg"`
	RecvMsgSetting    string `json:"recv_msg_setting"` // all / only_mention / mention_and_context
	MemberRole        string `json:"member_role"`
}
