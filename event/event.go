// Package event 事件处理注册
package event

import (
	"encoding/json"
	"sync"

	"github.com/tidwall/gjson" // 由于回包的d 类型不确定，gjson 用于从回包json中提可d 并进行针瀵规€х殑瑙ｆ瀽

	"github.com/sealdice/botgo/dto"
)

var eventParseFuncMapLock = new(sync.RWMutex)

//nolint:exhaustive // 只处理 dispatch opcode
var eventParseFuncMap = map[dto.OPCode]map[dto.EventType]eventParseFunc{
	dto.WSDispatchEvent: {
		dto.EventGuildCreate: guildHandler,
		dto.EventGuildUpdate: guildHandler,
		dto.EventGuildDelete: guildHandler,

		dto.EventChannelCreate: channelHandler,
		dto.EventChannelUpdate: channelHandler,
		dto.EventChannelDelete: channelHandler,

		dto.EventGuildMemberAdd:    guildMemberHandler,
		dto.EventGuildMemberUpdate: guildMemberHandler,
		dto.EventGuildMemberRemove: guildMemberHandler,

		dto.EventMessageCreate: messageHandler,
		dto.EventMessageDelete: messageDeleteHandler,

		dto.EventMessageReactionAdd:    messageReactionHandler,
		dto.EventMessageReactionRemove: messageReactionHandler,

		dto.EventAtMessageCreate:     atMessageHandler,
		dto.EventPublicMessageDelete: publicMessageDeleteHandler,

		dto.EventDirectMessageCreate: directMessageHandler,
		dto.EventDirectMessageDelete: directMessageDeleteHandler,

		dto.EventAudioStart:  audioHandler,
		dto.EventAudioFinish: audioHandler,
		dto.EventAudioOnMic:  audioHandler,
		dto.EventAudioOffMic: audioHandler,

		dto.EventMessageAuditPass:   messageAuditHandler,
		dto.EventMessageAuditReject: messageAuditHandler,

		dto.EventForumThreadCreate: threadHandler,
		dto.EventForumThreadUpdate: threadHandler,
		dto.EventForumThreadDelete: threadHandler,
		dto.EventForumPostCreate:   postHandler,
		dto.EventForumPostDelete:   postHandler,
		dto.EventForumReplyCreate:  replyHandler,
		dto.EventForumReplyDelete:  replyHandler,
		dto.EventForumAuditResult:  forumAuditHandler,

		dto.EventInteractionCreate:    interactionHandler,
		dto.EventGroupAtMessageCreate: groupAtMessageHandler,
		dto.EventGroupMessageCreate:   groupMessageHandler,
		dto.EventC2CMessageCreate:     c2cMessageHandler,
		dto.EventSubscribeMsgStatus:   subscribeStatusHandler,
		dto.EventC2CFriendAdd:         c2cFriendAddHandler,
		dto.EventC2CFriendDel:         c2cFriendDelHandler,
		dto.EventGroupAddRobot:        groupAddRobotHandler,
		dto.EventGroupDelRobot:        groupDelRobotHandler,
		dto.EventGroupMemberAdd:       groupMemberAddHandler,
		dto.EventGroupMemberRemove:    groupMemberRemoveHandler,
		dto.EventGroupJoinRequest:     groupJoinRequestHandler,
		dto.EventGroupMsgReceive:      groupMsgReceiveHandler,
		dto.EventGroupMsgReject:       groupMsgRejectHandler,
		dto.EventC2CMsgReceive:        c2cMsgReceiveHandler,
		dto.EventC2CMsgReject:         c2cMsgRejectHandler,
		dto.EventEnterAIO:             enterAIOHandler,
	},
}

// RegisterHandler 注册回调事件处理器
func RegisterHandler(opCode dto.OPCode, eventType dto.EventType, handler eventParseFunc) {
	eventParseFuncMapLock.Lock()
	defer eventParseFuncMapLock.Unlock()
	if eventParseFuncMap[opCode] == nil {
		eventParseFuncMap[opCode] = make(map[dto.EventType]eventParseFunc)
	}
	eventParseFuncMap[opCode][eventType] = handler
}

