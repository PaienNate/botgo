package dto

// MessageMediaToCreate 创建富媒体消息
type MessageMediaToCreate struct {
	FileType   int    `json:"file_type"`
	URL        string `json:"url"`
	SrvSendMsg bool   `json:"srv_send_msg"`
	FileData   []byte `json:"file_data"`
	// UploadID 分片上传任务 ID（upload_prepare 返回），走分片上传合并路径时填写。
	UploadID string `json:"upload_id,omitempty"`
	// FileName 文件名，分片上传或二进制上传时必填。
	FileName string `json:"file_name,omitempty"`
}

// Media 富媒体上传响应
type Media struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	Ttl      int    `json:"ttl"`
	// ID 发送消息的唯一 ID，仅 srv_send_msg=true 时返回
	ID string `json:"id,omitempty"`
	// RawURL 文件下载链接（COS 预签名 GET URL），
	// 仅分片上传合并且 file_type 为图片/视频/语音时返回
	RawURL string `json:"raw_url,omitempty"`
}
