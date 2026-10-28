package jobs

import (
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func woken(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestTheHubWakesOnlyTheNamedAttemptUnlessItBroadcasts(t *testing.T) {
	h := newHub(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a, b := uuid.New(), uuid.New()
	wakeA, cancelA := h.subscribeLog(a)
	wakeB, cancelB := h.subscribeLog(b)
	queue, cancelQueue := h.subscribe(a)

	h.wakeLog(a, false)
	assert.True(t, woken(wakeA))
	assert.False(t, woken(wakeB), "another attempt is not woken")
	assert.False(t, woken(queue), "the job queue waiters are a separate set")

	h.wakeLog(uuid.New(), false)
	assert.False(t, woken(wakeA))
	assert.False(t, woken(wakeB))

	h.wakeLog(a, false)
	h.wakeLog(a, false)
	assert.True(t, woken(wakeA))
	assert.False(t, woken(wakeA), "wake-ups coalesce and never block the hub")

	h.wakeLog(uuid.Nil, true)
	assert.True(t, woken(wakeA))
	assert.True(t, woken(wakeB))

	cancelA()
	cancelB()
	cancelQueue()
	assert.Empty(t, h.logs)
	assert.Empty(t, h.subs)
}
