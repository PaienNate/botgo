package dto

// Panel 指令面板配置。
type Panel struct {
	Items   []*PanelItem `json:"items,omitempty"`
	Remark  string       `json:"remark,omitempty"`
	Version int          `json:"version,omitempty"`
}

// PanelItem 面板元素。
type PanelItem struct {
	Name      string `json:"name,omitempty"`
	Desc      string `json:"desc,omitempty"`
	Type      string `json:"type,omitempty"` // command / link
	OnlyAdmin bool   `json:"only_admin,omitempty"`
	Link      string `json:"link,omitempty"`
}

// PanelRecord 面板记录。
type PanelRecord struct {
	PanelID    string `json:"panel_id"`
	Scope      string `json:"scope"`
	TargetType string `json:"target_type"`
	Panel      *Panel `json:"panel"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	Version    int    `json:"version,omitempty"`
}

// CreatePanelReq 创建指令面板请求。
type CreatePanelReq struct {
	Scope        string   `json:"scope"` // c2c / group / channel / dm
	TargetType   string   `json:"target_type,omitempty"`
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	Panel        *Panel   `json:"panel"`
}

// CreatePanelResp 创建指令面板响应。
type CreatePanelResp struct {
	PanelID string `json:"panel_id"`
}

// GetPanelResp 查询指令面板详情响应。
type GetPanelResp struct {
	PanelID      string   `json:"panel_id"`
	Scope        string   `json:"scope"`
	TargetType   string   `json:"target_type"`
	Panel        *Panel   `json:"panel"`
	CreatedAt    string   `json:"created_at,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	Version      int      `json:"version,omitempty"`
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}

// UpdatePanelReq 修改指令面板请求。
type UpdatePanelReq struct {
	Panel *Panel `json:"panel"`
}

// UpdatePanelResp 修改指令面板响应。
type UpdatePanelResp struct {
	Version int `json:"version"`
}

// ListPanelsResp 查询指令面板列表响应。
type ListPanelsResp struct {
	Records    []*PanelRecord `json:"records"`
	NextCursor string         `json:"next_cursor"`
	IsEnd      bool           `json:"is_end"`
}

// UpdatePanelTargetReq 修改指令面板关联对象请求。
type UpdatePanelTargetReq struct {
	Op           string   `json:"op"` // add / del
	UserOpenIDs  []string `json:"user_openids,omitempty"`
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}
