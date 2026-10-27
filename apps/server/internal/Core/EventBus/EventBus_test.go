package eventbus_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
)

func newBus() (*eventbus.Bus, *bytes.Buffer) {
	var logs bytes.Buffer
	return eventbus.New(slog.New(slog.NewJSONHandler(&logs, nil))), &logs
}

func TestPublishRunsHandlersInRegistrationOrder(t *testing.T) {
	bus, _ := newBus()
	var order []string
	for _, name := range []string{"first", "second", "third"} {
		bus.Subscribe(eventbus.SystemInstanceSetupCompleted, func(_ context.Context, _ eventbus.Event) error {
			order = append(order, name)
			return nil
		})
	}
	require.NoError(t, bus.Publish(context.Background(), eventbus.Event{Name: eventbus.SystemInstanceSetupCompleted}))
	assert.Equal(t, []string{"first", "second", "third"}, order)
}

func TestPublishPassesThePayloadOnlyToMatchingHandlers(t *testing.T) {
	bus, _ := newBus()
	var got any
	var other int
	bus.Subscribe("a", func(_ context.Context, e eventbus.Event) error {
		got = e.Payload
		return nil
	})
	bus.Subscribe("b", func(context.Context, eventbus.Event) error {
		other++
		return nil
	})
	require.NoError(t, bus.Publish(context.Background(), eventbus.Event{Name: "a", Payload: 42}))
	assert.Equal(t, 42, got)
	assert.Zero(t, other)
}

func TestPublishWithoutSubscribersIsANoop(t *testing.T) {
	bus, _ := newBus()
	assert.NoError(t, bus.Publish(context.Background(), eventbus.Event{Name: "nobody"}))
}

func TestHandlerFailuresAreLoggedReturnedAndDoNotStopTheChain(t *testing.T) {
	tests := []struct {
		name    string
		handler eventbus.Handler
		wantErr string
	}{
		{"error", func(context.Context, eventbus.Event) error { return errors.New("handler boom") }, "handler boom"},
		{"panic", func(context.Context, eventbus.Event) error { panic("handler exploded") }, "handler panicked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, logs := newBus()
			var ran bool
			bus.Subscribe("e", tt.handler)
			bus.Subscribe("e", func(context.Context, eventbus.Event) error {
				ran = true
				return nil
			})
			err := bus.Publish(context.Background(), eventbus.Event{Name: "e"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, logs.String(), tt.wantErr)
			assert.True(t, ran, "later handlers must still run")
		})
	}
}

func TestHandlerMaySubscribeWhilePublishing(t *testing.T) {
	bus, _ := newBus()
	var late int
	bus.Subscribe("e", func(context.Context, eventbus.Event) error {
		bus.Subscribe("e", func(context.Context, eventbus.Event) error {
			late++
			return nil
		})
		return nil
	})
	require.NoError(t, bus.Publish(context.Background(), eventbus.Event{Name: "e"}))
	assert.Zero(t, late)
	require.NoError(t, bus.Publish(context.Background(), eventbus.Event{Name: "e"}))
	assert.Equal(t, 1, late)
}
