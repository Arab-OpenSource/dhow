package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Khaymah/dhow/apps/server/internal/Modules/Jobs/Models"
)

const (
	eventChunk  = "chunk"
	eventStatus = "status"
	eventEnd    = "end"
)

func (s *Service) OpenLogStreams() int { return s.streams.open() }

type EventRef struct {
	Attempt int
	Seq     int64
}

func (r EventRef) ID() string { return strconv.Itoa(r.Attempt) + ":" + strconv.FormatInt(r.Seq, 10) }

type StreamRequest struct {
	Organization uuid.UUID
	Job          uuid.UUID
	Attempt      *int
	FromSeq      int64
	LastEvent    *EventRef
	User         uuid.UUID
	Reauthorize  func(ctx context.Context) error
}

type LogStream struct {
	service *Service
	ctx     context.Context
	req     StreamRequest
	attempt sqlc.GetLogAttemptRow
	after   int64
	release func()
}

func (s *Service) OpenLogStream(ctx context.Context, req StreamRequest) (*LogStream, error) {
	if err := s.openRate.allow(req.User, time.Now()); err != nil {
		return nil, err
	}
	a, err := s.logAttempt(ctx, req.Organization, req.Job, req.Attempt)
	if err != nil {
		return nil, err
	}
	release, err := s.streams.acquire(req.User, req.Organization)
	if err != nil {
		return nil, err
	}
	after := max(req.FromSeq-1, 0)
	if req.LastEvent != nil && req.LastEvent.Attempt == int(a.Number) {
		after = req.LastEvent.Seq
	}
	return &LogStream{service: s, ctx: ctx, req: req, attempt: a, after: after, release: release}, nil
}

type eventWriter struct {
	w    http.ResponseWriter
	rc   *http.ResponseController
	wait time.Duration
	buf  bytes.Buffer
}

func supported(err error) error {
	if errors.Is(err, http.ErrNotSupported) {
		return nil
	}
	return err
}

func (e *eventWriter) start() error {
	if err := supported(e.rc.SetReadDeadline(time.Time{})); err != nil {
		return err
	}
	h := e.w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	e.w.WriteHeader(http.StatusOK)
	e.buf.WriteString("retry: " + strconv.FormatInt(config.LogStreamRetry.Milliseconds(), 10) + "\n\n")
	return e.flush()
}

func (e *eventWriter) event(name, id string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e.buf.WriteString("event: " + name + "\n")
	if id != "" {
		e.buf.WriteString("id: " + id + "\n")
	}
	e.buf.WriteString("data: ")
	e.buf.Write(data)
	e.buf.WriteString("\n\n")
	return nil
}

func (e *eventWriter) comment() error {
	e.buf.WriteString(": keep-alive\n\n")
	return e.flush()
}

func (e *eventWriter) flush() error {
	if e.buf.Len() == 0 {
		return nil
	}
	if err := supported(e.rc.SetWriteDeadline(time.Now().Add(e.wait))); err != nil {
		return err
	}
	if _, err := e.w.Write(e.buf.Bytes()); err != nil {
		return err
	}
	e.buf.Reset()
	return supported(e.rc.Flush())
}

type ChunkEvent struct {
	Seq       int64
	Stream    string
	StepIndex *int       `json:",omitempty"`
	EmittedAt *time.Time `json:",omitempty"`
	CreatedAt time.Time
	Text      string
}

func (ls *LogStream) Serve(w http.ResponseWriter) error {
	defer ls.release()
	s := ls.service
	wake, unsubscribe := s.hub.subscribeLog(ls.attempt.Id)
	defer unsubscribe()
	out := &eventWriter{w: w, rc: http.NewResponseController(w), wait: s.wait}
	if err := out.start(); err != nil {
		return nil
	}
	tick := time.NewTicker(config.LogStreamTick)
	defer tick.Stop()
	last := ls.after
	var sent *models.LogStatus
	for {
		done, err := ls.pump(out, &last, &sent)
		if err != nil {
			ls.closed(err)
			return nil
		}
		if done {
			return nil
		}
		select {
		case <-ls.ctx.Done():
			return nil
		case <-s.stop:
			return nil
		case <-wake:
		case <-tick.C:
			if err := ls.req.Reauthorize(ls.ctx); err != nil {
				return nil
			}
			if err := out.comment(); err != nil {
				ls.closed(err)
				return nil
			}
		}
	}
}

func (ls *LogStream) closed(err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	ls.service.log.InfoContext(context.WithoutCancel(ls.ctx), "a log stream was closed",
		"attempt_id", ls.attempt.Id.String(), "slow_client", errors.Is(err, os.ErrDeadlineExceeded), "error", err)
}

func (ls *LogStream) pump(out *eventWriter, last *int64, sent **models.LogStatus) (bool, error) {
	st, err := ls.service.attemptState(ls.ctx, ls.req.Organization, ls.attempt.Id)
	if err != nil {
		return false, err
	}
	status := models.LogStatus{Attempt: int(ls.attempt.Number), State: st.State, Truncated: st.Truncated}
	if *sent == nil || **sent != status {
		if err := out.event(eventStatus, "", statusEvent(status)); err != nil {
			return false, err
		}
		*sent = &status
	}
	for {
		chunks, err := ls.service.readChunks(ls.ctx, ls.req.Organization, ls.attempt, *last+1, config.LogStreamBatchRows, config.LogStreamBatchBytes)
		if err != nil {
			return false, err
		}
		for _, c := range chunks {
			ev := ChunkEvent{Seq: c.Seq, Stream: c.Stream, StepIndex: c.StepIndex, EmittedAt: c.EmittedAt, CreatedAt: c.CreatedAt, Text: c.Text}
			if err := out.event(eventChunk, EventRef{Attempt: int(ls.attempt.Number), Seq: c.Seq}.ID(), ev); err != nil {
				return false, err
			}
			*last = c.Seq
		}
		if err := out.flush(); err != nil {
			return false, err
		}
		if len(chunks) == 0 || *last >= st.LastSeq {
			break
		}
	}
	if !models.AttemptFinished(st.State) {
		return false, nil
	}
	if err := out.event(eventEnd, "", statusEvent(status)); err != nil {
		return false, err
	}
	return true, out.flush()
}

type StatusEvent struct {
	Attempt   int
	State     string
	Truncated bool
}

func statusEvent(s models.LogStatus) StatusEvent {
	return StatusEvent(s)
}
