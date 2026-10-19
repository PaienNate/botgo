package event

import (
	"sync"

	"github.com/sealdice/botgo/dto"
)

// Handlers 管理所有支持的 handler 类型
type Handlers struct {
	Ready       ReadyHandler
	ErrorNotify ErrorNotifyHandler
	Plain       PlainEventHandler
	PlainTarget PlainEventHandler // 仅兼容

	Guild       GuildEventHandler
	GuildMember GuildMemberEventHandler
	Channel     ChannelEventHandler

	Message             MessageEventHandler
	MessageReaction     MessageReactionEventHandler
	ATMessage           ATMessageEventHandler
	DirectMessage       DirectMessageEventHandler
	MessageAudit        MessageAuditEventHandler
	MessageDelete       MessageDeleteEventHandler
	PublicMessageDelete PublicMessageDeleteEventHandler
	DirectMessageDelete DirectMessageDeleteEventHandler

	Audio AudioEventHandler

	Thread     ThreadEventHandler
	Post       PostEventHandler
	Reply      ReplyEventHandler
	ForumAudit ForumAuditEventHandler

	Interaction InteractionEventHandler

	GroupATMessage     GroupATMessageEventHandler
	GroupMessage       GroupMessageEventHandler
	C2CMessage         C2CMessageEventHandler
	SubscribeMsgStatus SubscribeMsgStatusEventHandler
	C2CFriend          C2CFriendEventHandler

	GroupAddRobot     GroupAddRobotEventHandler
	GroupDelRobot     GroupDelRobotEventHandler
	GroupMemberAdd    GroupMemberAddEventHandler
	GroupMemberRemove GroupMemberRemoveEventHandler
	GroupJoinRequest  GroupJoinRequestEventHandler

	GroupMsgReceive GroupMsgReceiveEventHandler
	GroupMsgReject  GroupMsgRejectEventHandler
	C2CMsgReceive   C2CMsgReceiveEventHandler
	C2CMsgReject    C2CMsgRejectEventHandler

	EnterAIO EnterAIOEventHandler
}

// DefaultHandlers 默认的 handler 实例
var DefaultHandlers Handlers

var (
	// DefaultHandlersMap 保存各个 AppID 的事件回调结构体
	DefaultHandlersMap = make(map[string]*Handlers)
	handlersMu         sync.RWMutex
)

// ReadyHandler 可以处理 ws 的ready 事件
type ReadyHandler func(event *dto.WSPayload, data *dto.WSReadyData)

// ErrorNotifyHandler 当ws 连接发生错误的时候，会回调，方便使用方监控相关错误
// 比如 reconnect invalidSession 等错误，错误可以转换为bot.Err
type ErrorNotifyHandler func(err error)

// PlainEventHandler 透传handler
type PlainEventHandler func(event *dto.WSPayload, message []byte) error

// GuildEventHandler 频道事件handler
type GuildEventHandler func(event *dto.WSPayload, data *dto.WSGuildData) error

// GuildMemberEventHandler 频道成员事件 handler
type GuildMemberEventHandler func(event *dto.WSPayload, data *dto.WSGuildMemberData) error

// ChannelEventHandler 子频道事件handler
type ChannelEventHandler func(event *dto.WSPayload, data *dto.WSChannelData) error

// MessageEventHandler 消息事件 handler
type MessageEventHandler func(event *dto.WSPayload, data *dto.WSMessageData) error

// MessageDeleteEventHandler 消息事件 handler
type MessageDeleteEventHandler func(event *dto.WSPayload, data *dto.WSMessageDeleteData) error

// PublicMessageDeleteEventHandler 消息事件 handler
type PublicMessageDeleteEventHandler func(event *dto.WSPayload, data *dto.WSPublicMessageDeleteData) error

// DirectMessageDeleteEventHandler 消息事件 handler
type DirectMessageDeleteEventHandler func(event *dto.WSPayload, data *dto.WSDirectMessageDeleteData) error

