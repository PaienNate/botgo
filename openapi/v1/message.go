package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/log"
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
	if msg == nil {
		return nil, errs.ErrNilMessage
	}
	msg = prepareChannelMessage(msg)
	opts := getOptions(ctx, opt...)
	send := func(m dto.APIMessage) (*dto.Message, error) {
		reqCMD := o.request(ctx).
			SetResult(dto.Message{}).
			SetPathParam("channel_id", channelID).
			SetBody(m)
		resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(messagesURI), opt...)
		return decodeMessage(resp, err)
	}
	return o.sendMessageWithFallback(ctx, "channel:"+channelID, msg, opts, send)
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
	if err := validateAPIMessage(msg); err != nil {
		return nil, err
	}
	msg = prepareSendMessage(msg)
	opts := getOptions(ctx, opt...)
	send := func(m dto.APIMessage) (*dto.Message, error) {
		reqCMD := o.request(ctx).
			SetResult(dto.Message{}).
			SetPathParam("group_id", groupID).
			SetBody(m)
		resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(getGroupURIBySendType(m.GetSendType())), opt...)
		return decodeMessage(resp, err)
	}
	return o.sendMessageWithFallback(ctx, "group:"+groupID, msg, opts, send)
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
	if err := validateAPIMessage(msg); err != nil {
		return nil, err
	}
	msg = prepareSendMessage(msg)
	opts := getOptions(ctx, opt...)
	send := func(m dto.APIMessage) (*dto.Message, error) {
		reqCMD := o.request(ctx).
			SetResult(dto.Message{}).
			SetPathParam("user_id", userID).
			SetBody(m)
		resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(getC2CURIBySendType(m.GetSendType())), opt...)
		return decodeMessage(resp, err)
	}
	return o.sendMessageWithFallback(ctx, "c2c:"+userID, msg, opts, send)
}

// PostGroupFile 上传群文件（富媒体）
func (o *openAPI) PostGroupFile(ctx context.Context, groupID string,
	msg *dto.MessageMediaToCreate) (*dto.Media, error) {
	if msg == nil {
		return nil, errs.ErrNilMessage
	}
	reqCMD := o.request(ctx).
		SetResult(dto.Media{}).
		SetPathParam("group_id", groupID).
		SetBody(msg)

	resp, err := reqCMD.Execute(http.MethodPost, o.getURL(groupRichMediaURI))
	media, err := decodeMedia(resp, err)
	if err != nil {
		log.Errorf("[QQBot][openapi] 上传群文件失败 app=%s group=%s code=%d trace=%s err=%s；请检查文件大小/类型、机器人权限或平台状态",
			o.appID, groupID, errorCode(err), errs.Error(err).Trace(), truncateLog(err.Error(), 600))
	}
	return media, err
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
	result, err := decodeMessage(resp, err)
	if err != nil {
		log.Errorf("[QQBot][openapi] 频道图片上传失败 app=%s channel=%s code=%d trace=%s err=%s；请检查图片大小/格式和机器人权限",
			o.appID, channelID, errorCode(err), errs.Error(err).Trace(), truncateLog(err.Error(), 600))
	}
	return result, err
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
	result, err := decodeMessage(resp, err)
	if err != nil {
		log.Errorf("[QQBot][openapi] 频道图片上传失败 app=%s channel=%s code=%d trace=%s err=%s；请检查图片大小/格式和机器人权限",
			o.appID, channelID, errorCode(err), errs.Error(err).Trace(), truncateLog(err.Error(), 600))
	}
	return result, err
}

