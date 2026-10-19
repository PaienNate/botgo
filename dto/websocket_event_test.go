package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_transposeIntentEventMap(t *testing.T) {
	t.Run("transpose", func(t *testing.T) {
		re := transposeIntentEventMap(intentEventMap)
		assert.Equal(t, IntentAudio, re[EventAudioFinish])
		assert.Equal(t, IntentAudio, re[EventAudioOffMic])
		assert.Equal(t, IntentGuilds, re[EventChannelCreate])
	})
}
