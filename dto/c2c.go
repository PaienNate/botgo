package dto

// WSC2CMsgReceiveData 单聊消息接收开启事件，对应 C2C_MSG_RECEIVE
type WSC2CMsgReceiveData struct {
	Timestamp int64  `json:"timestamp"` // 操作时间戳（秒）
	OpenID    string `json:"openid"`    // 用户 openid
}

// WSC2CMsgRejectData 单聊消息接收关闭事件，对应 C2C_MSG_REJECT
type WSC2CMsgRejectData struct {
	Timestamp int64  `json:"timestamp"` // 操作时间戳（秒）
	OpenID    string `json:"openid"`    // 用户 openid
}
