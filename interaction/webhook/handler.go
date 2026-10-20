package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/event"
	"github.com/sealdice/botgo/interaction/signature"
	"github.com/sealdice/botgo/internal/dedup"
	"github.com/sealdice/botgo/log"
	"github.com/sealdice/botgo/token"
)

// AckMode 控制 webhook 的 ACK 时机。
type AckMode int

const (
	// AckSync 处理完成后再 ACK，保持旧行为。handler 返回错误时 d=1，平台会重试。
	AckSync AckMode = iota
	// AckAsync 先 ACK，再异步处理。适合长耗时 handler；失败只能本地记录，平台不会重试。
	AckAsync
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// Deduper webhook 事件去重接口。
type Deduper interface {
	Seen(eventID string) bool
	Mark(eventID string)
	Unmark(eventID string)
}

// HandlerOptions webhook handler 配置。
type HandlerOptions struct {
	Credentials  *token.QQBotCredentials
	MaxBodyBytes int64
	Deduper      Deduper
	AckMode      AckMode
	// SkipVerify 仅用于本地测试。生产环境不要开启。
	SkipVerify   bool
	ErrorHandler func(error)
}

// Handler 可配置的 webhook handler。
type Handler struct {
	opts HandlerOptions
}

// NewHandler 创建 Handler。
func NewHandler(opts HandlerOptions) *Handler {
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBodyBytes
	}
	return &Handler{opts: opts}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.opts.MaxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "read body failed")
		return
	}

	payload := &dto.WSPayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	// 官方 URL 校验请求（op 13）不带签名头，必须先处理，再验签。
	if payload.OPCode == dto.HTTPCallbackValidation {
		h.handleValidation(w, r, payload)
		return
	}

	if !h.opts.SkipVerify {
		secret := ""
		if h.opts.Credentials != nil {
			secret = h.opts.Credentials.AppSecret
		}
		if secret == "" {
			log.Errorf("[QQBot][webhook] 缺少 AppSecret，无法验证回调 app=%s", h.appID())
			writeError(w, http.StatusInternalServerError, "missing app secret")
			return
		}
		pass, err := signature.Verify(secret, r.Header, body)
		if err != nil || !pass {
			log.Errorf("[QQBot][webhook] 签名校验失败 app=%s: %v；请检查 AppSecret 与回调地址配置", h.appID(), err)
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
	}

	payload.RawMessage = body
	if h.opts.Credentials != nil {
		payload.Session = &dto.Session{AppID: h.opts.Credentials.AppID}
	}

	switch payload.OPCode {
	case dto.WSDispatchEvent:
		h.handleDispatch(w, payload)
	case dto.WSHeartbeat:
		seq := uint32(0)
		if v, ok := payload.Data.(float64); ok {
			seq = uint32(v)
		}
		writeJSON(w, http.StatusOK, GenHeartbeatACK(seq))
	default:
		// 未知 op，按成功 ACK 处理，避免平台无限重试。
		writeJSON(w, http.StatusOK, GenDispatchACK(true))
	}
}

func (h *Handler) handleValidation(w http.ResponseWriter, r *http.Request, payload *dto.WSPayload) {
	secret := ""
	if h.opts.Credentials != nil {
		secret = h.opts.Credentials.AppSecret
	}
	if secret == "" {
		log.Errorf("[QQBot][webhook] 缺少 AppSecret，无法处理回调校验 app=%s", h.appID())
		writeError(w, http.StatusInternalServerError, "missing app secret")
		return
	}

	plainToken, eventTs, ok := extractValidationData(payload.Data)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid validation payload")
		return
	}
	rsp := GenValidationACK(&dto.WHValidationReq{
		PlainToken: plainToken,
		EventTs:    eventTs,
	}, r.Header, secret)
	if rsp == nil {
		writeError(w, http.StatusInternalServerError, "generate validation ack failed")
		return
	}
	writeJSON(w, http.StatusOK, string(rsp))
}

func (h *Handler) handleDispatch(w http.ResponseWriter, payload *dto.WSPayload) {
	dedupKeys := dedup.Keys(payload)
	if dedup.SeenAny(h.opts.Deduper, dedupKeys) {
		writeJSON(w, http.StatusOK, GenDispatchACK(true))
		return
	}

	// 在开始处理前就标记，避免平台超时重试与第一次处理并发执行，
	// 导致同一条消息被执行两次。处理失败时再撤销标记，让平台可以重试。
	dedup.MarkAll(h.opts.Deduper, dedupKeys)

	if h.opts.AckMode == AckAsync {
		writeJSON(w, http.StatusOK, GenDispatchACK(true))
		go func() {
			if err := dispatch(payload); err != nil {
				h.reportError(err, payload)
				dedup.UnmarkAll(h.opts.Deduper, dedupKeys)
			}
		}()
		return
	}

	if err := dispatch(payload); err != nil {
		h.reportError(err, payload)
		dedup.UnmarkAll(h.opts.Deduper, dedupKeys)
		writeJSON(w, http.StatusOK, GenDispatchACK(false))
		return
	}
	writeJSON(w, http.StatusOK, GenDispatchACK(true))
}

func (h *Handler) appID() string {
	if h.opts.Credentials != nil {
		return h.opts.Credentials.AppID
	}
	return ""
}

func (h *Handler) reportError(err error, payload *dto.WSPayload) {
	if h.opts.ErrorHandler != nil {
		h.opts.ErrorHandler(err)
		return
	}
	eventID, eventType := "", dto.EventType("")
	if payload != nil {
		eventID = payload.EventID
		eventType = payload.Type
	}
	log.Errorf("[QQBot][webhook] 事件处理失败 app=%s event_id=%s type=%s err=%v；若持续出现请检查业务 handler 返回值和日志",
		h.appID(), eventID, eventType, err)
}

func dispatch(payload *dto.WSPayload) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()
	return event.ParseAndHandle(payload)
}

func extractValidationData(data interface{}) (plainToken, eventTs string, ok bool) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return "", "", false
	}
	plainToken, _ = m["plain_token"].(string)
	eventTs, _ = m["event_ts"].(string)
	return plainToken, eventTs, plainToken != "" && eventTs != ""
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, fmt.Sprintf(`{"error":%q}`, message))
}
