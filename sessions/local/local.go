// Package local 基于 golang chan 实现的单机 manager。
package local

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/sessions/manager"
	"github.com/sealdice/botgo/websocket"
)

const stableConnectionDuration = 30 * time.Second

// New 创建本地session管理器
func New() *ChanManager {
	return &ChanManager{
		fatalChan: make(chan error, 1),
		backoffs:  make(map[string]*backoff.ExponentialBackOff),
	}
}

// ChanManager 默认的本地 session manager 实现
type ChanManager struct {
	sessionChan chan dto.Session
	fatalChan   chan error
	backoffMu   sync.Mutex
	backoffs    map[string]*backoff.ExponentialBackOff
}

// Start 启动本地 session manager
func (l *ChanManager) Start(ctx context.Context, apInfo *dto.WebsocketAP, tokenSource oauth2.TokenSource, intents *dto.Intent) error {
	defer log.Sync()
	if err := manager.CheckSessionLimit(apInfo); err != nil {
		log.Errorf("[ws/session/local] session limited apInfo: %+v", apInfo)
		return err
	}

	if l.fatalChan == nil {
		l.fatalChan = make(chan error, 1)
	}
	l.backoffMu.Lock()
	if l.backoffs == nil {
		l.backoffs = make(map[string]*backoff.ExponentialBackOff)
	}
	l.backoffMu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	startInterval := manager.CalcInterval(apInfo.SessionStartLimit.MaxConcurrency)
	log.Infof("[ws/session/local] will start %d sessions and per session start interval is %s",
		apInfo.Shards, startInterval)

	var appID string
	if botTokenSource, ok := tokenSource.(interface{ GetAppID() string }); ok {
		appID = botTokenSource.GetAppID()
	}

	// 按照shards数量初始化，用于启动连接的管理
	l.sessionChan = make(chan dto.Session, apInfo.Shards)
	for i := uint32(0); i < apInfo.Shards; i++ {
		session := dto.Session{
			URL:         apInfo.URL,
			TokenSource: tokenSource,
			Intent:      *intents,
			LastSeq:     0,
			Shards: dto.ShardConfig{
				ShardID:    i,
				ShardCount: apInfo.Shards,
			},
			AppID: appID,
		}
		l.sessionChan <- session
	}

	for {
		select {
		case <-runCtx.Done():
			log.Infof("[ws/session/local] context done, stop start session")
			return nil
		case err := <-l.fatalChan:
			log.Errorf("[ws/session/local] terminal error: %v", err)
			return err
		case session := <-l.sessionChan:
			// MaxConcurrency 代表的是每 5s 可以连多少个请求
			select {
			case <-runCtx.Done():
				return nil
			case <-time.After(startInterval):
			}
			go l.newConnect(runCtx, session)
		}
	}
}

// newConnect 启动一个新的连接，如果连接在监听过程中报错了，或者被远端关闭了链接，需要识别关闭的原因，能否继续 resume
// 如果能够 resume，则往 sessionChan 中放入带有 sessionID 的 session
// 如果不能，则清理掉 sessionID，将 session 放入 sessionChan 中
// session 的启动，交给 start 中的 for 循环执行，session 不自己递归进行重连，避免递归深度过深
func (l *ChanManager) newConnect(ctx context.Context, session dto.Session) {
	terminal := false
	defer func() {
		// panic 留下日志，按 transient 错误重排队；terminal 路径不再重排。
		if err := recover(); err != nil {
			websocket.PanicHandler(err, &session)
			if !terminal {
				l.requeue(ctx, session)
			}
		}
	}()

	wsClient := websocket.ClientImpl.New(ctx, session)
	if err := wsClient.Connect(); err != nil {
		log.Error(err)
		l.requeue(ctx, session)
		return
	}

	var err error
	// 如果 session id 不为空，则执行的是 resume 操作，如果为空，则执行的是 identify 操作
	if session.ID != "" {
		err = wsClient.Resume()
	} else {
		// 初次鉴权
		err = wsClient.Identify()
	}
	if err != nil {
		l.logSessionError("[ws/session] Identify/Resume", err)
		terminal = l.handleSessionError(ctx, session, wsClient.Session(), err)
		return
	}

	started := time.Now()
	if err = wsClient.Listening(); err != nil {
		l.logSessionError("[ws/session] Listening", err)
		if time.Since(started) >= stableConnectionDuration {
			l.resetBackoff(session)
			wsClient.Session().AuthRetryCount = 0
		}
		terminal = l.handleSessionError(ctx, session, wsClient.Session(), err)
		return
	}

	// ctx 取消导致的正常退出，不再重排。
	if ctx.Err() != nil {
		return
	}
	l.requeue(ctx, session)
}