// MessageReactionEventHandler 表情表态事件handler
type MessageReactionEventHandler func(event *dto.WSPayload, data *dto.WSMessageReactionData) error

// ATMessageEventHandler at 机器人消息事件handler
type ATMessageEventHandler func(event *dto.WSPayload, data *dto.WSATMessageData) error

// DirectMessageEventHandler 私信消息事件 handler
type DirectMessageEventHandler func(event *dto.WSPayload, data *dto.WSDirectMessageData) error

// AudioEventHandler 音频机器人事件handler
type AudioEventHandler func(event *dto.WSPayload, data *dto.WSAudioData) error

// MessageAuditEventHandler 消息审核事件 handler
type MessageAuditEventHandler func(event *dto.WSPayload, data *dto.WSMessageAuditData) error

// ThreadEventHandler 论坛主题事件 handler
type ThreadEventHandler func(event *dto.WSPayload, data *dto.WSThreadData) error

// PostEventHandler 论坛回帖事件 handler
type PostEventHandler func(event *dto.WSPayload, data *dto.WSPostData) error

// ReplyEventHandler 论坛帖子回复事件 handler
type ReplyEventHandler func(event *dto.WSPayload, data *dto.WSReplyData) error

// ForumAuditEventHandler 论坛帖子审核事件 handler
type ForumAuditEventHandler func(event *dto.WSPayload, data *dto.WSForumAuditData) error

// InteractionEventHandler 互动事件 handler
type InteractionEventHandler func(event *dto.WSPayload, data *dto.WSInteractionData) error

// ***************** 群消息C2C消息  *****************

// GroupATMessageEventHandler 群中at机器人消息事件handler
type GroupATMessageEventHandler func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error

// GroupMessageEventHandler 群聊普通消息 (非@) 事件 handler
type GroupMessageEventHandler func(event *dto.WSPayload, data *dto.WSGroupMessageData) error

// C2CMessageEventHandler 机器人消息事件handler
type C2CMessageEventHandler func(event *dto.WSPayload, data *dto.WSC2CMessageData) error

// ***************** C2C 添加/删除好友 *******************************

// C2CFriendEventHandler C2C 好友事件 handler
type C2CFriendEventHandler func(event *dto.WSPayload, data *dto.WSC2CFriendData) error

// ************************************************

// SubscribeMsgStatusEventHandler 订阅消息模板授权状态变更事件handler
type SubscribeMsgStatusEventHandler func(event *dto.WSPayload, data *dto.WSSubscribeMsgStatus) error

// EnterAIOEventHandler 进入AIO事件 handler
type EnterAIOEventHandler func(event *dto.WSPayload, data *dto.WSEnterAIOData) error

// GroupAddRobotEventHandler 机器人进群事件 handler
type GroupAddRobotEventHandler func(event *dto.WSPayload, data *dto.WSGroupRobotEventData) error

// GroupDelRobotEventHandler 机器人退群事件 handler
type GroupDelRobotEventHandler func(event *dto.WSPayload, data *dto.WSGroupRobotEventData) error

// GroupMemberAddEventHandler 群成员加入事件 handler
type GroupMemberAddEventHandler func(event *dto.WSPayload, data *dto.WSGroupMemberAddData) error

// GroupMemberRemoveEventHandler 群成员退出事件 handler
type GroupMemberRemoveEventHandler func(event *dto.WSPayload, data *dto.WSGroupMemberRemoveData) error

// GroupJoinRequestEventHandler 用户申请入群事件 handler
type GroupJoinRequestEventHandler func(event *dto.WSPayload, data *dto.WSGroupJoinRequestData) error

// GroupMsgReceiveEventHandler 群聊消息接收开启事件 handler
type GroupMsgReceiveEventHandler func(event *dto.WSPayload, data *dto.WSGroupMsgReceiveData) error

// GroupMsgRejectEventHandler 群聊消息接收关闭事件 handler
type GroupMsgRejectEventHandler func(event *dto.WSPayload, data *dto.WSGroupMsgRejectData) error

