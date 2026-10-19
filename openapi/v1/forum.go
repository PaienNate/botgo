package v1

import (
	"context"
	"net/http"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
	"github.com/sealdice/botgo/openapi/options"
)

var _ openapi.ForumAPI = (*openAPI)(nil)

// PostForumThread 发表论坛帖子。
func (o *openAPI) PostForumThread(
	ctx context.Context, channelID string, msg *dto.ForumThreadToCreate, opt ...options.Option,
) (*dto.ForumThreadResp, error) {
	req := o.request(ctx).
		SetResult(dto.ForumThreadResp{}).
		SetPathParam("channel_id", channelID).
		SetBody(msg)
	resp, err := baseRequest(ctx, req, http.MethodPut, o.getURL(forumThreadsURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.ForumThreadResp), nil
}
