// Package v1 是 openapi v1 版本的实现。
package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2" // resty 是一个优秀的 rest api 客户端，可以极大的减少开发基于 rest 标准接口求请求 of 封装工作量
	"golang.org/x/oauth2"

	"github.com/sealdice/botgo/constant"
	"github.com/sealdice/botgo/errs"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/openapi"
	"github.com/sealdice/botgo/version"
)

// MaxIdleConns 默认指定空闲连接池大小
const MaxIdleConns = 3000

type openAPI struct {
	appID       string
	tokenSource oauth2.TokenSource
	timeout     time.Duration

	sandbox     bool   // 请求沙箱环境
	debug       bool   // debug 模式，调试sdk时候使用
	lastTraceID string // lastTraceID id

	restyClient *resty.Client // resty client 复用
}

// Setup 注册
func Setup() {
	openapi.Register(openapi.APIv1, &openAPI{})
}

// Version 创建当前版本
func (o *openAPI) Version() openapi.APIVersion {
	return openapi.APIv1
}

// TraceID 获取 lastTraceID id
func (o *openAPI) TraceID() string {
	return o.lastTraceID
}

// Setup 生成一个实例
func (o *openAPI) Setup(botAppID string, tokenSource oauth2.TokenSource, inSandbox bool) openapi.OpenAPI {
	api := &openAPI{
		appID:       botAppID,
		tokenSource: tokenSource,
		timeout:     5 * time.Second,
		sandbox:     inSandbox,
	}
	api.setupClient(botAppID) // 初始化可复用的 client
	return api
}

// WithTimeout 设置请求接口超时时间
func (o *openAPI) WithTimeout(duration time.Duration) openapi.OpenAPI {
	o.restyClient.SetTimeout(duration)
	return o
}

// SetDebug 设置调试模式, 输出更多过程日志
func (o *openAPI) SetDebug(debug bool) openapi.OpenAPI {
	o.restyClient.Debug = debug
	return o
}

// Transport 透传请求
func (o *openAPI) Transport(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	resp, err := o.request(ctx).SetBody(body).Execute(method, url)
	return resp.Body(), err
}

// 初始化 client
func (o *openAPI) setupClient(appID string) {
	o.restyClient = resty.New().
		SetTransport(createTransport(nil, MaxIdleConns)). // 自定义 transport
		SetLogger(log.DefaultLogger).
		SetDebug(o.debug).
		SetTimeout(o.timeout).
		SetRetryCount(1).
		AddRetryCondition(func(resp *resty.Response, _ error) bool {
			// 11244: token 过期/不存在，强制刷新后重试一次。
			return isTokenExpiredResponse(resp)
		}).
		SetHeader("User-Agent", version.String()).
		SetHeader("X-Union-Appid", appID).
		SetPreRequestHook(
			func(_ *resty.Client, request *http.Request) error {
				// 执行请求前过滤器
				// 由于在 OnBeforeRequest 的时候，request 还没生成，所以 filter 不能使用，所以放到 PreRequestHook
				return openapi.DoReqFilterChains(request, nil)
			},
		).
		OnBeforeRequest(
			func(c *resty.Client, r *resty.Request) error {
				tk, err := o.tokenSource.Token()
				if err != nil {
					log.Errorf("[QQBot][openapi] 获取 access_token 失败 app=%s: %v；请检查 AppID/AppSecret、网络和 token 接口状态", o.appID, err)
					return err
				}
				// 只设置到 request 上，不能修改 client：并发请求会互相覆盖，存在 data race。
				r.SetAuthScheme(tk.TokenType)
				r.SetAuthToken(tk.AccessToken)

				if r.Header == nil {
					r.Header = make(http.Header)
				}
				r.Header.Set("X-Union-Appid", o.appID)
				return nil
			},
		).
		// 设置请求之后的钩子，打印日志，判断状态码
		OnAfterResponse(
			func(_ *resty.Client, resp *resty.Response) error {
				log.Infof("%v", respInfo(resp))
				// 执行请求后过滤器
				if err := openapi.DoRespFilterChains(resp.Request.RawRequest, resp.RawResponse); err != nil {
					return err
				}
				traceID := resp.Header().Get(constant.HeaderTraceID)
				o.lastTraceID = traceID
				// 非成功含义的状态码，需要返回 error 供调用方识别
				if !openapi.IsSuccessStatus(resp.StatusCode()) {
					code, message, ok := parseBusinessError(resp.Body())
					if !ok {
						code = resp.StatusCode()
						message = string(resp.Body())
					}
					o.logAPIError(resp, code, message, traceID)
					o.handleError(resp)
					return errs.New(resp.StatusCode(), string(resp.Body()), traceID)
				}
				// 官方部分接口会以 HTTP 200 + err_code/code 返回业务错误。
				// 如果不解析，调用方会把失败当成功，媒体上传、被动回复过期等都会静默丢消息。
				if code, message, ok := parseBusinessError(resp.Body()); ok {
					o.logAPIError(resp, code, message, traceID)
					o.handleError(resp)
					return errs.New(code, message, traceID)
				}
				return nil
			},
		)
}

