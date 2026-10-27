package eventbus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type Event struct {
	Name    string
	Payload any
}

type Handler func(ctx context.Context, event Event) error

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	log      *slog.Logger
}

func New(log *slog.Logger) *Bus {
	return &Bus{handlers: make(map[string][]Handler), log: log}
}

func (b *Bus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = append(b.handlers[name], h)
}

func (b *Bus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	hs := append([]Handler(nil), b.handlers[event.Name]...)
	b.mu.RUnlock()

	var errs []error
	for _, h := range hs {
		if err := b.run(ctx, h, event); err != nil {
			b.log.ErrorContext(ctx, "event handler failed", "event", event.Name, "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *Bus) run(ctx context.Context, h Handler, event Event) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("handler panicked: %v", rec)
		}
	}()
	return h(ctx, event)
}
