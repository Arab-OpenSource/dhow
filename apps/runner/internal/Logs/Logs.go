package logs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	masking "github.com/Khaymah/dhow/apps/runner/internal/Masking"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

var (
	ErrLeaseLost   = errors.New("the server says the lease is lost")
	ErrUnsupported = errors.New("the server does not store logs")
	ErrRejected    = errors.New("the server rejected the log batch")
	ErrNotDrained  = errors.New("the log pipeline did not drain in time")
)

type Appender interface {
	Append(ctx context.Context, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error)
}

type Options struct {
	Appender      Appender
	Masker        *masking.Masker
	Dir           string
	OpenSpill     func(path string) (SpillStore, error)
	MemoryBytes   int64
	SpillBytes    int64
	ChunkBytes    int
	FlushInterval time.Duration
	BatchBytes    int64
	BatchChunks   int
	Backoff       func(attempt int) time.Duration
	OnControl     func(*runnerv1.LeaseControl)
	Log           *slog.Logger
	Now           func() time.Time
}

type writerKey struct {
	stream runnerv1.LogStream
	step   int32
	has    bool
}

type Pipeline struct {
	o         Options
	q         *queue
	ctx       context.Context
	cancel    context.CancelFunc
	sent      chan struct{}
	flushed   chan struct{}
	stopFlush chan struct{}
	fatal     chan struct{}
	fatalErr  error
	fatalOnce sync.Once
	mu        sync.Mutex
	writers   map[writerKey]*Writer
	order     []*Writer
	truncated atomic.Bool
	disabled  atomic.Bool
	closeOnce sync.Once
}

func (o *Options) defaults() {
	if o.OpenSpill == nil {
		o.OpenSpill = OpenSpillFile
	}
	if o.MemoryBytes == 0 {
		o.MemoryBytes = config.LogMemoryQueueBytes
	}
	if o.SpillBytes == 0 {
		o.SpillBytes = config.LogSpillBytes
	}
	if o.ChunkBytes == 0 {
		o.ChunkBytes = config.LogChunkBytes
	}
	if o.FlushInterval == 0 {
		o.FlushInterval = config.LogFlushInterval
	}
	if o.BatchBytes == 0 {
		o.BatchBytes = config.LogBatchBytes
	}
	if o.BatchChunks == 0 {
		o.BatchChunks = config.LogBatchChunks
	}
	if o.Backoff == nil {
		o.Backoff = func(n int) time.Duration { return min(config.BackoffMax, config.BackoffMin<<min(n, 5)) }
	}
	if o.OnControl == nil {
		o.OnControl = func(*runnerv1.LeaseControl) {}
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Masker == nil {
		o.Masker = masking.New()
	}
}

func New(o Options) *Pipeline {
	o.defaults()
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pipeline{
		o: o, ctx: ctx, cancel: cancel, sent: make(chan struct{}), flushed: make(chan struct{}),
		stopFlush: make(chan struct{}), fatal: make(chan struct{}), writers: map[writerKey]*Writer{},
	}
	p.q = newQueue(o.MemoryBytes, o.SpillBytes, func() (SpillStore, string, error) {
		path := filepath.Join(o.Dir, config.SpillFileName)
		store, err := o.OpenSpill(path)
		return store, path, err
	})
	go p.send()
	go p.flushLoop()
	return p
}

func (p *Pipeline) Writer(stream runnerv1.LogStream, step *int32) *Writer {
	key := writerKey{stream: stream}
	if step != nil {
		key.step, key.has = *step, true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.writers[key]; ok && !w.isClosed() {
		return w
	}
	w := &Writer{p: p, stream: stream, ms: p.o.Masker.Stream()}
	if step != nil {
		s := *step
		w.step = &s
	}
	p.writers[key] = w
	p.order = append(p.order, w)
	return w
}

func (p *Pipeline) Systemf(step *int32, format string, args ...any) {
	_, _ = p.Writer(runnerv1.LogStream_LOG_STREAM_SYSTEM, step).Write([]byte(fmt.Sprintf(format, args...) + "\n"))
}

func (p *Pipeline) Fatal() <-chan struct{} { return p.fatal }

func (p *Pipeline) Err() error {
	select {
	case <-p.fatal:
		return p.fatalErr
	default:
		return nil
	}
}

func (p *Pipeline) Truncated() bool { return p.truncated.Load() }

func (p *Pipeline) raise(err error) {
	p.fatalOnce.Do(func() {
		p.fatalErr = err
		close(p.fatal)
	})
}

func (p *Pipeline) disable() {
	p.disabled.Store(true)
	p.q.stopSending()
}

func (p *Pipeline) flushLoop() {
	defer close(p.flushed)
	t := time.NewTicker(p.o.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-p.stopFlush:
			return
		case <-t.C:
			p.mu.Lock()
			ws := append([]*Writer(nil), p.order...)
			p.mu.Unlock()
			for _, w := range ws {
				w.tick()
			}
		}
	}
}

func (p *Pipeline) writersSnapshot() []*Writer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*Writer(nil), p.order...)
}

func (p *Pipeline) Close(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		var err error
		for _, w := range p.writersSnapshot() {
			if cerr := w.Close(); cerr != nil && !errors.Is(cerr, ErrClosed) {
				err = errors.Join(err, cerr)
			}
		}
		p.q.close()
		<-p.sent
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		err = ErrNotDrained
	}
	return errors.Join(err, p.shutdown())
}

func (p *Pipeline) Abort() error { return p.shutdown() }

func (p *Pipeline) Unblock() { p.q.unblock() }

func (p *Pipeline) shutdown() error {
	var err error
	p.closeOnce.Do(func() {
		p.cancel()
		close(p.stopFlush)
		err = p.q.release()
		<-p.flushed
	})
	return err
}
