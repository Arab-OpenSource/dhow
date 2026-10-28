package jobs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Models"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	noStepIndex          = -1
	pgCheckViolation     = "23514"
	logChunksTable       = "LogChunks"
	truncatedLineFormat  = "dhow: LOG_TRUNCATED: this attempt's log reached its limit of %d bytes, so later output is dropped\n"
	lateResultLineFormat = "dhow: LATE_RESULT: the runner reported %s (exit code %d, reason %s) after its lease ended, so the result was not applied\n"
)

var errLogPartitionMissing = errors.New("no log partition covers the current day")

type logRow struct {
	seq     int64
	stream  string
	step    int32
	emitted *time.Time
	data    []byte
}

type logState struct {
	lastSeq   int64
	bytes     int64
	truncated bool
}

type logPlan struct {
	rows      []logRow
	lastSeq   int64
	bytes     int64
	truncates bool
	expected  int64
}

func streamName(s runnerv1.LogStream) string {
	switch s {
	case runnerv1.LogStream_LOG_STREAM_STDOUT:
		return models.LogStdout
	case runnerv1.LogStream_LOG_STREAM_STDERR:
		return models.LogStderr
	case runnerv1.LogStream_LOG_STREAM_SYSTEM:
		return models.LogSystem
	default:
		return ""
	}
}

func validateChunks(chunks []*runnerv1.LogChunk) error {
	if len(chunks) > config.LogAppendMaxChunks {
		return runnerauth.ValidationFailed("Chunks")
	}
	for _, c := range chunks {
		switch {
		case c.GetSeq() < 1:
			return runnerauth.ValidationFailed("Seq")
		case streamName(c.GetStream()) == "":
			return runnerauth.ValidationFailed("Stream")
		case len(c.GetData()) == 0 || len(c.GetData()) > config.LogChunkMaxBytes:
			return runnerauth.ValidationFailed("Data")
		case c.StepIndex != nil && (c.GetStepIndex() < 0 || c.GetStepIndex() > config.LogMaxStepIndex):
			return runnerauth.ValidationFailed("StepIndex")
		case c.GetEmittedAt() != nil && c.GetEmittedAt().CheckValid() != nil:
			return runnerauth.ValidationFailed("EmittedAt")
		}
	}
	return nil
}

func ordered(chunks []*runnerv1.LogChunk) []*runnerv1.LogChunk {
	out := slices.Clone(chunks)
	slices.SortStableFunc(out, func(a, b *runnerv1.LogChunk) int { return cmp.Compare(a.GetSeq(), b.GetSeq()) })
	return slices.CompactFunc(out, func(a, b *runnerv1.LogChunk) bool { return a.GetSeq() == b.GetSeq() })
}

func rowOf(c *runnerv1.LogChunk, data []byte) logRow {
	r := logRow{seq: c.GetSeq(), stream: streamName(c.GetStream()), step: noStepIndex, data: data}
	if c.StepIndex != nil {
		r.step = c.GetStepIndex()
	}
	if c.GetEmittedAt() != nil {
		t := c.GetEmittedAt().AsTime()
		r.emitted = &t
	}
	return r
}

func planAppend(state logState, maxBytes int64, chunks []*runnerv1.LogChunk) logPlan {
	p := logPlan{lastSeq: state.lastSeq, bytes: state.bytes}
	truncated := state.truncated
	gap := false
	for _, c := range ordered(chunks) {
		if c.GetSeq() <= p.lastSeq {
			continue
		}
		if c.GetSeq() != p.lastSeq+1 {
			gap = true
			break
		}
		p.lastSeq = c.GetSeq()
		if truncated {
			continue
		}
		data := c.GetData()
		if room := max(maxBytes-p.bytes, 0); int64(len(data)) > room {
			data = data[:room]
			truncated, p.truncates = true, true
		}
		if len(data) > 0 {
			p.rows = append(p.rows, rowOf(c, data))
			p.bytes += int64(len(data))
		}
	}
	if p.truncates {
		line := []byte(fmt.Sprintf(truncatedLineFormat, maxBytes))
		p.lastSeq++
		p.rows = append(p.rows, logRow{seq: p.lastSeq, stream: models.LogSystem, step: noStepIndex, data: line})
		p.bytes += int64(len(line))
	}
	if gap {
		p.expected = p.lastSeq + 1
	}
	return p
}

