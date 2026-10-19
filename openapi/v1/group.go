package v1

import (
	"context"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

// 编译期断言：具体实现完整提供可选扩展接口。
var (
	_ openapi.GroupAPI         = (*openAPI)(nil)
	_ openapi.FormFileAPI      = (*openAPI)(nil)
	_ openapi.StreamMessageAPI = (*openAPI)(nil)
)

// GetGroupMembers 获取群成员列表。
func (o *openAPI) GetGroupMembers(ctx context.Context, groupOpenID, cursor string) (*dto.GetGroupMembersResp, error) {
	req := o.request(ctx).
		SetResult(dto.GetGroupMembersResp{}).
		SetPathParam("group_openid", groupOpenID)
	if cursor != "" {
		req.SetQueryParam("cursor", cursor)
	}
	resp, err := req.Get(o.getURL(groupMembersURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetGroupMembersResp), nil
}

// GetGroupMember 获取群成员信息。
func (o *openAPI) GetGroupMember(ctx context.Context, groupOpenID, memberOpenID string) (*dto.GroupMember, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GroupMember{}).
		SetPathParam("group_openid", groupOpenID).
		SetPathParam("member_openid", memberOpenID).
		Get(o.getURL(groupMemberURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GroupMember), nil
}

// GetGroupInfo 获取群基本信息。
func (o *openAPI) GetGroupInfo(ctx context.Context, groupOpenID string) (*dto.GroupInfo, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GroupInfo{}).
		SetPathParam("group_openid", groupOpenID).
		Get(o.getURL(groupInfoURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GroupInfo), nil
}

// GetBotState 获取机器人群内状态。
func (o *openAPI) GetBotState(ctx context.Context, groupOpenID string) (*dto.GroupBotState, error) {
	resp, err := o.request(ctx).
		SetResult(dto.GroupBotState{}).
		SetPathParam("group_openid", groupOpenID).
		Get(o.getURL(groupBotStateURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GroupBotState), nil
}
