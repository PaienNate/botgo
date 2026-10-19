package dto

// WSGroupRobotEventData 机器人加入/退出群聊事件数据
type WSGroupRobotEventData struct {
	GroupOpenID    string `json:"group_openid"`     // 群OpenID
	OpMemberOpenID string `json:"op_member_openid"` // 操作者OpenID
	Timestamp      int64  `json:"timestamp"`        // 时间戳（秒）
}

// WSGroupMemberAddData 群成员增加事件数据，对应 GROUP_MEMBER_ADD 事件
type WSGroupMemberAddData struct {
	GroupOpenID    string `json:"group_openid"`     // 群OpenID
	MemberOpenID   string `json:"member_openid"`    // 新成员OpenID
	OpMemberOpenID string `json:"op_member_openid"` // 操作者OpenID
	Timestamp      int64  `json:"timestamp"`        // 时间戳（秒）
}

// WSGroupMemberRemoveData 群成员减少事件数据，对应 GROUP_MEMBER_REMOVE 事件
type WSGroupMemberRemoveData struct {
	GroupOpenID    string `json:"group_openid"`     // 群OpenID
	MemberOpenID   string `json:"member_openid"`    // 被移除成员OpenID
	OpMemberOpenID string `json:"op_member_openid"` // 操作者OpenID
	Timestamp      int64  `json:"timestamp"`        // 时间戳（秒）
}

// WSGroupMsgReceiveData 群聊消息接收开启事件，对应 GROUP_MSG_RECEIVE
type WSGroupMsgReceiveData struct {
	Timestamp      int64  `json:"timestamp"`        // 操作时间戳（秒）
	GroupOpenID    string `json:"group_openid"`     // 群OpenID
	OpMemberOpenID string `json:"op_member_openid"` // 操作者OpenID
}

// WSGroupMsgRejectData 群聊消息接收关闭事件，对应 GROUP_MSG_REJECT
type WSGroupMsgRejectData struct {
	Timestamp      int64  `json:"timestamp"`        // 操作时间戳（秒）
	GroupOpenID    string `json:"group_openid"`     // 群OpenID
	OpMemberOpenID string `json:"op_member_openid"` // 操作者OpenID
}

// WSGroupJoinRequestData 用户申请入群事件，对应 GROUP_JOIN_REQUEST
type WSGroupJoinRequestData struct {
	GroupOpenID   string        `json:"group_openid"`    // 群OpenID
	JoinRequestID string        `json:"join_request_id"` // 申请ID
	RiskTips      string        `json:"risk_tips"`       // 安全提示语
	UnionOpenID   string        `json:"union_openid"`    // 统一标识
	MemberOpenID  string        `json:"member_openid"`   // 申请人 openid
	Username      string        `json:"username"`        // 申请人昵称
	ApplyAt       string        `json:"apply_at"`        // 申请时间戳（RFC3339）
	ApplySource   string        `json:"apply_source"`    // self_apply / invited
	InvitedBy     string        `json:"invited_by"`      // 邀请人 openid
	Bot           bool          `json:"bot"`             // 是否机器人
	VerifyInfo    *VerifyInfo   `json:"verify_info"`     // 入群验证方式
	AutoApproved  *AutoApproved `json:"auto_approved"`   // 自动审批信息
}

// VerifyInfo 入群验证信息
type VerifyInfo struct {
	Method        string      `json:"method"`
	VerifyMessage string      `json:"verify_message"`
	ReviewQAList  []*ReviewQA `json:"review_qa_list"`
}

// ReviewQA 入群问答
type ReviewQA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// AutoApproved 自动审批通过信息
type AutoApproved struct {
	StrategyID string `json:"strategy_id"`
}
