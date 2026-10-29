package e2e_test

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type logPage struct {
	Attempt   int
	AttemptId string
	State     string
	LastSeq   int64
	Truncated bool
	NextSeq   *int64
	Chunks    []struct {
		Seq       int64
		Stream    string
		StepIndex *int
		EmittedAt *string
		Text      string
	}
}

func (w world) logs(t *testing.T, c *apptest.Client, job uuid.UUID, query string) (logPage, apptest.Response) {
	t.Helper()
	res := c.Get(w.path("/jobs/" + job.String() + "/logs" + query))
	var p logPage
	if res.Status == http.StatusOK {
		res.Decode(t, &p)
	}
	return p, res
}

func (w world) allText(t *testing.T, job uuid.UUID) (string, []int64) {
	t.Helper()
	var text strings.Builder
	var seqs []int64
	from := int64(1)
	for {
		p, res := w.logs(t, w.owner, job, "?Limit=500&FromSeq="+strconv.FormatInt(from, 10))
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		for _, c := range p.Chunks {
			text.WriteString(c.Text)
			seqs = append(seqs, c.Seq)
		}
		if p.NextSeq == nil {
			return text.String(), seqs
		}
		from = *p.NextSeq
	}
}

func (w world) running(t *testing.T) (uuid.UUID, *apptest.TestRunner, *apptest.Grant) {
	t.Helper()
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	return id, r, r.MustStart(0)
}

func TestAppendStoresTheChunksAndHistoryReadsThemBack(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	step := int32(1)
	emitted := timestamppb.Now()
	res := r.MustAppend(g.Ref,
		apptest.Chunk(1, "building\n"),
		&runnerv1.LogChunk{Seq: 2, Stream: runnerv1.LogStream_LOG_STREAM_STDERR, StepIndex: &step, EmittedAt: emitted, Data: []byte("مرحبا\n")},
		&runnerv1.LogChunk{Seq: 3, Stream: runnerv1.LogStream_LOG_STREAM_SYSTEM, Data: []byte{'o', 'k', 0xff, 0xfe, '\n'}},
	)
	assert.EqualValues(t, 3, res.GetLastSeq())
	assert.Nil(t, res.ExpectedSeq)
	assert.False(t, res.GetLogTruncated())
	assert.EqualValues(t, 45, res.GetControl().GetLeaseTtlSeconds())

	p, raw := w.logs(t, w.owner, id, "")
	require.Equal(t, http.StatusOK, raw.Status, string(raw.Raw))
	assert.Equal(t, "jobs.logs.loaded", raw.Message)
	assert.Equal(t, 1, p.Attempt)
	assert.Equal(t, g.Grant.GetAttemptId(), p.AttemptId)
	assert.Equal(t, "running", p.State)
	assert.EqualValues(t, 3, p.LastSeq)
	assert.Nil(t, p.NextSeq)
	require.Len(t, p.Chunks, 3)
	assert.Equal(t, "stdout", p.Chunks[0].Stream)
	assert.Nil(t, p.Chunks[0].StepIndex)
	assert.Nil(t, p.Chunks[0].EmittedAt)
	assert.Equal(t, "stderr", p.Chunks[1].Stream)
	require.NotNil(t, p.Chunks[1].StepIndex)
	assert.Equal(t, 1, *p.Chunks[1].StepIndex)
	assert.NotNil(t, p.Chunks[1].EmittedAt)
	assert.Equal(t, "مرحبا\n", p.Chunks[1].Text)
	assert.Equal(t, "system", p.Chunks[2].Stream)
	assert.Equal(t, "ok��\n", p.Chunks[2].Text, "every invalid byte reads as U+FFFD")

	var stored []byte
	require.NoError(t, w.env.Pool.QueryRow(t.Context(),
		`SELECT "Data" FROM "LogChunks" WHERE "AttemptId" = $1 AND "Seq" = 3`, g.Grant.GetAttemptId()).Scan(&stored))
	assert.Equal(t, []byte{'o', 'k', 0xff, 0xfe, '\n'}, stored, "the bytes are stored as sent")
}