func insertRows(ctx context.Context, q *sqlc.Queries, a sqlc.LockAttemptLogRow, rows []logRow) error {
	if len(rows) == 0 {
		return nil
	}
	arg := sqlc.InsertLogChunksParams{OrganizationID: a.OrganizationId, JobID: a.JobId, AttemptID: a.Id}
	for _, r := range rows {
		arg.Seqs = append(arg.Seqs, r.seq)
		arg.Streams = append(arg.Streams, r.stream)
		arg.StepIndexes = append(arg.StepIndexes, r.step)
		arg.HasEmittedAt = append(arg.HasEmittedAt, r.emitted != nil)
		emitted := time.Time{}
		if r.emitted != nil {
			emitted = *r.emitted
		}
		arg.EmittedAt = append(arg.EmittedAt, emitted)
		arg.Data = append(arg.Data, r.data)
	}
	if _, err := q.InsertLogChunks(ctx, arg); err != nil {
		return partitionError(err)
	}
	return q.NotifyLogAppended(ctx, a.Id.String()+":"+strconv.FormatInt(rows[len(rows)-1].seq, 10))
}

func partitionError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == pgCheckViolation && pg.TableName == logChunksTable && pg.ConstraintName == "" {
		return fmt.Errorf("%w: %w", errLogPartitionMissing, err)
	}
	return err
}

func (s *Service) Append(ctx context.Context, ref *runnerv1.LeaseRef, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	if err := validateChunks(chunks); err != nil {
		return nil, err
	}
	var res *runnerv1.AppendResponse
	var attempt uuid.UUID
	err := s.withLease(ctx, ref, func(q *sqlc.Queries, _ runnerauth.Principal, l sqlc.LockActiveLeaseRow) error {
		attempt = l.AttemptId
		a, err := q.LockAttemptLog(ctx, l.AttemptId)
		if err != nil {
			return err
		}
		plan := planAppend(logState{lastSeq: a.LastSeq, bytes: a.LogBytes, truncated: a.Truncated}, s.logs.MaxBytes, chunks)
		if err := insertRows(ctx, q, a, plan.rows); err != nil {
			return err
		}
		if plan.lastSeq != a.LastSeq || plan.bytes != a.LogBytes || plan.truncates {
			if err := q.AdvanceAttemptLog(ctx, sqlc.AdvanceAttemptLogParams{
				LastSeq: plan.lastSeq, LogBytes: plan.bytes, Truncated: plan.truncates, ID: a.Id,
			}); err != nil {
				return err
			}
		}
		res = &runnerv1.AppendResponse{LastSeq: plan.lastSeq, LogTruncated: a.Truncated || plan.truncates, Control: control(l.CancelRequestedAt)}
		if plan.expected > 0 {
			res.ExpectedSeq = &plan.expected
		}
		return nil
	})
	if errors.Is(err, errLogPartitionMissing) {
		s.log.ErrorContext(ctx, "a job log append failed because the log partition of today is missing",
			"attempt_id", attempt.String(), "error", err)
		return nil, logUnavailable()
	}
	return res, err
}

func (s *Service) recordLateResult(ctx context.Context, attemptID uuid.UUID, o models.Outcome) {
	exit := int32(0)
	if o.ExitCode != nil {
		exit = *o.ExitCode
	}
	reason := o.FailureReason
	if reason == "" {
		reason = "none"
	}
	line := []byte(fmt.Sprintf(lateResultLineFormat, o.State, exit, reason))
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		a, err := q.LockAttemptLog(ctx, attemptID)
		if err != nil || a.LateResultLogged {
			return err
		}
		row := logRow{seq: a.LastSeq + 1, stream: models.LogSystem, step: noStepIndex, data: line}
		if err := insertRows(ctx, q, a, []logRow{row}); err != nil {
			return err
		}
		return q.MarkLateResultLogged(ctx, sqlc.MarkLateResultLoggedParams{
			LastSeq: row.seq, LogBytes: a.LogBytes + int64(len(line)), ID: a.Id,
		})
	})
	if err != nil {
		s.log.WarnContext(ctx, "the late result of an attempt was not written to its log", "attempt_id", attemptID.String(), "error", err)
	}
}