// C2CMsgReceiveEventHandler 单聊消息接收开启事件 handler
type C2CMsgReceiveEventHandler func(event *dto.WSPayload, data *dto.WSC2CMsgReceiveData) error

// C2CMsgRejectEventHandler 单聊消息接收关闭事件 handler
type C2CMsgRejectEventHandler func(event *dto.WSPayload, data *dto.WSC2CMsgRejectData) error

// RegisterHandlers 注册事件回调，并返回 intent 用于 websocket 的鉴权
func RegisterHandlers(handlers ...interface{}) dto.Intent {
	return registerHandlers(&DefaultHandlers, handlers...)
}

// RegisterHandlersByAppID 针对特定 AppID 注册事件回调
func RegisterHandlersByAppID(appID string, handlers ...interface{}) dto.Intent {
	handlersMu.Lock()
	hStruct, ok := DefaultHandlersMap[appID]
	if !ok {
		hStruct = &Handlers{}
		DefaultHandlersMap[appID] = hStruct
	}
	handlersMu.Unlock()
	return registerHandlers(hStruct, handlers...)
}

func registerHandlers(hStruct *Handlers, handlers ...interface{}) dto.Intent {
	var i dto.Intent
	for _, h := range handlers {
		switch handle := h.(type) {
		case ReadyHandler:
			hStruct.Ready = handle
		case ErrorNotifyHandler:
			hStruct.ErrorNotify = handle
		case PlainEventHandler:
			hStruct.Plain = handle
		case AudioEventHandler:
			hStruct.Audio = handle
			i |= dto.EventToIntent(
				dto.EventAudioStart, dto.EventAudioFinish,
				dto.EventAudioOnMic, dto.EventAudioOffMic,
			)
		case InteractionEventHandler:
			hStruct.Interaction = handle
			i |= dto.EventToIntent(dto.EventInteractionCreate)
		case SubscribeMsgStatusEventHandler:
			hStruct.SubscribeMsgStatus = handle
			i |= dto.EventToIntent(dto.EventSubscribeMsgStatus)
		case C2CFriendEventHandler:
			hStruct.C2CFriend = handle
			i |= dto.EventToIntent(dto.EventC2CFriendAdd)
		case EnterAIOEventHandler:
			hStruct.EnterAIO = handle
			i |= dto.EventToIntent(dto.EventEnterAIO)
		case GroupAddRobotEventHandler:
			hStruct.GroupAddRobot = handle
			i |= dto.EventToIntent(dto.EventGroupAddRobot)
		case GroupDelRobotEventHandler:
			hStruct.GroupDelRobot = handle
			i |= dto.EventToIntent(dto.EventGroupDelRobot)
		case GroupMemberAddEventHandler:
			hStruct.GroupMemberAdd = handle
			i |= dto.EventToIntent(dto.EventGroupMemberAdd)
		case GroupMemberRemoveEventHandler:
			hStruct.GroupMemberRemove = handle
			i |= dto.EventToIntent(dto.EventGroupMemberRemove)
		case GroupJoinRequestEventHandler:
			hStruct.GroupJoinRequest = handle
			i |= dto.EventToIntent(dto.EventGroupJoinRequest)
		case GroupMsgReceiveEventHandler:
			hStruct.GroupMsgReceive = handle
			i |= dto.EventToIntent(dto.EventGroupMsgReceive)
		case GroupMsgRejectEventHandler:
			hStruct.GroupMsgReject = handle
			i |= dto.EventToIntent(dto.EventGroupMsgReject)
		case C2CMsgReceiveEventHandler:
			hStruct.C2CMsgReceive = handle
			i |= dto.EventToIntent(dto.EventC2CMsgReceive)
		case C2CMsgRejectEventHandler:
			hStruct.C2CMsgReject = handle
			i |= dto.EventToIntent(dto.EventC2CMsgReject)
		default:
		}
	}
	i |= registerRelationHandlers(hStruct, i, handlers...)
	i |= registerMessageHandlers(hStruct, i, handlers...)
	i |= registerForumHandlers(hStruct, i, handlers...)

	return i
}

