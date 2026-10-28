package logs

import (
	"errors"
	"sync"

	masking "github.com/Khaymah/dhow/apps/runner/internal/Masking"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type Writer struct {
	p      *Pipeline
	stream runnerv1.LogStream
	step   *int32
	mu     sync.Mutex
	ms     *masking.Stream
	buf    []byte
	closed bool
}

func (w *Writer) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *Writer) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(b), nil
	}
	w.buf = append(w.buf, w.ms.Write(b)...)
	if err := w.cut(false, false); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *Writer) cut(all, final bool) error {
	limit := w.p.o.ChunkBytes
	for len(w.buf) >= limit || (all && len(w.buf) > 0) {
		n := cutPoint(w.buf, limit, final)
		if n == 0 {
			return nil
		}
		if err := w.p.q.push(w.stream, w.step, w.p.o.Now(), append([]byte(nil), w.buf[:n]...)); err != nil {
			if errors.Is(err, ErrClosed) {
				w.buf = nil
				return nil
			}
			if errors.Is(err, ErrDiskFull) {
				w.p.raise(err)
			}
			return err
		}
		w.buf = append(w.buf[:0], w.buf[n:]...)
	}
	return nil
}

func (w *Writer) tick() {
	if !w.mu.TryLock() {
		return
	}
	defer w.mu.Unlock()
	if w.closed || len(w.buf) == 0 {
		return
	}
	_ = w.cut(true, false)
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	w.buf = append(w.buf, w.ms.Flush()...)
	return w.cut(true, true)
}
