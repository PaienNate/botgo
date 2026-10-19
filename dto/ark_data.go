package dto

// ARKData 事件中的结构化卡片数据。
type ARKData struct {
	Prompt  string            `json:"prompt,omitempty"`
	ArkType string            `json:"ark_type,omitempty"`
	ArkName string            `json:"ark_name,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// ark_type 取值
const (
	ArkTypeTuwen         = "tuwen"          // 图文 H5
	ArkTypeFeed          = "feed"           // 图文卡片
	ArkTypeMiniApp       = "miniapp"        // 小程序
	ArkTypeMap           = "map"            // 位置卡片
	ArkTypeContactCard   = "contact_card"   // 好友名片
	ArkTypeVideoShare    = "video_share"    // 视频分享
	ArkTypeMusicTogether = "music_together" // 一起听歌
	ArkTypePicture       = "picture"        // 图片（20260916 新增）
)
