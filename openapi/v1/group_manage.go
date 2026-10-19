package v1

import (
	"context"
	"strconv"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.GroupManageAPI = (*openAPI)(nil)

// BatchRemoveMembers 群成员批量移除。
func (o *openAPI) BatchRemoveMembers(
	ctx context.Context, groupOpenID string, req *dto.BatchRemoveMembersReq,
) (*dto.BatchRemoveMembersResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.BatchRemoveMembersResp{}).
		SetPathParam("group_openid", groupOpenID).
		SetBody(req).
		Post(o.getURL(groupBatchRemoveMembersURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.BatchRemoveMembersResp), nil
}

// GetGroupBlacklist 查询群黑名单。
func (o *openAPI) GetGroupBlacklist(
	ctx context.Context, groupOpenID, cursor string, limit int,
) (*dto.GetGroupBlacklistResp, error) {
	req := o.request(ctx).
		SetResult(dto.GetGroupBlacklistResp{}).
		SetPathParam("group_openid", groupOpenID)
	if cursor != "" {
		req.SetQueryParam("cursor", cursor)
	}
	if limit > 0 {
		req.SetQueryParam("limit", strconv.Itoa(limit))
	}
	resp, err := req.Get(o.getURL(groupMemberBlacklistURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetGroupBlacklistResp), nil
}

// UpdateGroupBlacklist 群黑名单操作。
func (o *openAPI) UpdateGroupBlacklist(
	ctx context.Context, groupOpenID string, req *dto.UpdateGroupBlacklistReq,
) (*dto.UpdateGroupBlacklistResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.UpdateGroupBlacklistResp{}).
		SetPathParam("group_openid", groupOpenID).
		SetBody(req).
		Post(o.getURL(groupMemberBlacklistURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.UpdateGroupBlacklistResp), nil
}

// GetGroupMuteSetting 查询群禁言状态。
func (o *openAPI) GetGroupMuteSetting(ctx context.Context, groupOpenID string) (*dto.GroupMuteSetting, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GroupMuteSetting{}).
		SetPathParam("group_openid", groupOpenID).
		Get(o.getURL(groupRestrictChatSettingURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GroupMuteSetting), nil
}

// SetGroupMute 设置群成员禁言。
func (o *openAPI) SetGroupMute(ctx context.Context, groupOpenID string, req *dto.SetGroupMuteReq) error {
	_, err := o.request(ctx).
		SetPathParam("group_openid", groupOpenID).
		SetBody(req).
		Post(o.getURL(groupRestrictChatSettingURI))
	return err
}