// logSessionError 将 QQ 网关要求续期/重连的预期事件降为 Warn，避免用户误以为是故障。
func (l *ChanManager) logSessionError(prefix string, err error) {
	if isExpectedReconnect(err) {
		log.Warnf("%s: QQ 官方要求 WebSocket 重连（正常续期），正在自动恢复: %+v", prefix, err)
		return
	}
	log.Errorf("%s err %+v", prefix, err)
}

func isExpectedReconnect(err error) bool {
	if err == nil {
		return false
	}
	switch errs.Error(err).Code() {
	case errs.CodeNeedReConnect, errs.CodeConnCloseCantResume, errs.CodeInvalidSession,
		errs.WSCodeBackendSessionTimeOut:
		return true
	default:
		return false
	}
}

func (l *ChanManager) handleSessionError(
	ctx context.Context, original dto.Session, current *dto.Session, err error,
) (terminal bool) {
	if current == nil {
		current = &original
	}

	switch manager.Classify(err) {
	case manager.ActionTerminal:
		l.signalFatal(err)
		return true
	case manager.ActionIdentify:
		current.ID = ""
		current.LastSeq = 0
		switch errs.Error(err).Code() {
		case errs.WSCodeBackendDisallowdIntents:
			newIntent := current.Intent &^ (dto.IntentGroupMembers | dto.IntentEnterAIO)
			if newIntent == current.Intent {
				// 已经降级过仍然 4014，说明是其他 intent 无权限，停止重试。
				l.signalFatal(err)
				return true
			}
			log.Infof("[ws/session] disallowed intents (4014) detected, downgrade intents")
			current.Intent = newIntent
		case errs.WSCodeBackendAuthenticationFail:
			// 4004：token 已强制刷新并重试过一次，仍失败说明凭据有问题，停止重试。
			if current.AuthRetryCount >= 1 {
				l.signalFatal(err)
				return true
			}
			current.AuthRetryCount++
		}
	case manager.ActionResume:
		// 保留 session ID 和 seq，直接 resume
	case manager.ActionTransient:
		// 网络/临时错误，保留 session ID 和 seq，退避后 resume
	}

	l.requeue(ctx, *current)
	return false
}

func (l *ChanManager) signalFatal(err error) {
	select {
	case l.fatalChan <- err:
	default:
	}
}

func (l *ChanManager) requeue(ctx context.Context, session dto.Session) {
	delay := l.nextBackoff(session)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	select {
	case l.sessionChan <- session:
	case <-ctx.Done():
	}
}

func (l *ChanManager) nextBackoff(session dto.Session) time.Duration {
	key := backoffKey(session)
	l.backoffMu.Lock()
	b, ok := l.backoffs[key]
	if !ok {
		b = backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(time.Second),
			backoff.WithMultiplier(2),
			backoff.WithMaxInterval(60*time.Second),
			backoff.WithRandomizationFactor(0.5),
			backoff.WithMaxElapsedTime(0),
		)
		l.backoffs[key] = b
	}
	l.backoffMu.Unlock()

	d := b.NextBackOff()
	if d == backoff.Stop {
		return 60 * time.Second
	}
	return d
}

func (l *ChanManager) resetBackoff(session dto.Session) {
	key := backoffKey(session)
	l.backoffMu.Lock()
	if b, ok := l.backoffs[key]; ok {
		b.Reset()
	}
	l.backoffMu.Unlock()
}

func backoffKey(session dto.Session) string {
	return fmt.Sprintf("%s-%d-%d", session.AppID, session.Shards.ShardID, session.Shards.ShardCount)
}
