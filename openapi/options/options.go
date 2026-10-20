// Package options openapi options
package options

// Options are openapi options
type Options struct {
	URL     string
	HideTip bool // 撤回消息隐藏小灰条可选参数, true: 隐藏小灰条

	// ProactiveFallback 被动回复失败（msg_id/event_id 过期、越权等）时，
	// 去掉被动消息 ID 后按主动消息重试一次。
	//
	// 默认关闭：主动消息有独立频控/日限额，调用方通常需要自行排队。
	ProactiveFallback bool
	// RetryOnTransient 发送消息遇到空响应、5xx 或明确可重试的业务错误时重试一次。
	// 默认关闭，避免在响应丢失但消息已送达时造成重复发送。
	RetryOnTransient bool
}

// Option sets client options.
type Option func(*Options)

// WithURL replace default send URL
func WithURL(url string) Option {
	return func(o *Options) {
		o.URL = url
	}
}

// WithHideTip hide tip
func WithHideTip() Option {
	return func(o *Options) {
		o.HideTip = true
	}
}

// WithProactiveFallback 被动回复失败时回退为主动消息。
func WithProactiveFallback() Option {
	return func(o *Options) {
		o.ProactiveFallback = true
	}
}

// WithRetryOnTransient 空响应、5xx 等可重试错误时重试一次。
func WithRetryOnTransient() Option {
	return func(o *Options) {
		o.RetryOnTransient = true
	}
}
