// Package remote 基于 redis list 实现的分布式 session manager。
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/sessions/manager"
	"github.com/sealdice/botgo/sessions/remote/lock"
	"github.com/sealdice/botgo/token"
	"github.com/sealdice/botgo/websocket"
)

const (
	// defaultClusterKey 分布式锁 the 默认key，可以从外部通过 option 来指定
	defaultClusterKey = "defaultCluster"
	// sessionQueueSuffix session 队列的key后缀，实际上key为 `fmt.Sprintf("%s_%s", r.clusterKey, sessionQueueSuffix)`
	sessionQueueSuffix = "sessionsQueue"
	// distributeLockExpireTime 分发shard的实例的分布式锁的默认过期时间
	distributeLockExpireTime = 60 * time.Second
	// shardLockExpireTime 每个不同的shard实例的分布式锁，用于避免同一个 shard 被启动多个实例
	shardLockExpireTime = 30 * time.Second
	// stableConnectionDuration 连接稳定超过该时间后重置退避
	stableConnectionDuration = 30 * time.Second
)

// RedisManager 基于 redis 的 session 管理器，实现分布式 websocket 监听
type RedisManager struct {
	clusterKey         string
	sessionQueueKey    string
	client             *redis.Client
	sessionProduceChan chan dto.Session // 抢到锁的服务，用于持续生产session到redis list的本地chan
	fatalChan          chan error
	stopChan           chan struct{}
	backoffMu          sync.Mutex
	backoffs           map[string]*backoff.ExponentialBackOff
	tokenSource        oauth2.TokenSource
	appID              string
}

// New 创建一个新的基于 redis 的 session 管理器
// 使用 go-redis 调用 redis，超时时间请在 NewClient 时候设置
func New(client *redis.Client, opts ...Option) *RedisManager {
	r := &RedisManager{
		clusterKey: defaultClusterKey,
		client:     client,
		fatalChan:  make(chan error, 1),
		stopChan:   make(chan struct{}),
		backoffs:   make(map[string]*backoff.ExponentialBackOff),
	}
	for _, opt := range opts {
		opt(r)
	}
	// 针对不同的分布式key，设置不同的 queue key
	r.sessionQueueKey = fmt.Sprintf("%s_%s", r.clusterKey, sessionQueueSuffix)
	return r
}

// Start 启动 redis 的 session 管理器
func (r *RedisManager) Start(ctx context.Context, apInfo *dto.WebsocketAP, tokenSource oauth2.TokenSource, intents *dto.Intent) error {
	defer log.Sync()
	if err := manager.CheckSessionLimit(apInfo); err != nil {
		log.Errorf("[ws/session/redis] session limited apInfo: %+v", apInfo)
		return err
	}

	r.tokenSource = tokenSource
	if source, ok := tokenSource.(interface{ GetAppID() string }); ok {
		r.appID = source.GetAppID()
	}

	r.fatalChan = make(chan error, 1)
	r.stopChan = make(chan struct{})
	r.backoffMu.Lock()
	if r.backoffs == nil {
		r.backoffs = make(map[string]*backoff.ExponentialBackOff)
	}
	r.backoffMu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	startInterval := manager.CalcInterval(apInfo.SessionStartLimit.MaxConcurrency)
	log.Infof("[ws/session/redis] will start %d sessions and per session start interval is %s",
		apInfo.Shards, startInterval)

	// session 生产队列
	r.sessionProduceChan = make(chan dto.Session, apInfo.Shards)

	// 进行初始的session分发，抢锁，分发
	// 锁30s，抢到锁的进程，需要每30s续期一次，只要自己还存活，就不能够让另外的进程抢到锁重新进行shards分发
	distributeLock := lock.New(r.clusterKey, uuid.New().String(), r.client)
	if err := distributeLock.Lock(runCtx, distributeLockExpireTime); err == nil {
		log.Infof("[ws/session/redis] got distribute lock! i will do distributeSession, key: %s", r.clusterKey)
		// 抢到锁的进行初次分发
		if err = r.distributeSession(apInfo, tokenSource, intents); err != nil {
			log.Errorf("[ws/session/redis] distribute sessions failed: %v", err)
			return err
		}
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					panicErr := fmt.Errorf("distribute lock renew panic: %v\n%s", rec, debug.Stack())
					log.Errorf("[QQBot][ws/session/redis] %v", panicErr)
					r.signalFatal(panicErr)
				}
			}()
			distributeLock.StartRenew(runCtx, distributeLockExpireTime)
		}()
	} else {
		log.Errorf("got lock failed, err: %v", err)
	}

	// 持续 produce session，遇到网络问题在 chan 中重试
	// 对于抢到了锁的服务，生产第一批session到redis list
	// 对于没有抢到锁的服务，当ws异常，把session放回到 redis list 中，重新分发
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("session producer panic: %v\n%s", rec, debug.Stack())
				log.Errorf("[QQBot][ws/session/redis] %v", panicErr)
				r.signalFatal(panicErr)
			}
		}()
		r.sessionProducer(startInterval, r.stopChan)
	}()

	consumeErr := make(chan error, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicErr := fmt.Errorf("session consumer panic: %v\n%s", rec, debug.Stack())
				log.Errorf("[QQBot][ws/session/redis] %v", panicErr)
				consumeErr <- panicErr
			}
		}()
		consumeErr <- r.consume(runCtx, startInterval)
	}()

	select {
	case <-runCtx.Done():
		close(r.stopChan)
		return nil
	case err := <-r.fatalChan:
		log.Errorf("[ws/session/redis] terminal error: %v", err)
		close(r.stopChan)
		return err
	case err := <-consumeErr:
		close(r.stopChan)
		return err
	}
}

