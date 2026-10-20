// Package errs 是 SDK 里面的错误类型的集合，同时封装了 SDK 专用的错误类型。
package errs

import (
	"errors"
	"fmt"
)

var (
	// ErrNeedReConnect reconnect
	ErrNeedReConnect = New(CodeNeedReConnect, "need reconnect")
	// ErrInvalidSession 无效的 session
	ErrInvalidSession = New(CodeConnCloseCantResume, "invalid session")
	// ErrURLInvalid ws ap url 异常
	ErrURLInvalid = New(CodeConnCloseCantIdentify, "ws ap url is invalid")
	// ErrSessionLimit session 数量受到限制
	ErrSessionLimit = New(CodeConnCloseCantIdentify, "session num limit")

	// ErrNotFoundOpenAPI 未找到对应版本的openapi实现
	ErrNotFoundOpenAPI = New(CodeNotFoundOpenAPI, "not found openapi version")
	// ErrPagerIsNil 分页器为空
	ErrPagerIsNil = New(CodePagerIsNil, "pager is nil")
	// ErrEmptyResponse 接口返回成功状态码但响应体为空，无法确认操作是否生效。
	ErrEmptyResponse = New(CodeEmptyResponse, "empty response body")
	// ErrNilMessage 消息体为空
	ErrNilMessage = New(CodeNilMessage, "message is nil")
)

// sdk 错误码
const (
	CodeNeedReConnect = 9000
	// CodeInvalidSession 无效的的 session id 请重新连接
	CodeInvalidSession  = 9001
	CodeURLInvalid      = 9002
	CodeNotFoundOpenAPI = 9003
	CodeSessionLimit    = 9004
	// CodeConnCloseCantResume 关闭连接错误码，收拢 websocket close error，不允许 resume
	CodeConnCloseCantResume = 9005
	// CodeConnCloseCantIdentify 不允许连接的关闭连接错误，比如机器人被封禁
	CodeConnCloseCantIdentify = 9006
	// CodePagerIsNil 分页器为空
	CodePagerIsNil = 9007
	// CodeEmptyResponse 成功响应体为空
	CodeEmptyResponse = 9008
	// CodeNilMessage 消息体为空
	CodeNilMessage = 9009
)

// websocket错误码
const (
	WSCodeBackendUnknownError         = 4000 // 未知错误
	WSCodeBackendUnknownOpCode        = 4001 // 请求中的opcode非法
	WSCodeBackendDecodeError          = 4002 // 请求发送的数据解析失败，数据格式有问题
	WSCodeBackendNotAuthenticate      = 4003 // 尚未进行身份校验
	WSCodeBackendAuthenticationFail   = 4004 // 身份校验失败
	WSCodeBackendAlreadyAuthenticate  = 4005 // 重复的身份校验
	WSCodeBackendSessionNoLongerValid = 4006 // session已失效
	WSCodeBackendInvalidSeq           = 4007 // 非法的序号
	WSCodeBackendRateLimit            = 4008 // 请求超频
	WSCodeBackendSessionTimeOut       = 4009 // session过期，需要重连
	WSCodeBackendInvalidShard         = 4010 // 非法的shard
	WSCodeBackendShardingRequired     = 4011 // 当前shard承载数据过多，需要重连已重新分配shard
	WSCodeBackendInvalidAPIVersion    = 4012 // 非法的api版本号
	WSCodeBackendInvalidIntents       = 4013 // 非法的intents参数
	WSCodeBackendDisallowdIntents     = 4014 // 使用了未授权使用的intents
	WSCodeBackendBotOffline           = 4914 // 机器人已经被下架
	WSCodeBackendBotBanned            = 4915 // 机器人被封禁
)

// openapi错误码
const (
	APICodeTokenExpireOrNotExist = 11244 // token过期或者不存在

	// 被动回复相关错误：msg_id/event_id 过期、越权或不支持回复。
	APICodeMessageIDExpired       = 40034005 // 回复消息 msg_id 已过期
	APICodeMessageIDInvalid       = 40034024 // msg_id 无效或越权
	APICodeEventIDInvalid         = 40034025 // event_id 无效
	APICodeEventIDExpired         = 40034026 // event_id 已过期
	APICodeEventReplyUnsupported  = 40034027 // 该事件不支持回复消息
	APICodePassiveReplyLimit      = 40034128 // 被动回复时间或次数超限
	APICodeLegacyMessageIDExpired = 304103   // 消息 ID 已过期，不能回复
	APICodeLegacyMsgIDInvalid     = 304026   // 回复的消息 id 错误
	APICodeLegacyMsgExpired       = 304027   // 回复的消息过期
	APICodeLegacyNotAtBot         = 304028   // 非 At 当前用户的消息不允许回复

	// 消息去重：同一 msg_id + msg_seq 重复发送。
	APICodeMessageDedup = 40054005

	// Markdown 权限或内容错误。
	APICodeMarkdownNotAllowed    = 50056
	APICodeMarkdownInvalid       = 40034011
	APICodeMarkdownParamInvalid  = 40034008
	APICodeMarkdownNewline       = 40034009
	APICodeMarkdownSyntaxInParam = 40034010
	APICodeMarkdownNoTemplate    = 40034127
	APICodeMarkdownParamError    = 40034124
	APICodeLegacyMarkdownNoPerm  = 304036

	// 可重试的服务端/媒体转存错误。
	APICodeMediaTransferFailed = 40034004
	APICodeSendServerError     = 50055001
	APICodeC2CSendServerError  = 50055002
	APICodeArkSendServerError  = 50055006
)

// Err sdk err
type Err struct {
	code  int
	text  string
	trace string // 错误追踪ID，可用于向平台反馈问题
}

// New 创建一个新错误
func New(code int, text string, trace ...string) error {
	err := &Err{
		code: code,
		text: text,
	}
	if len(trace) > 0 {
		err.trace = trace[0]
	}
	return err
}

// Error 将错误转换为 sdk 的错误类型
func Error(err error) *Err {
	var e *Err
	if errors.As(err, &e) {
		return e
	}
	return &Err{
		code: 9999,
		text: err.Error(),
	}
}

// Error 输出错误信息
func (e Err) Error() string {
	return fmt.Sprintf("code:%v, text:%v, traceID:%s", e.code, e.text, e.trace)
}

// Code 获取错误码
func (e Err) Code() int {
	return e.code
}

// Text 获取错误信息
func (e Err) Text() string {
	return e.text
}

// Trace 获取错误追踪ID
func (e Err) Trace() string {
	return e.trace
}
