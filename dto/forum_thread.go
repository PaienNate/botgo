package dto

// ForumThreadToCreate 发表论坛帖子请求。
type ForumThreadToCreate struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Format  uint32 `json:"format"` // 1=text, 2=html, 3=markdown, 4=json
}

// 论坛帖子文本格式
const (
	ForumFormatText     uint32 = 1
	ForumFormatHTML     uint32 = 2
	ForumFormatMarkdown uint32 = 3
	ForumFormatJSON     uint32 = 4
)

// ForumThreadResp 发表论坛帖子响应。
type ForumThreadResp struct {
	TaskID     string `json:"task_id"`
	CreateTime string `json:"create_time"`
}
