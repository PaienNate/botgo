package v1

import (
	"context"
	"strconv"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.PanelAPI = (*openAPI)(nil)

// ListPanels 查询指令面板列表。
func (o *openAPI) ListPanels(
	ctx context.Context, scope, cursor string, limit int,
) (*dto.ListPanelsResp, error) {
	req := o.request(ctx).
		SetResult(dto.ListPanelsResp{}).
		SetQueryParam("scope", scope)
	if cursor != "" {
		req.SetQueryParam("cursor", cursor)
	}
	if limit > 0 {
		req.SetQueryParam("limit", strconv.Itoa(limit))
	}
	resp, err := req.Get(o.getURL(panelsURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.ListPanelsResp), nil
}

// CreatePanel 创建指令面板。
func (o *openAPI) CreatePanel(ctx context.Context, req *dto.CreatePanelReq) (*dto.CreatePanelResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.CreatePanelResp{}).
		SetBody(req).
		Post(o.getURL(panelsURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.CreatePanelResp), nil
}

// GetPanel 查询指令面板详情。
func (o *openAPI) GetPanel(ctx context.Context, panelID string) (*dto.GetPanelResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GetPanelResp{}).
		SetPathParam("panel_id", panelID).
		Get(o.getURL(panelURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetPanelResp), nil
}

// UpdatePanel 修改指令面板。
func (o *openAPI) UpdatePanel(
	ctx context.Context, panelID string, req *dto.UpdatePanelReq,
) (*dto.UpdatePanelResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UpdatePanelResp{}).
		SetPathParam("panel_id", panelID).
		SetBody(req).
		Put(o.getURL(panelURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UpdatePanelResp), nil
}

// DeletePanel 删除指令面板。
func (o *openAPI) DeletePanel(ctx context.Context, panelID string) error {
	_, err := o.request(ctx).
		SetPathParam("panel_id", panelID).
		Delete(o.getURL(panelURI))
	return err
}

// UpdatePanelTarget 修改指令面板关联对象。
func (o *openAPI) UpdatePanelTarget(
	ctx context.Context, panelID string, req *dto.UpdatePanelTargetReq,
) error {
	_, err := o.request(ctx).
		SetPathParam("panel_id", panelID).
		SetBody(req).
		Put(o.getURL(panelTargetURI))
	return err
}
