package dto

// JoinRequest 入群申请。
type JoinRequest struct {
	JoinRequestID string      `json:"join_request_id"`
	RiskTips      string      `json:"risk_tips,omitempty"`
	UnionOpenID   string      `json:"union_openid,omitempty"`
	MemberOpenID  string      `json:"member_openid"`
	Username      string      `json:"username,omitempty"`
	ApplyAt       string      `json:"apply_at,omitempty"`
	ApplySource   string      `json:"apply_source,omitempty"`
	InvitedBy     string      `json:"invited_by,omitempty"`
	Bot           bool        `json:"bot,omitempty"`
	VerifyInfo    *VerifyInfo `json:"verify_info,omitempty"`
}

// GetJoinRequestsResp 入群申请列表响应。
type GetJoinRequestsResp struct {
	List       []*JoinRequest `json:"list"`
	NextCursor string         `json:"next_cursor"`
}

// ApproveJoinRequestReq 入群申请审批请求。
type ApproveJoinRequestReq struct {
	Op                   string `json:"op"` // approve / decline
	JoinRequestID        string `json:"join_request_id,omitempty"`
	RejectReason         string `json:"reject_reason,omitempty"`
	AddToMemberBlacklist bool   `json:"add_to_member_blacklist,omitempty"`
}