func (r *RedisManager) consume(ctx context.Context, startInterval time.Duration) error {
	log.Debug("[ws/session/redis] start consume for session")
	for {
		// brpop 返回 key value
		data, err := r.client.BRPop(ctx, startInterval*2, r.sessionQueueKey).Result()
		if err != nil {
			if ctx.Err() != nil {
				//nolint:nilerr // ctx 取消是正常关闭
				return nil
			}
			if !errors.Is(err, redis.Nil) {
				log.Errorf("[ws/session/redis] rpop failed, err: %v", err)
			}
			continue
		}
		if len(data) < 2 {
			log.Errorf("[ws/session/redis] data is not valid, data: %+v", data)
			continue
		}
		log.Debugf("[ws/session/redis] consume data: %s", data)

		session := &dto.Session{}
		if err := json.Unmarshal([]byte(data[1]), session); err != nil {
			// 解析出错，不放回去，直接丢弃
			log.Errorf("[ws/session/redis] unmarshal session failed, err: %v", err)
			continue
		}
		// TokenSource 不参与序列化，消费端从本地配置重新注入。
		if session.TokenSource == nil {
			session.TokenSource = r.tokenSource
		}
		if session.AppID == "" {
			session.AppID = r.appID
		}
		if session.TokenSource == nil {
			log.Errorf("[ws/session/redis] session has no token source, drop")
			continue
		}

		go r.newConnect(ctx, *session)
		time.Sleep(startInterval) // 启动一个连接后，等待一下，避免触发服务端的并发控制
	}
}

// getShardLockKey 获取 shard 的锁
func (r *RedisManager) getShardLockKey(session dto.Session) string {
	return fmt.Sprintf("%s_shard_%d_%d",
		r.clusterKey, session.Shards.ShardID, session.Shards.ShardCount)
}

// newConnect 启动一个新的连接，如果连接在监听过程中报错了，或者被远端关闭了链接，需要识别关闭的原因，能否继续 resume
// 如果能够 resume，则往 sessionChan 中放入带有 sessionID 的 session
// 如果不能，则清理掉 sessionID，将 session 放入 sessionChan 中
// session 的启动，交给 start 中的 for 循环执行，session 不自己递归进行重连，避免递归深度过深
func (r *RedisManager) newConnect(ctx context.Context, session dto.Session) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var shardLock *lock.Lock
	terminal := false
	defer func() {
		if rec := recover(); rec != nil {
			websocket.PanicHandler(rec, &session)
			if !terminal && shardLock != nil {
				r.releaseAndRequeue(ctx, shardLock, session)
			}
		}
	}()

	// 锁 shard，避免针对相同 shard 消费重复了
	shardLock = lock.New(r.getShardLockKey(session), uuid.NewString(), r.client)
	if err := shardLock.Lock(ctx, shardLockExpireTime); err != nil {
		// shard 抢锁失败，把 session 放回去，避免上一个 session 的锁释放失败，导致下一个 session 无法启动
		r.requeue(ctx, session)
		return
	}
	go shardLock.StartRenew(ctx, shardLockExpireTime)

	// token初始化失败，重新放回去
	if err := token.StartRefreshAccessToken(ctx, session.TokenSource); err != nil {
		r.releaseAndRequeue(ctx, shardLock, session)
		return
	}

	wsClient := websocket.ClientImpl.New(ctx, session)
	if err := wsClient.Connect(); err != nil {
		log.Error(err)
		r.releaseAndRequeue(ctx, shardLock, session)
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
		log.Errorf("[ws/session/remote] Identify/Resume err %+v", err)
		terminal = r.handleSessionError(ctx, session, wsClient.Session(), err, shardLock)
		return
	}

	started := time.Now()
	if err = wsClient.Listening(); err != nil {
		log.Errorf("[ws/session/remote] Listening err %+v", err)
		if time.Since(started) >= stableConnectionDuration {
			r.resetBackoff(session)
			wsClient.Session().AuthRetryCount = 0
		}
		terminal = r.handleSessionError(ctx, session, wsClient.Session(), err, shardLock)
		return
	}

	if ctx.Err() != nil {
		r.releaseLock(ctx, shardLock)
		return
	}
	r.releaseAndRequeue(ctx, shardLock, session)
}

