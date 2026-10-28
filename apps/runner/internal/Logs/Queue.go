package logs

import (
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const maxRecordBody = config.LogChunkBytes + 1024

var ErrClosed = errors.New("the log pipeline is closed")

type queue struct {
	mu        sync.Mutex
	cond      *sync.Cond
	nextSeq   int64
	mem       []*runnerv1.LogChunk
	memBytes  int64
	memMax    int64
	spillMax  int64
	openSpill func() (SpillStore, string, error)
	spill     *spill
	closed    bool
	dropping  bool
	unblocked bool
	dropped   int64
	err       error
}

func newQueue(memMax, spillMax int64, openSpill func() (SpillStore, string, error)) *queue {
	q := &queue{nextSeq: 1, memMax: memMax, spillMax: spillMax, openSpill: openSpill}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) spilled() int64 {
	if q.spill == nil {
		return 0
	}
	return q.spill.write - q.spill.read
}

func (q *queue) fits(size int64) bool {
	if q.spilled() == 0 && q.memBytes+size <= q.memMax {
		return true
	}
	return q.spilled()+size+recordHeader+64 <= q.spillMax
}

func (q *queue) push(stream runnerv1.LogStream, step *int32, at time.Time, data []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	size := int64(len(data))
	for !q.closed && !q.dropping && !q.unblocked && q.err == nil && !q.fits(size) && (q.memBytes > 0 || q.spilled() > 0) {
		q.cond.Wait()
	}
	if q.unblocked && !q.fits(size) && (q.memBytes > 0 || q.spilled() > 0) {
		q.dropped += size
		return nil
	}
	switch {
	case q.err != nil:
		return q.err
	case q.dropping:
		return nil
	case q.closed:
		return ErrClosed
	}
	c := &runnerv1.LogChunk{Seq: q.nextSeq, Stream: stream, StepIndex: step, EmittedAt: timestamppb.New(at), Data: data}
	if q.spilled() == 0 && q.memBytes+size <= q.memMax {
		q.mem = append(q.mem, c)
		q.memBytes += size
	} else if err := q.toSpill(c); err != nil {
		q.err = err
		q.cond.Broadcast()
		return err
	}
	q.nextSeq++
	q.cond.Broadcast()
	return nil
}

func (q *queue) toSpill(c *runnerv1.LogChunk) error {
	if q.spill == nil {
		store, path, err := q.openSpill()
		if err != nil {
			return err
		}
		q.spill = &spill{store: store, path: path}
	}
	return q.spill.push(c)
}

func (q *queue) pop(maxBytes int64, maxChunks int) ([]*runnerv1.LogChunk, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.mem) == 0 && q.spilled() == 0 && !q.closed && q.err == nil {
		q.cond.Wait()
	}
	var out []*runnerv1.LogChunk
	var size int64
	for len(out) < maxChunks {
		var next *runnerv1.LogChunk
		switch {
		case len(q.mem) > 0:
			next = q.mem[0]
			if len(out) > 0 && size+int64(len(next.GetData())) > maxBytes {
				return out, nil
			}
			q.mem = q.mem[1:]
			q.memBytes -= int64(len(next.GetData()))
		case q.spilled() > 0:
			c, err := q.spill.pop()
			if err != nil {
				q.err = err
				q.cond.Broadcast()
				return out, err
			}
			next = c
		default:
			q.cond.Broadcast()
			return out, nil
		}
		size += int64(len(next.GetData()))
		out = append(out, next)
		if size >= maxBytes {
			break
		}
	}
	q.cond.Broadcast()
	return out, nil
}

func (q *queue) bumpSeq(next int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.nextSeq = max(q.nextSeq, next)
}

func (q *queue) stopSending() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dropping = true
	q.mem, q.memBytes = nil, 0
	if q.spill != nil {
		q.spill.read, q.spill.write, q.spill.count = 0, 0, 0
		_ = q.spill.store.Truncate(0)
	}
	q.cond.Broadcast()
}

func (q *queue) unblock() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.unblocked = true
	q.cond.Broadcast()
	return q.dropped
}

func (q *queue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func (q *queue) release() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.mem, q.memBytes = nil, 0
	q.cond.Broadcast()
	if q.spill == nil {
		return nil
	}
	err := q.spill.close()
	q.spill = nil
	return err
}