func getHandler(opCode dto.OPCode, eventType dto.EventType) (eventParseFunc, bool) {
	eventParseFuncMapLock.RLock()
	defer eventParseFuncMapLock.RUnlock()
	f, ok := eventParseFuncMap[opCode][eventType]
	return f, ok
}

type eventParseFunc func(event *dto.WSPayload, message []byte) error

// ParseAndHandle 处理回调事件
// Filter 事件过滤器。返回 false 时该事件不会继续投递。
type Filter func(payload *dto.WSPayload) bool

// Observer 事件观察者。用于转发、指标、日志等旁路逻辑。
type Observer func(payload *dto.WSPayload)

type filterEntry struct {
	id uint64
	fn Filter
}

type observerEntry struct {
	id uint64
	fn Observer
}

var (
	filterMu    sync.RWMutex
	filterSeq   uint64
	filters     []filterEntry
	observerMu  sync.RWMutex
	observerSeq uint64
	observers   []observerEntry
)

// RegisterFilter 注册事件过滤器，返回取消函数。
func RegisterFilter(f Filter) func() {
	if f == nil {
		return func() {}
	}
	filterMu.Lock()
	filterSeq++
	id := filterSeq
	filters = append(filters, filterEntry{id: id, fn: f})
	filterMu.Unlock()
	return func() {
		filterMu.Lock()
		defer filterMu.Unlock()
		for i, entry := range filters {
			if entry.id == id {
				filters = append(filters[:i], filters[i+1:]...)
				return
			}
		}
	}
}

// RegisterObserver 注册事件观察者，返回取消函数。
func RegisterObserver(o Observer) func() {
	if o == nil {
		return func() {}
	}
	observerMu.Lock()
	observerSeq++
	id := observerSeq
	observers = append(observers, observerEntry{id: id, fn: o})
	observerMu.Unlock()
	return func() {
		observerMu.Lock()
		defer observerMu.Unlock()
		for i, entry := range observers {
			if entry.id == id {
				observers = append(observers[:i], observers[i+1:]...)
				return
			}
		}
	}
}

func applyFilters(payload *dto.WSPayload) bool {
	filterMu.RLock()
	snapshot := append([]filterEntry(nil), filters...)
	filterMu.RUnlock()
	for _, entry := range snapshot {
		ok := false
		func() {
			defer func() { _ = recover() }()
			ok = entry.fn(payload)
		}()
		if !ok {
			return false
		}
	}
	return true
}

func notifyObservers(payload *dto.WSPayload) {
	observerMu.RLock()
	snapshot := append([]observerEntry(nil), observers...)
	observerMu.RUnlock()
	for _, entry := range snapshot {
		func() {
			defer func() { _ = recover() }()
			entry.fn(payload)
		}()
	}
}

func ParseAndHandle(payload *dto.WSPayload) error {
	if !applyFilters(payload) {
		return nil
	}
	notifyObservers(payload)

	// 指定类型的handler
	if h, ok := getHandler(payload.OPCode, payload.Type); ok {
		return h(payload, payload.RawMessage)
	}
	// 透传handler
	hStruct := getHandlers(payload)
	if hStruct.Plain != nil {
		return hStruct.Plain(payload, payload.RawMessage)
	}
	return nil
}

func getHandlers(payload *dto.WSPayload) *Handlers {
	if payload != nil && payload.Session != nil && payload.Session.AppID != "" {
		handlersMu.RLock()
		h, ok := DefaultHandlersMap[payload.Session.AppID]
		handlersMu.RUnlock()
		if ok {
			return h
		}
	}
	return &DefaultHandlers
}

// ParseData 解析数据
func ParseData(message []byte, target interface{}) error {
	data := gjson.Get(string(message), "d")
	return json.Unmarshal([]byte(data.String()), target)
}

func guildHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGuildData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Guild != nil {
		return h.Guild(payload, data)
	}
	return nil
}

func channelHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSChannelData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Channel != nil {
		return h.Channel(payload, data)
	}
	return nil
}

func guildMemberHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGuildMemberData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GuildMember != nil {
		return h.GuildMember(payload, data)
	}
	return nil
}

func messageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Message != nil {
		return h.Message(payload, data)
	}
	return nil
}

func messageDeleteHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSMessageDeleteData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.MessageDelete != nil {
		return h.MessageDelete(payload, data)
	}
	return nil
}

func messageReactionHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSMessageReactionData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.MessageReaction != nil {
		return h.MessageReaction(payload, data)
	}
	return nil
}

func atMessageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSATMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.ATMessage != nil {
		return h.ATMessage(payload, data)
	}
	return nil
}

func groupAtMessageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupATMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupATMessage != nil {
		return h.GroupATMessage(payload, data)
	}
	return nil
}

func groupMessageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupMessage != nil {
		return h.GroupMessage(payload, data)
	}
	return nil
}

func c2cMessageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSC2CMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.C2CMessage != nil {
		return h.C2CMessage(payload, data)
	}
	return nil
}

func subscribeStatusHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSSubscribeMsgStatus{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.SubscribeMsgStatus != nil {
		return h.SubscribeMsgStatus(payload, data)
	}
	return nil
}

func c2cFriendDelHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSC2CFriendData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.C2CFriend != nil {
		return h.C2CFriend(payload, data)
	}
	return nil
}

func c2cFriendAddHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSC2CFriendData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.C2CFriend != nil {
		return h.C2CFriend(payload, data)
	}
	return nil
}

func publicMessageDeleteHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSPublicMessageDeleteData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.PublicMessageDelete != nil {
		return h.PublicMessageDelete(payload, data)
	}
	return nil
}

func directMessageHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSDirectMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.DirectMessage != nil {
		return h.DirectMessage(payload, data)
	}
	return nil
}

func directMessageDeleteHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSDirectMessageDeleteData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.DirectMessageDelete != nil {
		return h.DirectMessageDelete(payload, data)
	}
	return nil
}

func audioHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSAudioData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Audio != nil {
		return h.Audio(payload, data)
	}
	return nil
}

func threadHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSThreadData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Thread != nil {
		return h.Thread(payload, data)
	}
	return nil
}

func postHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSPostData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Post != nil {
		return h.Post(payload, data)
	}
	return nil
}

func replyHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSReplyData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Reply != nil {
		return h.Reply(payload, data)
	}
	return nil
}

func forumAuditHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSForumAuditData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.ForumAudit != nil {
		return h.ForumAudit(payload, data)
	}
	return nil
}

func messageAuditHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSMessageAuditData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.MessageAudit != nil {
		return h.MessageAudit(payload, data)
	}
	return nil
}

func interactionHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSInteractionData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.Interaction != nil {
		return h.Interaction(payload, data)
	}
	return nil
}

func enterAIOHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSEnterAIOData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.EnterAIO != nil {
		return h.EnterAIO(payload, data)
	}
	return nil
}

func groupAddRobotHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupRobotEventData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupAddRobot != nil {
		return h.GroupAddRobot(payload, data)
	}
	return nil
}

func groupDelRobotHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupRobotEventData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupDelRobot != nil {
		return h.GroupDelRobot(payload, data)
	}
	return nil
}

func groupMemberAddHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupMemberAddData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupMemberAdd != nil {
		return h.GroupMemberAdd(payload, data)
	}
	return nil
}

func groupMemberRemoveHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupMemberRemoveData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupMemberRemove != nil {
		return h.GroupMemberRemove(payload, data)
	}
	return nil
}

func groupJoinRequestHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupJoinRequestData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupJoinRequest != nil {
		return h.GroupJoinRequest(payload, data)
	}
	return nil
}

func groupMsgReceiveHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupMsgReceiveData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupMsgReceive != nil {
		return h.GroupMsgReceive(payload, data)
	}
	return nil
}

func groupMsgRejectHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSGroupMsgRejectData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.GroupMsgReject != nil {
		return h.GroupMsgReject(payload, data)
	}
	return nil
}

func c2cMsgReceiveHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSC2CMsgReceiveData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.C2CMsgReceive != nil {
		return h.C2CMsgReceive(payload, data)
	}
	return nil
}

func c2cMsgRejectHandler(payload *dto.WSPayload, message []byte) error {
	data := &dto.WSC2CMsgRejectData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	h := getHandlers(payload)
	if h.C2CMsgReject != nil {
		return h.C2CMsgReject(payload, data)
	}
	return nil
}
