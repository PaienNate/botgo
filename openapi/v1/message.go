package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/openapi/options"
)

// Message 拉取单条消息
func (o *openAPI) Message(ctx context.Context, channelID string, messageID string, opt ...options.Option) (
	*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", messageID)

	resp, err := baseRequest(ctx, reqCMD, http.MethodGet, o.getURL(messagesURI), opt...)
	if err != nil {
		return nil, err
	}

	// 兼容处理
	result := resp.Result().(*dto.Message)
	if result.ID == "" {
		body := gjson.Get(resp.String(), "message")
		if err := json.Unmarshal([]byte(body.String()), result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Messages 拉取消息列表
func (o *openAPI) Messages(ctx context.Context, channelID string, pager *dto.MessagesPager, opt ...options.Option) (
	[]*dto.Message, error) {
	if pager == nil {
		return nil, errs.ErrPagerIsNil
	}
	reqCMD := o.request(ctx).
		SetPathParam("channel_id", channelID).
		SetQueryParams(pager.QueryParams())

	resp, err := baseRequest(ctx, reqCMD, http.MethodGet, o.getURL(messagesURI), opt...)
	if err != nil {
		return nil, err
	}
	messages := make([]*dto.Message, 0)
	if err := json.Unmarshal(resp.Body(), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// PostMessage 发消息
func (o *openAPI) PostMessage(ctx context.Context, channelID string, msg *dto.MessageToCreate,
	opt ...options.Option) (*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(messagesURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

// PatchMessage 编辑消息
func (o *openAPI) PatchMessage(ctx context.Context,
	channelID string, messageID string, msg *dto.MessageToCreate, opt ...options.Option) (*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", messageID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPatch, o.getURL(messageURI), opt...)
	if err != nil {
		return nil, err
	}

	return resp.Result().(*dto.Message), nil
}

// RetractMessage 撤回消息
func (o *openAPI) RetractMessage(ctx context.Context,
	channelID, msgID string, opt ...options.Option) error {
	reqCMD := o.request(ctx).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", msgID)

	_, err := baseRequest(ctx, reqCMD, http.MethodDelete, o.getURL(messageURI), opt...)
	return err
}

// RetractC2CMessage 撤回C2C消息
func (o *openAPI) RetractC2CMessage(ctx context.Context,
	userID, msgID string, opt ...options.Option) error {
	reqCMD := o.request(ctx).
		SetPathParam("user_id", userID).
		SetPathParam("message_id", msgID)
	_, err := baseRequest(ctx, reqCMD, http.MethodDelete, o.getURL(retractC2cMessageURI), opt...)
	return err
}

// RetractGroupMessage 撤回群消息
func (o *openAPI) RetractGroupMessage(ctx context.Context,
	groupID, msgID string, opt ...options.Option) error {
	reqCMD := o.request(ctx).
		SetPathParam("group_id", groupID).
		SetPathParam("message_id", msgID)

	_, err := baseRequest(ctx, reqCMD, http.MethodDelete, o.getURL(retractGroupMessageURI), opt...)
	return err
}

// PostSettingGuide 发送设置引导消息 atUserID为要at的用户
func (o *openAPI) PostSettingGuide(ctx context.Context,
	channelID string, atUserIDs []string, opt ...options.Option) (*dto.Message, error) {
	var content string
	for _, userID := range atUserIDs {
		content += fmt.Sprintf("<@%s>", userID)
	}
	msg := &dto.SettingGuideToCreate{
		Content: content,
	}
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(settingGuideURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

func getGroupURIBySendType(msgType dto.SendType) uri {
	switch msgType {
	case dto.RichMedia:
		return groupRichMediaURI
	default:
		return groupMessagesURI
	}
}

// PostGroupMessage 回复群消息
func (o *openAPI) PostGroupMessage(ctx context.Context, groupID string, msg dto.APIMessage,
	opt ...options.Option) (*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("group_id", groupID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(getGroupURIBySendType(msg.GetSendType())), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

func getC2CURIBySendType(msgType dto.SendType) uri {
	switch msgType {
	case dto.RichMedia:
		return c2cRichMediaURI
	default:
		return c2cMessagesURI
	}
}

// PostC2CMessage 回复C2C消息
func (o *openAPI) PostC2CMessage(ctx context.Context, userID string, msg dto.APIMessage,
	opt ...options.Option) (*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("user_id", userID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(getC2CURIBySendType(msg.GetSendType())), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

// PostGroupFile 上传群文件（富媒体）
func (o *openAPI) PostGroupFile(ctx context.Context, groupID string,
	msg *dto.MessageMediaToCreate) (*dto.Media, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Media{}).
		SetPathParam("group_id", groupID).
		SetBody(msg)

	resp, err := reqCMD.Execute(http.MethodPost, o.getURL(groupRichMediaURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Media), nil
}

// PostFormFileImage 频道图片表单上传。
func (o *openAPI) PostFormFileImage(
	ctx context.Context, channelID string, m map[string]string, path string,
) (*dto.Message, error) {
	resp, err := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetFormData(m).
		SetFile("file_image", path).
		Post(o.getURL(messagesURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

// PostFormFileReaderImage 频道图片表单上传（Reader）。
func (o *openAPI) PostFormFileReaderImage(
	ctx context.Context, channelID string, m map[string]string, filename string, r io.Reader,
) (*dto.Message, error) {
	resp, err := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetFormData(m).
		SetFileReader("file_image", filename, r).
		Post(o.getURL(messagesURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

// PostStreamMessage 流式发送单聊消息。
func (o *openAPI) PostStreamMessage(
	ctx context.Context, userOpenID string, msg *dto.StreamMessageToCreate,
) (*dto.StreamMessageResp, error) {
	resp, err := o.request(ctx).
		SetResult(dto.StreamMessageResp{}).
		SetPathParam("user_openid", userOpenID).
		SetBody(msg).
		Post(o.getURL(streamMessagesURI))
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.StreamMessageResp), nil
}

// PostMessageMultipart 发送频道消息（multipart/form-data，可带 file_image）。
// 官方文档：/channels/{channel_id}/messages 支持 application/json 与 multipart/form-data。
func (o *openAPI) PostMessageMultipart(
	ctx context.Context, channelID string, msg *dto.MessageToCreate, fileImageData []byte,
	opt ...options.Option,
) (*dto.Message, error) {
	if msg == nil {
		msg = &dto.MessageToCreate{}
	}
	form := map[string]string{}
	if msg.Content != "" {
		form["content"] = msg.Content
	}
	if msg.Embed != nil {
		form["embed"] = toJSON(msg.Embed)
	}
	if msg.Ark != nil {
		form["ark"] = toJSON(msg.Ark)
	}
	if msg.Image != "" && len(fileImageData) == 0 {
		form["image"] = msg.Image
	}
	if msg.MsgID != "" {
		form["msg_id"] = msg.MsgID
	}
	if msg.EventID != "" {
		form["event_id"] = msg.EventID
	}
	if msg.MessageReference != nil {
		form["message_reference"] = toJSON(msg.MessageReference)
	}
	if msg.Markdown != nil {
		form["markdown"] = toJSON(msg.Markdown)
	}

	req := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("channel_id", channelID).
		SetFormData(form)
	if len(fileImageData) > 0 {
		req.SetFileReader("file_image", "filename.jpg", bytes.NewReader(fileImageData))
	}
	resp, err := baseRequest(ctx, req, http.MethodPost, o.getURL(messagesURI), opt...)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func baseRequest(ctx context.Context, reqCMD *resty.Request, method, url string, opt ...options.Option) (
	*resty.Response, error) {
	opts := getOptions(ctx, opt...)
	if opts.URL != "" {
		url = opts.URL
	}
	if opts.HideTip {
		reqCMD = reqCMD.SetQueryParam("hidetip", "true")
	}

	return reqCMD.Execute(method, url)
}

func getOptions(_ context.Context, opt ...options.Option) *options.Options {
	opts := &options.Options{}
	for _, o := range opt {
		o(opts)
	}
	return opts
}