// PostStreamMessage 流式发送单聊消息。
func (o *openAPI) PostStreamMessage(
	ctx context.Context, userOpenID string, msg *dto.StreamMessageToCreate,
) (*dto.StreamMessageResp, error) {
	if msg == nil {
		return nil, errs.ErrNilMessage
	}
	resp, err := o.request(ctx).
		SetResult(dto.StreamMessageResp{}).
		SetPathParam("user_openid", userOpenID).
		SetBody(msg).
		Post(o.getURL(streamMessagesURI))
	result, err := decodeStreamMessage(resp, err)
	if err != nil {
		log.Errorf("[QQBot][openapi] 流式消息发送失败 app=%s user=%s code=%d trace=%s err=%s；请检查 stream_msg_id/index/state 和消息内容",
			o.appID, userOpenID, errorCode(err), errs.Error(err).Trace(), truncateLog(err.Error(), 600))
	}
	return result, err
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
	// 官方/botpy 约束：file_image 与 message_reference 不能同时传入，优先级为 file_image。
	if msg.MessageReference != nil && len(fileImageData) == 0 {
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
	result, err := decodeMessage(resp, err)
	if err != nil {
		log.Errorf("[QQBot][openapi] 频道 multipart 消息发送失败 app=%s channel=%s code=%d trace=%s err=%s；请检查 file_image/message_reference 和机器人权限",
			o.appID, channelID, errorCode(err), errs.Error(err).Trace(), truncateLog(err.Error(), 600))
	}
	return result, err
}

var msgSeqCounter atomic.Uint32

func init() {
	// 用时间做种子，避免进程重启后从 1 开始，与 5 分钟内旧消息的 msg_seq 冲突。
	msgSeqCounter.Store(uint32(time.Now().UnixNano()%1_000_000_000) + 1)
}

func nextMsgSeq() uint32 {
	for {
		if seq := msgSeqCounter.Add(1); seq != 0 {
			return seq
		}
	}
}

func validateAPIMessage(msg dto.APIMessage) error {
	switch m := msg.(type) {
	case nil:
		return errs.ErrNilMessage
	case *dto.MessageToCreate:
		if m == nil {
			return errs.ErrNilMessage
		}
	case *dto.RichMediaMessage:
		if m == nil {
			return errs.ErrNilMessage
		}
	}
	return nil
}

// prepareChannelMessage 清理频道/私信接口不接受的 v2 字段。
// msg_type 仅用于群聊/单聊 v2 接口，频道消息接口不接受该字段。
func prepareChannelMessage(msg *dto.MessageToCreate) *dto.MessageToCreate {
	if msg == nil || msg.MsgType == 0 {
		return msg
	}
	clone := *msg
	clone.MsgType = dto.TextMsg
	return &clone
}

// prepareSendMessage 为被动回复补充 msg_seq。
// 官方要求同一 msg_id/event_id 的多次回复必须使用不同的 msg_seq，否则返回 40054005。
func prepareSendMessage(msg dto.APIMessage) dto.APIMessage {
	switch m := msg.(type) {
	case *dto.MessageToCreate:
		if m == nil || (m.MsgID == "" && m.EventID == "") || m.MsgSeq != 0 {
			return msg
		}
		clone := *m
		clone.MsgSeq = nextMsgSeq()
		return &clone
	case dto.MessageToCreate:
		if (m.MsgID == "" && m.EventID == "") || m.MsgSeq != 0 {
			return msg
		}
		m.MsgSeq = nextMsgSeq()
		return m
	case *dto.RichMediaMessage:
		if m == nil || m.EventID == "" || m.MsgSeq != 0 {
			return msg
		}
		clone := *m
		clone.MsgSeq = int64(nextMsgSeq())
		return &clone
	case dto.RichMediaMessage:
		if m.EventID == "" || m.MsgSeq != 0 {
			return msg
		}
		m.MsgSeq = int64(nextMsgSeq())
		return m
	default:
		return msg
	}
}

// sendMessageWithFallback 处理官方常见的“可恢复”发送失败：
//   - markdown 权限/格式错误 -> 降级为纯文本
//   - 40054005 消息去重 -> 换一个 msg_seq 重试
//   - 被动回复 msg_id/event_id 过期 -> 可选回退为主动消息
//   - 空响应/5xx -> 可选重试一次
func (o *openAPI) sendMessageWithFallback(
	ctx context.Context,
	target string,
	msg dto.APIMessage,
	opts *options.Options,
	send func(dto.APIMessage) (*dto.Message, error),
) (*dto.Message, error) {
	if opts == nil {
		opts = &options.Options{}
	}
	msgID, eventID := messageIDs(msg)
	actions := make([]string, 0, 4)

	res, err := send(msg)
	if err == nil {
		return res, nil
	}

	if plain, ok := withoutMarkdown(msg); ok && isMarkdownError(err) {
		log.Warnf("[QQBot][openapi] markdown 被拒绝，已自动降级为纯文本 app=%s target=%s msg_id=%s event_id=%s err=%s；若持续出现请检查机器人 markdown 权限或消息内容",
			o.appID, target, msgID, eventID, truncateLog(err.Error(), 600))
		actions = append(actions, "markdown->text")
		res, err = send(plain)
		if err == nil {
			return res, nil
		}
		msg = plain
	}

	if retry, ok := withFreshMsgSeq(msg); ok && isMsgSeqDuplicate(err) {
		log.Warnf("[QQBot][openapi] 消息被去重，已自动更换 msg_seq 重试 app=%s target=%s msg_id=%s event_id=%s err=%s；若频繁出现请检查调用方是否复用 MsgSeq",
			o.appID, target, msgID, eventID, truncateLog(err.Error(), 600))
		actions = append(actions, "new-msg-seq")
		res, err = send(retry)
		if err == nil {
			return res, nil
		}
		msg = retry
	}

	if opts.ProactiveFallback {
		if proactive, ok := withoutPassiveID(msg); ok && isPassiveReplyError(err) {
			log.Warnf("[QQBot][openapi] 被动回复失败，已回退为主动消息 app=%s target=%s msg_id=%s event_id=%s err=%s；这会占用主动消息配额，请确认调用方限流",
				o.appID, target, msgID, eventID, truncateLog(err.Error(), 600))
			actions = append(actions, "proactive")
			res, err = send(proactive)
			if err == nil {
				return res, nil
			}
			msg = proactive
		}
	}

	if opts.RetryOnTransient && isTransientSendError(err) {
		if sleepErr := sleepContext(ctx, 200*time.Millisecond); sleepErr != nil {
			return nil, sleepErr
		}
		log.Warnf("[QQBot][openapi] 发送遇到临时错误或空响应，已重试一次 app=%s target=%s msg_id=%s event_id=%s err=%s；若频繁出现请检查网络或平台状态",
			o.appID, target, msgID, eventID, truncateLog(err.Error(), 600))
		actions = append(actions, "retry")
		res, err = send(msg)
		if err == nil {
			return res, nil
		}
	}

	actionText := "无自动降级"
	if len(actions) > 0 {
		actionText = strings.Join(actions, ",")
	}
	hint := "请检查机器人权限、msg_id/event_id 是否过期、消息内容、主动消息配额或平台状态"
	if isPassiveReplyError(err) && !opts.ProactiveFallback {
		hint += "；如允许主动消息，可启用 options.WithProactiveFallback（注意主动消息配额）"
	}
	if isMarkdownError(err) {
		hint += "；如机器人没有 markdown 权限，请关闭 markdown 或检查消息内容"
	}
	if isTransientSendError(err) && !opts.RetryOnTransient {
		hint += "；如平台响应不稳定，可启用 options.WithRetryOnTransient（注意重复发送风险）"
	}
	log.Errorf("[QQBot][openapi] 消息发送最终失败 app=%s target=%s msg_id=%s event_id=%s code=%d trace=%s actions=%s err=%s；%s",
		o.appID, target, msgID, eventID, errorCode(err), errs.Error(err).Trace(), actionText, truncateLog(err.Error(), 800), hint)
	return nil, err
}

func messageIDs(msg dto.APIMessage) (msgID, eventID string) {
	switch m := msg.(type) {
	case *dto.MessageToCreate:
		if m != nil {
			return m.MsgID, m.EventID
		}
	case dto.MessageToCreate:
		return m.MsgID, m.EventID
	case *dto.RichMediaMessage:
		if m != nil {
			return "", m.EventID
		}
	case dto.RichMediaMessage:
		return "", m.EventID
	}
	return "", ""
}

func withoutMarkdown(msg dto.APIMessage) (dto.APIMessage, bool) {
	switch m := msg.(type) {
	case *dto.MessageToCreate:
		if m == nil || m.Markdown == nil || m.Media != nil {
			return nil, false
		}
		content := m.Markdown.Content
		if strings.TrimSpace(content) == "" {
			content = m.Content
		}
		if strings.TrimSpace(content) == "" {
			return nil, false
		}
		clone := *m
		clone.Markdown = nil
		clone.Content = content
		if clone.MsgType == dto.MarkdownMsg {
			clone.MsgType = dto.TextMsg
		}
		return &clone, true
	case dto.MessageToCreate:
		if m.Markdown == nil || m.Media != nil {
			return nil, false
		}
		content := m.Markdown.Content
		if strings.TrimSpace(content) == "" {
			content = m.Content
		}
		if strings.TrimSpace(content) == "" {
			return nil, false
		}
		m.Markdown = nil
		m.Content = content
		if m.MsgType == dto.MarkdownMsg {
			m.MsgType = dto.TextMsg
		}
		return m, true
	default:
		return nil, false
	}
}

func withoutPassiveID(msg dto.APIMessage) (dto.APIMessage, bool) {
	switch m := msg.(type) {
	case *dto.MessageToCreate:
		if m == nil || (m.MsgID == "" && m.EventID == "") {
			return nil, false
		}
		clone := *m
		clone.MsgID = ""
		clone.EventID = ""
		clone.MsgSeq = 0
		return &clone, true
	case dto.MessageToCreate:
		if m.MsgID == "" && m.EventID == "" {
			return nil, false
		}
		m.MsgID = ""
		m.EventID = ""
		m.MsgSeq = 0
		return m, true
	case *dto.RichMediaMessage:
		if m == nil || m.EventID == "" {
			return nil, false
		}
		clone := *m
		clone.EventID = ""
		clone.MsgSeq = 0
		return &clone, true
	case dto.RichMediaMessage:
		if m.EventID == "" {
			return nil, false
		}
		m.EventID = ""
		m.MsgSeq = 0
		return m, true
	default:
		return nil, false
	}
}

func withFreshMsgSeq(msg dto.APIMessage) (dto.APIMessage, bool) {
	switch m := msg.(type) {
	case *dto.MessageToCreate:
		if m == nil || (m.MsgID == "" && m.EventID == "") {
			return nil, false
		}
		clone := *m
		clone.MsgSeq = nextMsgSeq()
		return &clone, true
	case dto.MessageToCreate:
		if m.MsgID == "" && m.EventID == "" {
			return nil, false
		}
		m.MsgSeq = nextMsgSeq()
		return m, true
	case *dto.RichMediaMessage:
		if m == nil || m.EventID == "" {
			return nil, false
		}
		clone := *m
		clone.MsgSeq = int64(nextMsgSeq())
		return &clone, true
	case dto.RichMediaMessage:
		if m.EventID == "" {
			return nil, false
		}
		m.MsgSeq = int64(nextMsgSeq())
		return m, true
	default:
		return nil, false
	}
}

func isMarkdownError(err error) bool {
	switch errorCode(err) {
	case errs.APICodeMarkdownNotAllowed,
		errs.APICodeMarkdownInvalid,
		errs.APICodeMarkdownParamInvalid,
		errs.APICodeMarkdownNewline,
		errs.APICodeMarkdownSyntaxInParam,
		errs.APICodeMarkdownNoTemplate,
		errs.APICodeMarkdownParamError,
		errs.APICodeLegacyMarkdownNoPerm,
		50037, 50041, 50042, 50054, 50055, 50057:
		return true
	}
	text := errorText(err)
	return strings.Contains(text, "markdown") &&
		(strings.Contains(text, "不允许") ||
			strings.Contains(text, "not allowed") ||
			strings.Contains(text, "无权限") ||
			strings.Contains(text, "invalid") ||
			strings.Contains(text, "参数") ||
			strings.Contains(text, "content"))
}

func isMsgSeqDuplicate(err error) bool {
	if errorCode(err) == errs.APICodeMessageDedup {
		return true
	}
	text := errorText(err)
	return strings.Contains(text, "msgseq") ||
		strings.Contains(text, "msg_seq") ||
		strings.Contains(text, "消息被去重") ||
		strings.Contains(text, "dedup")
}

func isPassiveReplyError(err error) bool {
	switch errorCode(err) {
	case errs.APICodeMessageIDExpired,
		errs.APICodeMessageIDInvalid,
		errs.APICodeEventIDInvalid,
		errs.APICodeEventIDExpired,
		errs.APICodeEventReplyUnsupported,
		errs.APICodePassiveReplyLimit,
		errs.APICodeLegacyMessageIDExpired,
		errs.APICodeLegacyMsgIDInvalid,
		errs.APICodeLegacyMsgExpired,
		errs.APICodeLegacyNotAtBot:
		return true
	}
	text := errorText(err)
	hasID := strings.Contains(text, "msg_id") || strings.Contains(text, "event_id") || strings.Contains(text, "被动回复")
	hasReason := strings.Contains(text, "过期") || strings.Contains(text, "invalid") ||
		strings.Contains(text, "expired") || strings.Contains(text, "越权") ||
		strings.Contains(text, "超限") || strings.Contains(text, "不能回复") ||
		strings.Contains(text, "不支持")
	return hasID && hasReason
}

func isTransientSendError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	switch errorCode(err) {
	case errs.CodeEmptyResponse,
		errs.APICodeMediaTransferFailed,
		errs.APICodeSendServerError,
		errs.APICodeC2CSendServerError,
		errs.APICodeArkSendServerError,
		500, 502, 503, 504:
		return true
	}
	text := errorText(err)
	return strings.Contains(text, "timeout") ||
		strings.Contains(text, "timed out") ||
		strings.Contains(text, "connection reset") ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "eof") ||
		strings.Contains(text, "temporarily unavailable") ||
		strings.Contains(text, "稍后重试")
}