// request 每个请求，都需要创建一个 request
func (o *openAPI) request(ctx context.Context) *resty.Request {
	return o.restyClient.R().SetContext(ctx)
}

// GetAppID 获取appid
func (o *openAPI) GetAppID() string {
	if o == nil {
		return ""
	}
	return o.appID
}

// logAPIError 输出一条能直接定位问题的 Warn 日志。
// 注意：不要把完整请求体写进错误日志，消息内容可能包含用户隐私。
func (o *openAPI) logAPIError(resp *resty.Response, code int, message, traceID string) {
	method, path := "", ""
	status := 0
	if resp != nil {
		status = resp.StatusCode()
		if resp.Request != nil {
			method = resp.Request.Method
			if resp.Request.RawRequest != nil && resp.Request.RawRequest.URL != nil {
				path = resp.Request.RawRequest.URL.Path
			}
			if path == "" {
				path = resp.Request.URL
			}
		}
	}
	message = truncateLog(message, 300)
	if code == errs.APICodeTokenExpireOrNotExist {
		log.Warnf("[QQBot][openapi] access_token 过期或不存在，正在强制刷新并重试 app=%s method=%s path=%s trace=%s",
			o.appID, method, path, traceID)
		return
	}
	log.Warnf("[QQBot][openapi] 请求返回错误 app=%s method=%s path=%s status=%d code=%d trace=%s message=%s；请检查机器人权限、参数或平台状态",
		o.appID, method, path, status, code, traceID, message)
}

func truncateLog(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return strings.ToValidUTF8(s[:limit], "") + "...(truncated)"
}

// handleError 处理openapi调用失败的情况
func (o *openAPI) handleError(resp *resty.Response) {
	code, _, ok := parseBusinessError(resp.Body())
	if !ok {
		return
	}
	if code == errs.APICodeTokenExpireOrNotExist {
		forceRefreshToken(o.tokenSource)
	}
}

// parseBusinessError 解析 HTTP 200 响应体中的业务错误码。
// 官方错误体通常为 {"err_code":40034005,"message":"...","trace_id":"..."}，
// 部分旧接口使用 {"code":...}。
func parseBusinessError(body []byte) (int, string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return 0, "", false
	}
	var b struct {
		Message string          `json:"message"`
		Code    json.RawMessage `json:"code"`
		ErrCode json.RawMessage `json:"err_code"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return 0, "", false
	}
	code := parseErrorCode(b.ErrCode)
	if code == 0 {
		code = parseErrorCode(b.Code)
	}
	if code == 0 {
		return 0, "", false
	}
	message := b.Message
	if message == "" {
		message = string(body)
	}
	return code, message, true
}

func parseErrorCode(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	text := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	code, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return code
}

func isTokenExpiredResponse(resp *resty.Response) bool {
	if resp == nil {
		return false
	}
	code, _, ok := parseBusinessError(resp.Body())
	return ok && code == errs.APICodeTokenExpireOrNotExist
}

func forceRefreshToken(tokenSource oauth2.TokenSource) {
	if refreshable, ok := tokenSource.(interface {
		ForceRefresh() (*oauth2.Token, error)
	}); ok {
		_, _ = refreshable.ForceRefresh()
		return
	}
	_, _ = tokenSource.Token()
}

// respInfo 用于输出日志的时候格式化数据
func respInfo(resp *resty.Response) string {
	bodyJSON, _ := json.Marshal(resp.Request.Body)
	return fmt.Sprintf(
		"[OPENAPI]%v %v, traceID:%v, status:%v, elapsed:%v req: %v, resp: %v",
		resp.Request.Method,
		resp.Request.URL,
		resp.Header().Get(constant.HeaderTraceID),
		resp.Status(),
		resp.Time(),
		redactLogBody(bodyJSON),
		redactLogBody(resp.Body()),
	)
}

// redactLogBody 对请求/响应 JSON 做最小化脱敏并截断，避免把消息正文、
// 媒体 base64、token、签名等内容写进 Info 日志。
func redactLogBody(raw []byte) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return truncateLog(string(raw), 2048)
	}
	redactLogValue(v)
	redacted, err := json.Marshal(v)
	if err != nil {
		return truncateLog(string(raw), 2048)
	}
	return truncateLog(string(redacted), 2048)
}

func redactLogValue(v interface{}) {
	switch value := v.(type) {
	case map[string]interface{}:
		for key, child := range value {
			switch strings.ToLower(key) {
			case "content", "markdown", "file_data", "file_info",
				"access_token", "token", "secret", "clientsecret",
				"plain_token", "signature":
				value[key] = "[REDACTED]"
			default:
				redactLogValue(child)
			}
		}
	case []interface{}:
		for _, child := range value {
			redactLogValue(child)
		}
	}
}
func createTransport(localAddr net.Addr, idleConns int) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   60 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	if localAddr != nil {
		dialer.LocalAddr = localAddr
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          idleConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   idleConns,
		MaxConnsPerHost:       idleConns,
	}
}
