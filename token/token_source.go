// Package token 基于 golang.org/x/oauth2 标准实现token source
package token

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"

	"github.com/sealdice/botgo/constant"
	"github.com/sealdice/botgo/log"
)

const (
	// TypeBearer ..
	TypeBearer string = "Bearer"
	// TypeQQBot ..
	TypeQQBot string = "QQBot"

	// defaultRefreshAhead 提前刷新时间。官方要求在上一个 token 接近过期 60 秒内
	// 获取新 token，因此这里主动提前 60 秒刷新。
	defaultRefreshAhead = 60 * time.Second
	// tokenRequestTimeout token 接口请求超时。
	tokenRequestTimeout = 10 * time.Second
	// tokenResponseLimit 防止异常响应占用过多内存。
	tokenResponseLimit = 1 << 20
)

type qqBotTokenReq struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}

type qqBotTokenRsp struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (r *qqBotTokenRsp) UnmarshalJSON(data []byte) error {
	// 创建一个临时结构体来解析 JSON 数据
	var temp struct {
		Code        int             `json:"code"`
		Message     string          `json:"message"`
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
	}

	// 解析 JSON 数据到临时结构体
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	var expiresIn int64
	if len(temp.ExpiresIn) > 0 && string(temp.ExpiresIn) != "null" {
		// 尝试先解析为字符串
		var s string
		if err := json.Unmarshal(temp.ExpiresIn, &s); err == nil {
			if s != "" {
				var err error
				expiresIn, err = strconv.ParseInt(s, 10, 64)
				if err != nil {
					return err
				}
			}
		} else {
			// 尝试解析为整型
			var n int64
			if err := json.Unmarshal(temp.ExpiresIn, &n); err == nil {
				expiresIn = n
			} else {
				return err
			}
		}
	}

	// 赋值给结构体字段
	r.ExpiresIn = expiresIn
	r.Code = temp.Code
	r.Message = temp.Message
	r.AccessToken = temp.AccessToken
	return nil
}

// QQBotCredentials QQ机器人appid、secret
type QQBotCredentials struct {
	AppID     string `yaml:"appid"`
	AppSecret string `yaml:"secret"`
}

// RefreshableTokenSource 支持主动失效和强制刷新的 token source。
// 这是一个可选接口，不改变 oauth2.TokenSource 的兼容性。
type RefreshableTokenSource interface {
	oauth2.TokenSource
	Invalidate()
	ForceRefresh() (*oauth2.Token, error)
}

// QQBotTokenSource QQ机器人token source
type QQBotTokenSource struct {
	credentials *QQBotCredentials
	cachedToken atomic.Value
	sg          singleflight.Group
}

// NewQQBotTokenSource 初始化
func NewQQBotTokenSource(credentials *QQBotCredentials) oauth2.TokenSource {
	return &QQBotTokenSource{
		credentials: credentials,
	}
}

// Token 获取access token
func (w *QQBotTokenSource) Token() (*oauth2.Token, error) {
	if token, ok := w.loadValidToken(); ok {
		return token, nil
	}

	// 获取新的 access token
	newToken, err, _ := w.sg.Do("retrieve access token", func() (interface{}, error) {
		// singleflight 内二次检查，避免并发请求重复刷新
		if token, ok := w.loadValidToken(); ok {
			return token, nil
		}
		return w.getNewToken()
	})
	if err != nil {
		return nil, err
	}
	token, ok := newToken.(*oauth2.Token)
	if !ok || token == nil {
		return nil, errors.New("invalid token type")
	}
	w.cachedToken.Store(token)
	return token, nil
}

// Invalidate 清空缓存，下一次 Token() 会重新请求。
func (w *QQBotTokenSource) Invalidate() {
	// 使用 typed nil，保证 atomic.Value 的 concrete type 一致。
	w.cachedToken.Store((*oauth2.Token)(nil))
}

// ForceRefresh 强制刷新 token。
func (w *QQBotTokenSource) ForceRefresh() (*oauth2.Token, error) {
	w.Invalidate()
	return w.Token()
}

func (w *QQBotTokenSource) loadValidToken() (*oauth2.Token, bool) {
	raw := w.cachedToken.Load()
	if raw == nil {
		return nil, false
	}
	token, ok := raw.(*oauth2.Token)
	if !ok || token == nil || !token.Valid() {
		return nil, false
	}
	return token, true
}

