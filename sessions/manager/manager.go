// Package manager 实现 session manager 所需要的公共方法。
package manager

import (
	"math"
	"time"

	"github.com/sealdice/botgo/dto"
	"github.com/sealdice/botgo/errs"
)

// CanNotResumeErrSet 不能进行 resume 操作的错误码
var CanNotResumeErrSet = map[int]bool{
	errs.CodeConnCloseCantResume: true,
}

// CanNotIdentifyErrSet 不能进行 identify 操作的错误码
var CanNotIdentifyErrSet = map[int]bool{
	errs.CodeConnCloseCantIdentify: true,
}

// concurrencyTimeWindowSec 并发时间窗口，单位秒
const concurrencyTimeWindowSec = 2

// CalcInterval 根据并发要求，计算连接启动间隔
func CalcInterval(maxConcurrency uint32) time.Duration {
	if maxConcurrency == 0 {
		maxConcurrency = 1
	}
	f := math.Round(concurrencyTimeWindowSec / float64(maxConcurrency))
	if f == 0 {
		f = 1
	}
	return time.Duration(f) * time.Second
}

// Action 是连接错误对应的处理动作。
type Action int

const (
	// ActionResume 保留 session，直接 resume。
	ActionResume Action = iota
	// ActionIdentify 清空 session，重新 identify。
	ActionIdentify
	// ActionTerminal 不可恢复，停止该 app 的 manager。
	ActionTerminal
	// ActionTransient 网络/临时错误，退避重连。
	ActionTransient
)

// Classify 根据错误码决定下一步动作。
func Classify(err error) Action {
	if err == nil {
		return ActionTransient
	}
	code := errs.Error(err).Code()
	switch code {
	case errs.WSCodeBackendSessionTimeOut, // 4009
		errs.WSCodeBackendRateLimit, // 4008，可 resume
		errs.CodeNeedReConnect:      // 服务端要求重连
		return ActionResume

	case errs.CodeConnCloseCantResume, // 9005
		errs.CodeInvalidSession,                // 9001
		errs.WSCodeBackendAuthenticationFail,   // 4004
		errs.WSCodeBackendSessionNoLongerValid, // 4006
		errs.WSCodeBackendInvalidSeq:           // 4007
		return ActionIdentify

	case errs.WSCodeBackendDisallowdIntents: // 4014，先降级再 identify
		return ActionIdentify

	case errs.CodeConnCloseCantIdentify, // 9006
		errs.WSCodeBackendBotOffline,        // 4914
		errs.WSCodeBackendBotBanned,         // 4915
		errs.WSCodeBackendInvalidIntents,    // 4013
		errs.WSCodeBackendUnknownOpCode,     // 4001
		errs.WSCodeBackendDecodeError,       // 4002
		errs.WSCodeBackendInvalidShard,      // 4010
		errs.WSCodeBackendShardingRequired,  // 4011
		errs.WSCodeBackendInvalidAPIVersion: // 4012
		return ActionTerminal
	}

	// 4900-4913 内部错误：重新 identify
	if code >= 4900 && code <= 4913 {
		return ActionIdentify
	}
	return ActionTransient
}

// CanNotResume 是否是不能够 resume 的错误
func CanNotResume(err error) bool {
	e := errs.Error(err)
	if flag, ok := CanNotResumeErrSet[e.Code()]; ok {
		return flag
	}
	return false
}

// CanNotIdentify 是否是不能够 identify 的错误
func CanNotIdentify(err error) bool {
	e := errs.Error(err)
	if flag, ok := CanNotIdentifyErrSet[e.Code()]; ok {
		return flag
	}
	return false
}

// CheckSessionLimit 检查链接数是否达到限制，如果达到限制需要等待重置
func CheckSessionLimit(apInfo *dto.WebsocketAP) error {
	if apInfo.Shards > apInfo.SessionStartLimit.Remaining {
		return errs.ErrSessionLimit
	}
	return nil
}
