package logs

import (
	"errors"
	"time"

	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func (p *Pipeline) send() {
	defer close(p.sent)
	for {
		batch, err := p.q.pop(p.o.BatchBytes, p.o.BatchChunks)
		if err != nil {
			p.o.Log.Error("the log spill failed, so the attempt stops", "error", err)
			p.raise(err)
			p.disable()
			return
		}
		if len(batch) == 0 {
			return
		}
		p.deliver(batch)
		if p.ctx.Err() != nil {
			return
		}
	}
}

func (p *Pipeline) deliver(batch []*runnerv1.LogChunk) {
	attempt := 0
	for len(batch) > 0 && !p.disabled.Load() {
		res, err := p.o.Appender.Append(p.ctx, batch)
		if err != nil {
			if p.ctx.Err() != nil || p.giveUp(err) {
				return
			}
			if !p.wait(p.o.Backoff(attempt)) {
				return
			}
			attempt++
			continue
		}
		attempt = 0
		if c := res.GetControl(); c != nil {
			p.o.OnControl(c)
		}
		if res.GetLogTruncated() {
			p.o.Log.Info("the server truncated this attempt's log, so later output is dropped")
			p.truncated.Store(true)
			p.disable()
			return
		}
		p.q.bumpSeq(res.GetLastSeq() + 1)
		from := res.GetLastSeq() + 1
		if res.ExpectedSeq != nil {
			from = res.GetExpectedSeq()
		}
		batch = since(batch, from)
		if len(batch) > 0 && batch[0].GetSeq() != from {
			p.o.Log.Error("the server expects a log chunk this runner no longer has, so log shipping stops",
				"expected_seq", from, "first_seq", batch[0].GetSeq())
			p.disable()
			return
		}
	}
}

func (p *Pipeline) giveUp(err error) bool {
	switch {
	case errors.Is(err, ErrLeaseLost):
		p.raise(ErrLeaseLost)
	case errors.Is(err, ErrUnsupported):
		p.o.Log.Warn("the server does not accept logs, so this attempt's output is dropped")
	case errors.Is(err, ErrRejected):
		p.o.Log.Error("the server rejected a log batch, so log shipping stops", "error", err)
	default:
		return false
	}
	p.disable()
	return true
}

func (p *Pipeline) wait(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-p.ctx.Done():
		return false
	}
}

func since(batch []*runnerv1.LogChunk, from int64) []*runnerv1.LogChunk {
	for i, c := range batch {
		if c.GetSeq() >= from {
			return batch[i:]
		}
	}
	return nil
}
