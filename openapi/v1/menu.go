package v1

import (
	"context"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.MenuAPI = (*openAPI)(nil)

// GetMenu 查询全局自定义菜单。
func (o *openAPI) GetMenu(ctx context.Context) (*dto.GetMenuResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GetMenuResp{}).
		Get(o.getURL(menuURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetMenuResp), nil
}

// PutMenu 修改全局自定义菜单。
func (o *openAPI) PutMenu(ctx context.Context, req *dto.PutMenuReq) (*dto.PutMenuResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.PutMenuResp{}).
		SetBody(req).
		Put(o.getURL(menuURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.PutMenuResp), nil
}