func registerForumHandlers(hStruct *Handlers, i dto.Intent, handlers ...interface{}) dto.Intent {
	for _, h := range handlers {
		switch handle := h.(type) {
		case ThreadEventHandler:
			hStruct.Thread = handle
			i |= dto.EventToIntent(
				dto.EventForumThreadCreate, dto.EventForumThreadUpdate, dto.EventForumThreadDelete,
			)
		case PostEventHandler:
			hStruct.Post = handle
			i |= dto.EventToIntent(dto.EventForumPostCreate, dto.EventForumPostDelete)
		case ReplyEventHandler:
			hStruct.Reply = handle
			i |= dto.EventToIntent(dto.EventForumReplyCreate, dto.EventForumReplyDelete)
		case ForumAuditEventHandler:
			hStruct.ForumAudit = handle
			i |= dto.EventToIntent(dto.EventForumAuditResult)
		default:
		}
	}
	return i
}

// registerRelationHandlers 注册频道关系链相关handlers
func registerRelationHandlers(hStruct *Handlers, i dto.Intent, handlers ...interface{}) dto.Intent {
	for _, h := range handlers {
		switch handle := h.(type) {
		case GuildEventHandler:
			hStruct.Guild = handle
			i |= dto.EventToIntent(dto.EventGuildCreate, dto.EventGuildDelete, dto.EventGuildUpdate)
		case GuildMemberEventHandler:
			hStruct.GuildMember = handle
			i |= dto.EventToIntent(dto.EventGuildMemberAdd, dto.EventGuildMemberRemove, dto.EventGuildMemberUpdate)
		case ChannelEventHandler:
			hStruct.Channel = handle
			i |= dto.EventToIntent(dto.EventChannelCreate, dto.EventChannelDelete, dto.EventChannelUpdate)
		default:
		}
	}
	return i
}

// registerMessageHandlers 注册消息相关的handler
func registerMessageHandlers(hStruct *Handlers, i dto.Intent, handlers ...interface{}) dto.Intent {
	for _, h := range handlers {
		switch handle := h.(type) {
		case MessageEventHandler:
			hStruct.Message = handle
			i |= dto.EventToIntent(dto.EventMessageCreate)
		case ATMessageEventHandler:
			hStruct.ATMessage = handle
			i |= dto.EventToIntent(dto.EventAtMessageCreate)
		case DirectMessageEventHandler:
			hStruct.DirectMessage = handle
			i |= dto.EventToIntent(dto.EventDirectMessageCreate)
		case MessageDeleteEventHandler:
			hStruct.MessageDelete = handle
			i |= dto.EventToIntent(dto.EventMessageDelete)
		case PublicMessageDeleteEventHandler:
			hStruct.PublicMessageDelete = handle
			i |= dto.EventToIntent(dto.EventPublicMessageDelete)
		case DirectMessageDeleteEventHandler:
			hStruct.DirectMessageDelete = handle
			i |= dto.EventToIntent(dto.EventDirectMessageDelete)
		case MessageReactionEventHandler:
			hStruct.MessageReaction = handle
			i |= dto.EventToIntent(dto.EventMessageReactionAdd, dto.EventMessageReactionRemove)
		case MessageAuditEventHandler:
			hStruct.MessageAudit = handle
			i |= dto.EventToIntent(dto.EventMessageAuditPass, dto.EventMessageAuditReject)
		case GroupATMessageEventHandler:
			hStruct.GroupATMessage = handle
			i |= dto.EventToIntent(dto.EventGroupAtMessageCreate)
		case GroupMessageEventHandler:
			hStruct.GroupMessage = handle
			i |= dto.EventToIntent(dto.EventGroupMessageCreate)
		case C2CMessageEventHandler:
			hStruct.C2CMessage = handle
			i |= dto.EventToIntent(dto.EventC2CMessageCreate)
		default:
		}
	}
	return i
}
