package manager

import (
	"errors"
	"testing"

	"github.com/sealdice/botgo/errs"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Action
	}{
		{"4009 resume", errs.New(errs.WSCodeBackendSessionTimeOut, "timeout"), ActionResume},
		{"4008 resume", errs.New(errs.WSCodeBackendRateLimit, "rate limit"), ActionResume},
		{"9000 reconnect", errs.New(errs.CodeNeedReConnect, "reconnect"), ActionResume},
		{"9005 identify", errs.New(errs.CodeConnCloseCantResume, "cant resume"), ActionIdentify},
		{"4004 identify", errs.New(errs.WSCodeBackendAuthenticationFail, "auth"), ActionIdentify},
		{"4006 identify", errs.New(errs.WSCodeBackendSessionNoLongerValid, "invalid session"), ActionIdentify},
		{"4007 identify", errs.New(errs.WSCodeBackendInvalidSeq, "invalid seq"), ActionIdentify},
		{"4014 identify", errs.New(errs.WSCodeBackendDisallowdIntents, "disallowed"), ActionIdentify},
		{"4900 identify", errs.New(4900, "internal"), ActionIdentify},
		{"4913 identify", errs.New(4913, "internal"), ActionIdentify},
		{"4914 terminal", errs.New(errs.WSCodeBackendBotOffline, "offline"), ActionTerminal},
		{"4915 terminal", errs.New(errs.WSCodeBackendBotBanned, "banned"), ActionTerminal},
		{"4013 terminal", errs.New(errs.WSCodeBackendInvalidIntents, "invalid intents"), ActionTerminal},
		{"4001 terminal", errs.New(errs.WSCodeBackendUnknownOpCode, "opcode"), ActionTerminal},
		{"unknown transient", errors.New("network"), ActionTransient},
		{"nil transient", nil, ActionTransient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Fatalf("Classify(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