func errorCode(err error) int {
	return errs.Error(err).Code()
}

func errorText(err error) string {
	return strings.ToLower(errs.Error(err).Text())
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func decodeMessage(resp *resty.Response, err error) (*dto.Message, error) {
	if err != nil {
		return nil, err
	}
	if isEmptyBody(resp) {
		return nil, errs.ErrEmptyResponse
	}
	result, ok := resp.Result().(*dto.Message)
	if !ok || result == nil {
		return nil, errs.ErrEmptyResponse
	}
	return result, nil
}

func decodeMedia(resp *resty.Response, err error) (*dto.Media, error) {
	if err != nil {
		return nil, err
	}
	if isEmptyBody(resp) {
		return nil, errs.ErrEmptyResponse
	}
	result, ok := resp.Result().(*dto.Media)
	if !ok || result == nil {
		return nil, errs.ErrEmptyResponse
	}
	return result, nil
}

func decodeStreamMessage(resp *resty.Response, err error) (*dto.StreamMessageResp, error) {
	if err != nil {
		return nil, err
	}
	if isEmptyBody(resp) {
		return nil, errs.ErrEmptyResponse
	}
	result, ok := resp.Result().(*dto.StreamMessageResp)
	if !ok || result == nil {
		return nil, errs.ErrEmptyResponse
	}
	return result, nil
}

func isEmptyBody(resp *resty.Response) bool {
	if resp == nil {
		return true
	}
	body := bytes.TrimSpace(resp.Body())
	return len(body) == 0 || bytes.Equal(body, []byte("null"))
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
