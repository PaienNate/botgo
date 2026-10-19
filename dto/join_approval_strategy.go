package dto

// JoinApprovalStrategy 入群自动审批策略。
type JoinApprovalStrategy struct {
	StrategyID         string   `json:"strategy_id"`
	GroupOpenIDs       []string `json:"group_openids,omitempty"`
	GroupIDs           []uint64 `json:"group_ids,omitempty"`
	WhitelistUserCount int      `json:"whitelist_user_count"`
	IsEnable           string   `json:"is_enable"` // on / off
	ExpireAt           string   `json:"expire_at,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
	UpdatedAt          string   `json:"updated_at,omitempty"`
	Remark             string   `json:"remark,omitempty"`
}

// GetJoinApprovalStrategiesResp 入群自动审批策略列表响应。
type GetJoinApprovalStrategiesResp struct {
	Strategies []*JoinApprovalStrategy `json:"strategies"`
	NextCursor string                  `json:"next_cursor"`
}

// CreateJoinApprovalStrategyReq 创建入群自动审批策略请求。
type CreateJoinApprovalStrategyReq struct {
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	GroupIDs     []uint64 `json:"group_ids,omitempty"`
	IsEnable     string   `json:"is_enable,omitempty"`
	ExpireAt     string   `json:"expire_at,omitempty"`
	Remark       string   `json:"remark,omitempty"`
}

// CreateJoinApprovalStrategyResp 创建入群自动审批策略响应。
type CreateJoinApprovalStrategyResp struct {
	StrategyID string `json:"strategy_id"`
	IsEnable   string `json:"is_enable"`
	ExpireAt   string `json:"expire_at"`
}

// GroupAction 策略关联群增删操作。
type GroupAction struct {
	Op           string   `json:"op"` // add / del
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	GroupIDs     []uint64 `json:"group_ids,omitempty"`
}

// UpdateJoinApprovalStrategyReq 修改入群自动审批策略请求。
type UpdateJoinApprovalStrategyReq struct {
	IsEnable    string       `json:"is_enable,omitempty"`
	ExpireAt    string       `json:"expire_at,omitempty"`
	GroupAction *GroupAction `json:"group_action,omitempty"`
	Remark      string       `json:"remark,omitempty"`
}

// UpdateJoinApprovalStrategyResp 修改入群自动审批策略响应。
type UpdateJoinApprovalStrategyResp struct {
	IsEnable string `json:"is_enable"`
	ExpireAt string `json:"expire_at"`
}

// UpdateJoinApprovalWhitelistReq 修改策略白名单请求。
type UpdateJoinApprovalWhitelistReq struct {
	Op             string   `json:"op"` // add / del
	WhitelistUsers []string `json:"whitelist_users"`
}

// UpdateJoinApprovalWhitelistResp 修改策略白名单响应。
type UpdateJoinApprovalWhitelistResp struct {
	StrategyID         string `json:"strategy_id"`
	WhitelistUserCount int    `json:"whitelist_user_count"`
	UpdatedAt          string `json:"updated_at"`
}