func TestAGapAnswersExpectedSeqAndDuplicatesAreIgnored(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	r.MustAppend(g.Ref, apptest.Chunk(1, "a"), apptest.Chunk(2, "b"), apptest.Chunk(3, "c"))

	res := r.MustAppend(g.Ref, apptest.Chunk(5, "e"), apptest.Chunk(6, "f"))
	assert.EqualValues(t, 3, res.GetLastSeq())
	require.NotNil(t, res.ExpectedSeq)
	assert.EqualValues(t, 4, res.GetExpectedSeq())

	res = r.MustAppend(g.Ref, apptest.Chunk(3, "X"), apptest.Chunk(2, "Y"), apptest.Chunk(4, "d"), apptest.Chunk(4, "Z"), apptest.Chunk(6, "f"))
	assert.EqualValues(t, 4, res.GetLastSeq(), "the contiguous part is kept and the rest waits")
	require.NotNil(t, res.ExpectedSeq)
	assert.EqualValues(t, 5, res.GetExpectedSeq())

	res = r.MustAppend(g.Ref, apptest.Chunk(6, "f"), apptest.Chunk(5, "e"))
	assert.EqualValues(t, 6, res.GetLastSeq(), "a batch out of order is put in order")
	assert.Nil(t, res.ExpectedSeq)

	res = r.MustAppend(g.Ref)
	assert.EqualValues(t, 6, res.GetLastSeq(), "an empty batch reports the position")

	text, seqs := w.allText(t, id)
	assert.Equal(t, "abcdef", text)
	assert.Equal(t, []int64{1, 2, 3, 4, 5, 6}, seqs)
}

