package dto

// Menu 全局自定义菜单。
type Menu struct {
	Items []*MenuItem `json:"items,omitempty"`
}

// MenuItem 菜单项。
type MenuItem struct {
	Name         string         `json:"name,omitempty"`
	Type         string         `json:"type,omitempty"` // switch / send_message / link / menu
	SubMenuItems []*SubMenuItem `json:"sub_menu_items,omitempty"`
	SendMessage  string         `json:"send_message,omitempty"`
	Link         string         `json:"link,omitempty"`
	Switch       *MenuSwitch    `json:"switch,omitempty"`
}

// SubMenuItem 子菜单项。
type SubMenuItem struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"` // send_message / link
	SendMessage string `json:"send_message,omitempty"`
	Link        string `json:"link,omitempty"`
}

// MenuSwitch 菜单开关配置。
type MenuSwitch struct {
	SwitchID string `json:"switch_id,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

// GetMenuResp 查询全局自定义菜单响应。
type GetMenuResp struct {
	Version int   `json:"version"`
	Menu    *Menu `json:"menu,omitempty"`
}

// PutMenuReq 修改全局自定义菜单请求。
type PutMenuReq struct {
	Menu *Menu `json:"menu,omitempty"`
}

// PutMenuResp 修改全局自定义菜单响应。
type PutMenuResp struct {
	Version int `json:"version"`
}