func (r *RedisManager) handleSessionError(
	ctx context.Context, original dto.Session, current *dto.Session, err error, shardLock *lock.Lock,
) (terminal bool) {
	if current == nil {
		current = &original
	}

	switch manager.Classify(err) {
	case manager.ActionTerminal:
		r.releaseLock(ctx, shardLock)
		r.signalFatal(err)
		return true
	case manager.ActionIdentify:
		current.ID = ""
		current.LastSeq = 0
		switch errs.Error(err).Code() {
		case errs.WSCodeBackendDisallowdIntents:
			newIntent := current.Intent &^ (dto.IntentGroupMembers | dto.IntentEnterAIO)
			if newIntent == current.Intent {
				r.releaseLock(ctx, shardLock)
				r.signalFatal(err)
				return true
			}
			log.Infof("[ws/session/remote] disallowed intents (4014) detected, downgrade intents")
			current.Intent = newIntent
		case errs.WSCodeBackendAuthenticationFail:
			// 4004：token 已强制刷新并重试过一次，仍失败说明凭据有问题，停止重试。
			if current.AuthRetryCount >= 1 {
				r.releaseLock(ctx, shardLock)
				r.signalFatal(err)
				return true
			}
			current.AuthRetryCount++
		}
	case manager.ActionResume:
		// 保留 session ID 和 seq，直接 resume
	case manager.ActionTransient:
		// 网络/临时错误，保留 session ID 和 seq，退避后 resume
	}

	r.releaseAndRequeue(ctx, shardLock, *current)
	return false
}

func (r *RedisManager) releaseAndRequeue(ctx context.Context, shardLock *lock.Lock, session dto.Session) {
	r.releaseLock(ctx, shardLock)
	r.requeue(ctx, session)
}

func (r *RedisManager) releaseLock(ctx context.Context, shardLock *lock.Lock) {
	if shardLock == nil {
		return
	}
	shardLock.StopRenew()
	if err := shardLock.Release(ctx); err != nil {
		log.Errorf("[ws/session/remote] release shardLock failed, err: %s", err)
	}
}

func (r *RedisManager) signalFatal(err error) {
	select {
	case r.fatalChan <- err:
	default:
	}
}

func (r *RedisManager) requeue(ctx context.Context, session dto.Session) {
	delay := r.nextBackoff(session)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	select {
	case r.sessionProduceChan <- session:
	case <-ctx.Done():
	}
}

func (r *RedisManager) nextBackoff(session dto.Session) time.Duration {
	key := backoffKey(session)
	r.backoffMu.Lock()
	b, ok := r.backoffs[key]
	if !ok {
		b = backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(time.Second),
			backoff.WithMultiplier(2),
			backoff.WithMaxInterval(60*time.Second),
			backoff.WithRandomizationFactor(0.5),
			backoff.WithMaxElapsedTime(0),
		)
		r.backoffs[key] = b
	}
	r.backoffMu.Unlock()

	d := b.NextBackOff()
	if d == backoff.Stop {
		return 60 * time.Second
	}
	return d
}

func (r *RedisManager) resetBackoff(session dto.Session) {
	key := backoffKey(session)
	r.backoffMu.Lock()
	if b, ok := r.backoffs[key]; ok {
		b.Reset()
	}
	r.backoffMu.Unlock()
}

func backoffKey(session dto.Session) string {
	return fmt.Sprintf("%s-%d-%d", session.AppID, session.Shards.ShardID, session.Shards.ShardCount)
}