func TestAppendNeedsTheCurrentLease(t *testing.T) {
	w := newWorld(t)
	_, r, g := w.running(t)
	other := w.runner(apptest.SeedRunnerSpec{})

	stale := &runnerv1.LeaseRef{LeaseId: g.Ref.GetLeaseId(), FencingToken: g.Ref.GetFencingToken() + 1}
	_, err := r.Append(stale, apptest.Chunk(1, "x"))
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
	_, err = other.Append(g.Ref, apptest.Chunk(1, "x"))
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")

	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	_, err = r.Append(g.Ref, apptest.Chunk(1, "x"))
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
	var n int
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "LogChunks"`).Scan(&n))
	assert.Zero(t, n)
}

func TestAppendWorksBeforeStartForImagePullLines(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	r.MustAppend(g.Ref, &runnerv1.LogChunk{Seq: 1, Stream: runnerv1.LogStream_LOG_STREAM_SYSTEM, Data: []byte("pulling image\n")})
	p, _ := w.logs(t, w.owner, id, "")
	assert.Equal(t, "leased", p.State)
	require.Len(t, p.Chunks, 1)
}

func TestAppendRefusesMalformedChunks(t *testing.T) {
	w := newWorld(t)
	_, r, g := w.running(t)
	badStep := int32(100)
	tooMany := make([]*runnerv1.LogChunk, config.LogAppendMaxChunks+1)
	for i := range tooMany {
		tooMany[i] = apptest.Chunk(int64(i+1), "x")
	}
	tests := []struct {
		name   string
		chunks []*runnerv1.LogChunk
		field  string
	}{
		{"seq zero", []*runnerv1.LogChunk{apptest.Chunk(0, "x")}, "Seq"},
		{"no stream", []*runnerv1.LogChunk{{Seq: 1, Data: []byte("x")}}, "Stream"},
		{"empty data", []*runnerv1.LogChunk{apptest.Chunk(1, "")}, "Data"},
		{"a chunk over 64 KiB", []*runnerv1.LogChunk{apptest.Chunk(1, strings.Repeat("x", config.LogChunkMaxBytes+1))}, "Data"},
		{"a step past the last", []*runnerv1.LogChunk{{Seq: 1, Stream: runnerv1.LogStream_LOG_STREAM_STDOUT, StepIndex: &badStep, Data: []byte("x")}}, "StepIndex"},
		{"an invalid time", []*runnerv1.LogChunk{{Seq: 1, Stream: runnerv1.LogStream_LOG_STREAM_STDOUT, EmittedAt: &timestamppb.Timestamp{Nanos: -1}, Data: []byte("x")}}, "EmittedAt"},
		{"too many chunks", tooMany, "Chunks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Append(g.Ref, tt.chunks...)
			requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: "+tt.field)
		})
	}
	_, err := r.Append(g.Ref, apptest.Chunk(1, strings.Repeat("x", config.LogChunkMaxBytes)))
	require.NoError(t, err, "exactly 64 KiB is accepted")
	big := make([]*runnerv1.LogChunk, 17)
	for i := range big {
		big[i] = apptest.Chunk(int64(i+2), strings.Repeat("y", config.LogChunkMaxBytes))
	}
	_, err = r.Append(g.Ref, big...)
	require.Error(t, err, "a request over 1 MiB is refused")
}

func TestTheCapTruncatesOnceWithOneSystemLine(t *testing.T) {
	w := setup(t, apptest.NewWithOptions(t, apptest.Options{Profile: func(c *config.Config) {
		c.Logs = config.Logs{MaxStreams: 10}
		c.SetEnvSetting(config.EnvJobLogMaxBytes, strconv.Itoa(config.LogMaxBytesMin))
	}}))
	id, r, g := w.running(t)
	block := strings.Repeat("z", config.LogChunkMaxBytes)
	chunks := []*runnerv1.LogChunk{apptest.Chunk(1, strings.Repeat("h", 1000))}
	for i := 1; i < 10; i++ {
		chunks = append(chunks, apptest.Chunk(int64(i+1), block))
	}
	res := r.MustAppend(g.Ref, chunks...)
	assert.False(t, res.GetLogTruncated())
	chunks = chunks[:0]
	for i := 10; i < 18; i++ {
		chunks = append(chunks, apptest.Chunk(int64(i+1), block))
	}
	res = r.MustAppend(g.Ref, chunks...)
	assert.True(t, res.GetLogTruncated())
	assert.EqualValues(t, 19, res.GetLastSeq(), "the dropped chunks are acknowledged and the system line takes the next seq")

	res = r.MustAppend(g.Ref, apptest.Chunk(19, "discarded-output"), apptest.Chunk(20, "discarded-output"))
	assert.True(t, res.GetLogTruncated())
	assert.EqualValues(t, 20, res.GetLastSeq())
	_, err := r.Heartbeat(g.Ref)
	require.NoError(t, err, "the job keeps running")

	var runnerBytes, systemLines int
	var systemText string
	require.NoError(t, w.env.Pool.QueryRow(t.Context(),
		`SELECT COALESCE(sum(octet_length("Data")) FILTER (WHERE "Stream" = 'stdout'), 0), count(*) FILTER (WHERE "Stream" = 'system'),
		        COALESCE(string_agg(convert_from("Data", 'UTF8'), '') FILTER (WHERE "Stream" = 'system'), '')
		 FROM "LogChunks" WHERE "AttemptId" = $1`, g.Grant.GetAttemptId()).Scan(&runnerBytes, &systemLines, &systemText))
	assert.Equal(t, config.LogMaxBytesMin, runnerBytes, "the crossing chunk is cut at the limit")
	assert.Equal(t, 1, systemLines)
	assert.Contains(t, systemText, "LOG_TRUNCATED")
	assert.NotContains(t, systemText, "discarded-output")

	p, _ := w.logs(t, w.owner, id, "?FromSeq=17")
	assert.True(t, p.Truncated)
	require.Len(t, p.Chunks, 2)
	assert.EqualValues(t, 17, p.Chunks[0].Seq)
	assert.Len(t, p.Chunks[0].Text, config.LogMaxBytesMin-1000-15*config.LogChunkMaxBytes)
	assert.Equal(t, "system", p.Chunks[1].Stream)
	assert.EqualValues(t, 19, p.Chunks[1].Seq)
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
}

func TestALateResultIsWrittenOnceAsASystemLine(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	r.MustAppend(g.Ref, apptest.Chunk(1, "step one\n"))
	w.env.ExpireLease(g.Ref.GetLeaseId())

	err := r.Complete(g, failed, 2, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
	err = r.Complete(g, failed, 2, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
	w.env.Reconcile()
	err = r.Complete(g, failed, 2, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")

	p, _ := w.logs(t, w.owner, id, "?Attempt=1")
	assert.Equal(t, "lost", p.State, "the late result changes no state")
	require.Len(t, p.Chunks, 2)
	assert.Equal(t, "system", p.Chunks[1].Stream)
	assert.EqualValues(t, 2, p.Chunks[1].Seq)
	assert.Contains(t, p.Chunks[1].Text, "LATE_RESULT")
	assert.Contains(t, p.Chunks[1].Text, "failed (exit code 2, reason STEP_FAILED)")
}

func TestALateResultAfterTheReconcilerIsLoggedToo(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	w.env.ExpireLease(g.Ref.GetLeaseId())
	w.env.Reconcile()
	err := r.Complete(g, succeeded, 0, noReason)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
	p, _ := w.logs(t, w.owner, id, "?Attempt=1")
	require.Len(t, p.Chunks, 1)
	assert.Contains(t, p.Chunks[0].Text, "LATE_RESULT: the runner reported succeeded (exit code 0, reason none)")
}

func TestHistoryPagesByCountAndBySize(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	var chunks []*runnerv1.LogChunk
	for i := range 250 {
		chunks = append(chunks, apptest.Chunk(int64(i+1), "line "+strconv.Itoa(i+1)+"\n"))
	}
	r.MustAppend(g.Ref, chunks...)
	p, _ := w.logs(t, w.owner, id, "")
	require.Len(t, p.Chunks, config.LogPageDefaultLimit)
	require.NotNil(t, p.NextSeq)
	assert.EqualValues(t, 101, *p.NextSeq)
	p, _ = w.logs(t, w.owner, id, "?FromSeq=201&Limit=100")
	require.Len(t, p.Chunks, 50)
	assert.Nil(t, p.NextSeq)

	_, r2, g2 := w.running(t)
	block := strings.Repeat("q", config.LogChunkMaxBytes)
	for start := 1; start <= 20; start += 10 {
		var big []*runnerv1.LogChunk
		for i := range 10 {
			big = append(big, apptest.Chunk(int64(start+i), block))
		}
		r2.MustAppend(g2.Ref, big...)
	}
	job2 := uuid.MustParse(g2.Grant.GetJobId())
	p, _ = w.logs(t, w.owner, job2, "?Limit=500")
	assert.Len(t, p.Chunks, 16, "a page stops at about 1 MiB")
	require.NotNil(t, p.NextSeq)
	assert.EqualValues(t, 17, *p.NextSeq)
	text, _ := w.allText(t, job2)
	assert.Equal(t, strings.Repeat(block, 20), text)
}

func TestHistoryRefusesBadQueriesAndOtherOrganizations(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.running(t)
	queued := w.job(t, apptest.SimpleSpec(), nil)
	tests := []struct {
		name   string
		job    uuid.UUID
		query  string
		status int
		code   string
	}{
		{"an attempt that does not exist", id, "?Attempt=2", http.StatusNotFound, "ATTEMPT_NOT_FOUND"},
		{"a job that never started", queued, "", http.StatusNotFound, "ATTEMPT_NOT_FOUND"},
		{"a job that does not exist", uuid.New(), "", http.StatusNotFound, "NOT_FOUND"},
		{"attempt zero", id, "?Attempt=0", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"FromSeq zero", id, "?FromSeq=0", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"Limit over 500", id, "?Limit=501", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"Limit that is not a number", id, "?Limit=many", http.StatusBadRequest, "BAD_REQUEST"},
		{"an attempt past int32", id, "?Attempt=2147483648", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, res := w.logs(t, w.owner, tt.job, tt.query)
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, tt.code, res.Error.Code)
		})
	}

	other := w.env.SeedOrganization("Other", "other-logs")
	outsider := w.env.SeedSession(w.env.SeedUser(other, "outsider-logs@example.com", "a long password for outsider", "owner"))
	res := outsider.Get("/organizations/" + other.String() + "/jobs/" + id.String() + "/logs")
	assert.Equal(t, http.StatusNotFound, res.Status, "a job of another organization is not found through one's own")
	res = outsider.Get(w.path("/jobs/" + id.String() + "/logs"))
	assert.Equal(t, http.StatusNotFound, res.Status)
	viewer := w.env.SeedSession(w.env.SeedUser(w.org, "viewer-logs@example.com", "a long password for viewer", "viewer"))
	res = viewer.Get(w.path("/jobs/" + id.String() + "/logs"))
	assert.Equal(t, http.StatusForbidden, res.Status)
	assert.Equal(t, http.StatusOK, viewer.Get(w.path("/jobs/"+id.String())).Status, "the viewer still reads the job")
}

func TestTheServerNeverLogsTheContentOfLogs(t *testing.T) {
	w := newWorld(t)
	_, r, g := w.running(t)
	marker := "log-content-" + uuid.NewString()
	r.MustAppend(g.Ref, apptest.Chunk(1, marker+"\n"))

	var today string
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), `SELECT "LogPartitionName"("LogPartitionToday"())`).Scan(&today))
	_, err := w.env.Pool.Exec(t.Context(), `DROP TABLE "`+today+`"`)
	require.NoError(t, err)
	_, err = r.Append(g.Ref, apptest.Chunk(2, marker+" again\n"))
	requireRPC(t, err, connect.CodeUnavailable, "LOG_STORAGE_UNAVAILABLE")
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))

	logs := w.env.Logs.String()
	assert.Contains(t, logs, "the log partition of today is missing")
	assert.False(t, bytes.Contains([]byte(logs), []byte(marker)), "the server log carries no log content")
}

func TestHistoryIsRateLimitedPerUser(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.running(t)
	for i := range config.LogReadBurst {
		_, res := w.logs(t, w.owner, id, "")
		require.Equal(t, http.StatusOK, res.Status, "request %d", i+1)
	}
	_, res := w.logs(t, w.owner, id, "")
	assert.Equal(t, http.StatusTooManyRequests, res.Status)
	require.NotNil(t, res.Error)
	assert.Equal(t, "RATE_LIMITED", res.Error.Code)
	assert.Equal(t, "1", res.Header.Get("Retry-After"))
	other := w.env.SeedSession(w.env.SeedUser(w.org, "admin-rate@example.com", "a long password for admin", "admin"))
	_, res = w.logs(t, other, id, "")
	assert.Equal(t, http.StatusOK, res.Status, "the limit is per user")
}

func TestOpeningStreamsIsRateLimitedPerUser(t *testing.T) {
	w := newWorld(t)
	queued := w.job(t, apptest.SimpleSpec(), nil)
	srv := w.env.Serve()
	for i := range config.LogStreamOpenBurst {
		s := w.owner.OpenStream(t, srv, apptest.LogStreamPath(w.org, queued), nil)
		require.Equal(t, http.StatusNotFound, s.Status, "open %d", i+1)
	}
	s := w.owner.OpenStream(t, srv, apptest.LogStreamPath(w.org, queued), nil)
	assert.Equal(t, http.StatusTooManyRequests, s.Status)
	assert.Contains(t, s.Body, `"Code":"RATE_LIMITED"`)
}

func TestLogReadsUseTheirOwnBoundedPool(t *testing.T) {
	w := setup(t, apptest.NewWithOptions(t, apptest.Options{
		LogReadWait: 300 * time.Millisecond,
		Profile:     func(c *config.Config) { c.Logs = config.Logs{ReadConns: 1} },
	}))
	id, r, g := w.running(t)
	r.MustAppend(g.Ref, apptest.Chunk(1, "x"))
	logPool := w.env.Wired.LogPool
	require.NotNil(t, logPool)
	assert.EqualValues(t, 1, logPool.Config().MaxConns)
	var timeout, main string
	require.NoError(t, logPool.QueryRow(t.Context(), `SHOW statement_timeout`).Scan(&timeout))
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), `SHOW statement_timeout`).Scan(&main))
	assert.Equal(t, "5s", timeout)
	assert.Equal(t, "0", main, "the main pool keeps no statement timeout")

	held, err := logPool.Acquire(t.Context())
	require.NoError(t, err)
	start := time.Now()
	_, res := w.logs(t, w.owner, id, "")
	assert.Equal(t, http.StatusServiceUnavailable, res.Status, "a busy log pool answers 503 in bounded time")
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.Equal(t, http.StatusOK, w.owner.Get(w.path("/jobs/"+id.String())).Status, "the API keeps its own pool")
	held.Release()
	p, res := w.logs(t, w.owner, id, "")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Len(t, p.Chunks, 1)
}
