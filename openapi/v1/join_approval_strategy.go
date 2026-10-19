package v1

import (
	"context"
	"strconv"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.JoinApprovalStrategyAPI = (*openAPI)(nil)

// ListJoinApprovalStrategies 查询入群自动审批策略列表。
func (o *openAPI) ListJoinApprovalStrategies(
	ctx context.Context, cursor string, limit int,
) (*dto.GetJoinApprovalStrategiesResp, error) {
	req := o.request(ctx).SetResult(dto.GetJoinApprovalStrategiesResp{})
	if cursor != "" {
		req.SetQueryParam("cursor", cursor)
	}
	if limit > 0 {
		req.SetQueryParam("limit", strconv.Itoa(limit))
	}
	resp, err := req.Get(o.getURL(joinApprovalStrategyURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetJoinApprovalStrategiesResp), nil
}

// CreateJoinApprovalStrategy 创建入群自动审批策略。
func (o *openAPI) CreateJoinApprovalStrategy(
	ctx context.Context, req *dto.CreateJoinApprovalStrategyReq,
) (*dto.CreateJoinApprovalStrategyResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.CreateJoinApprovalStrategyResp{}).
		SetBody(req).
		Post(o.getURL(joinApprovalStrategyURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.CreateJoinApprovalStrategyResp), nil
}

// UpdateJoinApprovalStrategy 修改入群自动审批策略。
func (o *openAPI) UpdateJoinApprovalStrategy(
	ctx context.Context, strategyID string, req *dto.UpdateJoinApprovalStrategyReq,
) (*dto.UpdateJoinApprovalStrategyResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UpdateJoinApprovalStrategyResp{}).
		SetPathParam("strategy_id", strategyID).
		SetBody(req).
		Patch(o.getURL(joinApprovalStrategyItemURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UpdateJoinApprovalStrategyResp), nil
}

// DeleteJoinApprovalStrategy 删除入群自动审批策略。
func (o *openAPI) DeleteJoinApprovalStrategy(ctx context.Context, strategyID string) error {
	_, err := o.request(ctx).
		SetPathParam("strategy_id", strategyID).
		Delete(o.getURL(joinApprovalStrategyItemURI))
	return err
}

// ExecuteJoinApprovalStrategy 执行入群自动审批策略。
func (o *openAPI) ExecuteJoinApprovalStrategy(ctx context.Context, strategyID string) error {
	_, err := o.request(ctx).
		SetPathParam("strategy_id", strategyID).
		Post(o.getURL(joinApprovalStrategyExecuteURI))
	return err
}

// UpdateJoinApprovalStrategyWhitelist 修改入群自动审批策略白名单。
func (o *openAPI) UpdateJoinApprovalStrategyWhitelist(
	ctx context.Context, strategyID string, req *dto.UpdateJoinApprovalWhitelistReq,
) (*dto.UpdateJoinApprovalWhitelistResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UpdateJoinApprovalWhitelistResp{}).
		SetPathParam("strategy_id", strategyID).
		SetBody(req).
		Post(o.getURL(joinApprovalStrategyWhitelistURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UpdateJoinApprovalWhitelistResp), nil
}
