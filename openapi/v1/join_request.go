package v1

import (
	"context"
	"strconv"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/openapi"
)

var _ openapi.JoinRequestAPI = (*openAPI)(nil)

// ListJoinRequests 拉取入群申请列表。
func (o *openAPI) ListJoinRequests(
	ctx context.Context, groupOpenID, cursor string, limit int,
) (*dto.GetJoinRequestsResp, error) {
	req := o.request(ctx).
		SetResult(dto.GetJoinRequestsResp{}).
		SetPathParam("group_openid", groupOpenID)
	if cursor != "" {
		req.SetQueryParam("cursor", cursor)
	}
	if limit > 0 {
		req.SetQueryParam("limit", strconv.Itoa(limit))
	}
	resp, err := req.Get(o.getURL(groupJoinRequestListURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.GetJoinRequestsResp), nil
}

// ApproveJoinRequest 审批入群申请。
func (o *openAPI) ApproveJoinRequest(
	ctx context.Context, groupOpenID, memberOpenID string, req *dto.ApproveJoinRequestReq,
) error {
	_, err := o.request(ctx).
		SetPathParam("group_openid", groupOpenID).
		SetPathParam("member_openid", memberOpenID).
		SetBody(req).
		Post(o.getURL(groupApprovalJoinRequestURI))
	return err
}
