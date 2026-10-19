package v1

import (
	"context"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.ShareAPI = (*openAPI)(nil)

// GenerateURLLink 生成机器人分享链接。
func (o *openAPI) GenerateURLLink(ctx context.Context, callbackData string) (*dto.GenerateURLLinkResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GenerateURLLinkResp{}).
		SetBody(&dto.GenerateURLLinkReq{CallbackData: callbackData}).
		Post(o.getURL(generateURLLinkURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GenerateURLLinkResp), nil
}
