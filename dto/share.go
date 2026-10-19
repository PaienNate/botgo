package dto

// GenerateURLLinkReq 生成机器人分享链接请求。
type GenerateURLLinkReq struct {
	CallbackData string `json:"callback_data,omitempty"` // 回传给机器人后台的数据，最长 32 字符
}

// GenerateURLLinkData 分享链接数据。
type GenerateURLLinkData struct {
	URL string `json:"url"`
}

// GenerateURLLinkResp 生成机器人分享链接响应。
type GenerateURLLinkResp struct {
	Data GenerateURLLinkData `json:"data"`
}
