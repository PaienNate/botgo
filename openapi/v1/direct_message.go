package v1

import (
	"context"
	"net/http"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/openapi/options"
)

// CreateDirectMessage 创建私信频道
func (o *openAPI) CreateDirectMessage(ctx context.Context,
	dm *dto.DirectMessageToCreate, opt ...options.Option) (*dto.DirectMessage, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.DirectMessage{}).
		SetBody(dm)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(userMeDMURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.DirectMessage), nil
}

// PostDirectMessage 在私信频道内发消息
func (o *openAPI) PostDirectMessage(ctx context.Context,
	dm *dto.DirectMessage, msg *dto.MessageToCreate, opt ...options.Option) (*dto.Message, error) {
	if msg == nil {
		return nil, errs.ErrNilMessage
	}
	msg = prepareChannelMessage(msg)
	opts := getOptions(ctx, opt...)
	send := func(m dto.APIMessage) (*dto.Message, error) {
		reqCMD := o.request(ctx).
			SetResult(dto.Message{}).
			SetPathParam("guild_id", dm.GuildID).
			SetBody(m)
		resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(dmsURI), opt...)
		return decodeMessage(resp, err)
	}
	return sendMessageWithFallback(ctx, msg, opts, send)
}

// RetractDMMessage 撤回私信消息
func (o *openAPI) RetractDMMessage(ctx context.Context,
	guildID, msgID string, opt ...options.Option) error {
	reqCMD := o.request(ctx).
		SetPathParam("guild_id", guildID).
		SetPathParam("message_id", msgID)

	_, err := baseRequest(ctx, reqCMD, http.MethodDelete, o.getURL(dmsMessageURI), opt...)
	return err
}

// PostDMSettingGuide 鍙戦€佺淇¤缃紩瀵? jumpGuildID为设置引导要跳转的频道ID
func (o *openAPI) PostDMSettingGuide(ctx context.Context,
	dm *dto.DirectMessage, jumpGuildID string, opt ...options.Option) (*dto.Message, error) {
	msg := &dto.SettingGuideToCreate{
		SettingGuide: &dto.SettingGuide{
			GuildID: jumpGuildID,
		},
	}
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("guild_id", dm.GuildID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(dmSettingGuideURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}
