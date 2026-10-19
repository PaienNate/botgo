package dto

// BatchRemoveMembersReq 群成员批量移除请求。
type BatchRemoveMembersReq struct {
	MemberOpenIDs        []string `json:"member_openids"`
	AddToMemberBlacklist bool     `json:"add_to_member_blacklist,omitempty"`
}

// BatchRemoveMembersResp 群成员批量移除响应。
type BatchRemoveMembersResp struct {
	RemoveMembersResult             string   `json:"remove_members_result"`
	AddToMemberBlacklistFailOpenIDs []string `json:"add_to_member_blacklist_fail_openids"`
}

// BlacklistUser 群黑名单用户。
type BlacklistUser struct {
	UnionOpenID  string `json:"union_openid,omitempty"`
	MemberOpenID string `json:"member_openid"`
	Username     string `json:"username,omitempty"`
	BannedAt     string `json:"banned_at,omitempty"`
	Bot          bool   `json:"bot,omitempty"`
}

// GetGroupBlacklistResp 群黑名单查询响应。
type GetGroupBlacklistResp struct {
	Users      []*BlacklistUser `json:"users"`
	NextCursor string           `json:"next_cursor"`
}

// UpdateGroupBlacklistReq 群黑名单操作请求。
type UpdateGroupBlacklistReq struct {
	Op            string   `json:"op"` // add / del
	MemberOpenIDs []string `json:"member_openids"`
}

// UpdateGroupBlacklistResp 群黑名单操作响应。
type UpdateGroupBlacklistResp struct {
	FailOpenIDs []string `json:"fail_openids"`
}

// GroupMuteSetting 群禁言状态。
type GroupMuteSetting struct {
	GlobalRule *GlobalMuteRule    `json:"global_rule"`
	Members    []*MemberMuteState `json:"members"`
}

// GlobalMuteRule 群级禁言规则。
type GlobalMuteRule struct {
	Mode           string               `json:"mode"` // none / always / schedule
	ScheduleRules  []*MuteScheduleRule  `json:"schedule_rules,omitempty"`
	RecurringRules []*MuteRecurringRule `json:"recurring_rules,omitempty"`
}

// MuteScheduleRule 定时禁言规则。
type MuteScheduleRule struct {
	TaskID  string `json:"task_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
	Enabled bool   `json:"enabled"`
}

// MuteRecurringRule 周期禁言规则。
type MuteRecurringRule struct {
	TaskID    string `json:"task_id"`
	Weekdays  []int  `json:"weekdays"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Enabled   bool   `json:"enabled"`
}

// MemberMuteState 成员禁言状态。
type MemberMuteState struct {
	MemberOpenID string `json:"member_openid"`
	MuteExpireAt string `json:"mute_expire_at"`
	Username     string `json:"username,omitempty"`
	UnionOpenID  string `json:"union_openid,omitempty"`
}

// SetGroupMuteReq 设置群成员禁言请求。
type SetGroupMuteReq struct {
	Members []*SetMemberMuteState `json:"members,omitempty"`
}

// SetMemberMuteState 单个成员的禁言操作。
type SetMemberMuteState struct {
	Op           string `json:"op"` // add / update / del
	MemberOpenID string `json:"member_openid"`
	MuteExpireAt string `json:"mute_expire_at,omitempty"`
}