func (w *QQBotTokenSource) appID() string {
	if w == nil || w.credentials == nil {
		return ""
	}
	return w.credentials.AppID
}

func (w *QQBotTokenSource) getNewToken() (*oauth2.Token, error) {
	retrieveReq := qqBotTokenReq{
		AppID:        w.credentials.AppID,
		ClientSecret: w.credentials.AppSecret,
	}
	data, err := json.Marshal(retrieveReq)
	if err != nil {
		return nil, err
	}
	payload := bytes.NewReader(data)
	log.Debugf("retrieve access token URL:%v", getTokenURL())

	ctx, cancel := context.WithTimeout(context.Background(), tokenRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, getTokenURL(), payload)
	if err != nil {
		log.Errorf("[QQBot][token] 初始化 token 请求失败 app=%s: %v", w.appID(), err)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{
		Timeout: tokenRequestTimeout,
	}
	rsp, err := client.Do(req)
	if err != nil {
		log.Errorf("[QQBot][token] 请求 access_token 失败 app=%s: %v；请检查网络和 token 地址", w.appID(), err)
		return nil, err
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	rspTraceID := rsp.Header.Get(constant.HeaderTraceID)
	body, err := io.ReadAll(io.LimitReader(rsp.Body, tokenResponseLimit))
	if err != nil {
		log.Errorf("[QQBot][token] 读取 token 响应失败 app=%s: %v", w.appID(), err)
		return nil, err
	}
	if rsp.StatusCode != http.StatusOK {
		log.Errorf("[QQBot][token] token 接口返回异常状态 app=%s status=%d trace=%s", w.appID(), rsp.StatusCode, rspTraceID)
		return nil, fmt.Errorf("retrieve token http status %d, traceID %s", rsp.StatusCode, rspTraceID)
	}
	log.Debugf("access token response received, size=%d traceID=%s", len(body), rspTraceID)
	retrieveRsp := &qqBotTokenRsp{}
	if err = json.Unmarshal(body, retrieveRsp); err != nil {
		log.Errorf("[QQBot][token] 解析 token 响应失败 app=%s trace=%s: %v", w.appID(), rspTraceID, err)
		return nil, err
	}
	if retrieveRsp.Code != 0 {
		log.Errorf("[QQBot][token] 获取 access_token 失败 app=%s code=%d trace=%s message=%s；请检查 AppID/AppSecret 和机器人状态",
			w.appID(), retrieveRsp.Code, rspTraceID, retrieveRsp.Message)
		return nil, fmt.Errorf("%v.%v, traceID: %s", retrieveRsp.Code, retrieveRsp.Message, rspTraceID)
	}
	if retrieveRsp.AccessToken == "" {
		log.Errorf("[QQBot][token] token 响应缺少 access_token app=%s trace=%s", w.appID(), rspTraceID)
		return nil, fmt.Errorf("empty access_token, traceID: %s", rspTraceID)
	}
	if retrieveRsp.ExpiresIn <= 0 {
		log.Errorf("[QQBot][token] token 响应 expires_in 无效 app=%s value=%d trace=%s", w.appID(), retrieveRsp.ExpiresIn, rspTraceID)
		return nil, fmt.Errorf("invalid expires_in: %d, traceID: %s", retrieveRsp.ExpiresIn, rspTraceID)
	}
	expiry := time.Now().Add(time.Duration(retrieveRsp.ExpiresIn) * time.Second)
	return &oauth2.Token{
		AccessToken: retrieveRsp.AccessToken,
		TokenType:   TypeQQBot,
		Expiry:      expiry,
		ExpiresIn:   retrieveRsp.ExpiresIn,
	}, nil
}

// GetAppID 获取appid
func (w *QQBotTokenSource) GetAppID() string {
	if w == nil || w.credentials == nil {
		return ""
	}
	return w.credentials.AppID
}

// RefreshErrorHandler token 刷新失败回调。
type RefreshErrorHandler func(err error)

var (
	refreshErrMu        sync.RWMutex
	refreshErrorHandler RefreshErrorHandler
)

// SetRefreshErrorHandler 设置 token 刷新失败回调。
// 建议在 StartRefreshAccessToken 之前调用。
func SetRefreshErrorHandler(h RefreshErrorHandler) {
	refreshErrMu.Lock()
	refreshErrorHandler = h
	refreshErrMu.Unlock()
}

func currentRefreshErrorHandler() RefreshErrorHandler {
	refreshErrMu.RLock()
	defer refreshErrMu.RUnlock()
	return refreshErrorHandler
}

// StartRefreshAccessToken 启动获取AccessToken的后台刷新
func StartRefreshAccessToken(ctx context.Context, tokenSource oauth2.TokenSource) error {
	tk, err := tokenSource.Token()
	if err != nil {
		return err
	}
	appID := ""
	if source, ok := tokenSource.(interface{ GetAppID() string }); ok {
		appID = source.GetAppID()
	}
	go func() {
		for {
			ttlSeconds := tk.ExpiresIn
			if ttlSeconds <= 0 && !tk.Expiry.IsZero() {
				ttlSeconds = int64(time.Until(tk.Expiry).Seconds())
			}
			refreshMilliSec := getRefreshMilliSec(ttlSeconds)
			log.Debugf("refresh after %d milli sec", refreshMilliSec)
			timer := time.NewTimer(time.Duration(refreshMilliSec) * time.Millisecond)
			select {
			case <-timer.C:
				timer.Stop()
			case <-ctx.Done():
				timer.Stop()
				log.Warnf("[QQBot][token] 刷新协程退出 app=%s: %v", appID, ctx.Err())
				return
			}

			b := backoff.NewExponentialBackOff(
				backoff.WithInitialInterval(time.Second),
				backoff.WithMultiplier(2),
				backoff.WithMaxInterval(time.Minute),
				backoff.WithRandomizationFactor(0.5),
				backoff.WithMaxElapsedTime(0),
			)
			err := backoff.RetryNotify(func() error {
				var refreshErr error
				tk, refreshErr = forceRefreshToken(tokenSource)
				return refreshErr
			}, backoff.WithContext(b, ctx), func(err error, d time.Duration) {
				if h := currentRefreshErrorHandler(); h != nil {
					h(err)
				}
				log.Errorf("[QQBot][token] 刷新 access_token 失败 app=%s: %v；%v 后重试", appID, err, d)
			})
			if err != nil {
				// WithContext 在 ctx 取消时返回；其他情况 MaxElapsedTime 为 0，不会发生。
				log.Warnf("[QQBot][token] 停止刷新 access_token app=%s: %v", appID, err)
				return
			}
			if tk == nil {
				log.Errorf("[QQBot][token] access_token 刷新后为空 app=%s", appID)
				continue
			}
			log.Infof("[QQBot][token] access_token 已刷新 app=%s expires_in=%d", appID, tk.ExpiresIn)
		}
	}()
	return nil
}

// getRefreshMilliSec 为token刷新保留提前量，避免因为网络延迟等原因导致token刷新不及时。
func getRefreshMilliSec(tokenTTLSec int64) int64 {
	if tokenTTLSec <= 0 {
		// 异常 TTL，避免紧凑循环
		return int64(time.Second / time.Millisecond)
	}
	ttl := time.Duration(tokenTTLSec) * time.Second
	if ttl <= defaultRefreshAhead {
		return 0
	}
	d := ttl - defaultRefreshAhead
	// 增加最多 5 秒的 jitter，避免所有应用同时刷新。
	jitter := time.Duration(rand.Int63n(int64(5 * time.Second)))
	if d > jitter {
		d -= jitter
	} else {
		d = 0
	}
	return int64(d / time.Millisecond)
}

func forceRefreshToken(tokenSource oauth2.TokenSource) (*oauth2.Token, error) {
	if refreshable, ok := tokenSource.(interface {
		ForceRefresh() (*oauth2.Token, error)
	}); ok {
		return refreshable.ForceRefresh()
	}
	return tokenSource.Token()
}

var tokenURLOverride string

func getTokenURL() string {
	if tokenURLOverride != "" {
		return tokenURLOverride
	}
	return fmt.Sprintf("%v%v", constant.TokenDomain, "/app/getAppAccessToken")
}
