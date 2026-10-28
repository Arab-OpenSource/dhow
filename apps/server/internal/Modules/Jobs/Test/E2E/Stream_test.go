package e2e_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const eventWait = 5 * time.Second

type chunkEvent struct {
	Seq    int64
	Stream string
	Text   string
}

type statusEvent struct {
	Attempt   int
	State     string
	Truncated bool
}

func nextEvent(t *testing.T, s *apptest.Stream, name string) apptest.Event {
	t.Helper()
	for {
		ev, err := s.Next(eventWait)
		require.NoError(t, err, "waiting for %s", name)
		if ev.Name == "comment" {
			continue
		}
		require.Equal(t, name, ev.Name, "event data: %s", ev.Data)
		return ev
	}
}

func nextChunk(t *testing.T, s *apptest.Stream) chunkEvent {
	t.Helper()
	ev := nextEvent(t, s, "chunk")
	var c chunkEvent
	require.NoError(t, json.Unmarshal([]byte(ev.Data), &c))
	require.Equal(t, strconv.FormatInt(c.Seq, 10), ev.ID, "the event id is the seq")
	return c
}

func nextStatus(t *testing.T, s *apptest.Stream, name string) statusEvent {
	t.Helper()
	var st statusEvent
	require.NoError(t, json.Unmarshal([]byte(nextEvent(t, s, name).Data), &st))
	return st
}

func streamText(seq int64) string {
	return "خطوة " + strconv.FormatInt(seq, 10) + " \x1b[32mok\x1b[0m\n"
}

func appendRange(t *testing.T, r *apptest.TestRunner, g *apptest.Grant, from, to int64) string {
	t.Helper()
	var chunks []*runnerv1.LogChunk
	var all strings.Builder
	for seq := from; seq <= to; seq++ {
		chunks = append(chunks, apptest.Chunk(seq, streamText(seq)))
		all.WriteString(streamText(seq))
	}
	r.MustAppend(g.Ref, chunks...)
	return all.String()
}

func TestTheStreamResumesWithTheSameBytesAfterADisconnect(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	srv := w.env.Serve()
	path := apptest.LogStreamPath(w.org, id)
	want := appendRange(t, r, g, 1, 50)

	first := w.owner.OpenStream(t, srv, path, nil)
	require.Equal(t, http.StatusOK, first.Status, first.Body)
	assert.Equal(t, "text/event-stream; charset=utf-8", first.Header.Get("Content-Type"))
	assert.Equal(t, "no", first.Header.Get("X-Accel-Buffering"))
	assert.Equal(t, statusEvent{Attempt: 1, State: "running"}, nextStatus(t, first, "status"))
	var got strings.Builder
	var last int64
	for range 20 {
		c := nextChunk(t, first)
		require.Equal(t, last+1, c.Seq)
		got.WriteString(c.Text)
		last = c.Seq
	}
	first.Close()
	require.True(t, first.WaitClosed(eventWait))

	want += appendRange(t, r, g, 51, 80)
	second := w.owner.OpenStream(t, srv, path, map[string]string{"Last-Event-ID": strconv.FormatInt(last, 10)})
	require.Equal(t, http.StatusOK, second.Status, second.Body)
	nextStatus(t, second, "status")
	for last < 80 {
		c := nextChunk(t, second)
		require.Equal(t, last+1, c.Seq, "no gap and no duplicate after the resume")
		got.WriteString(c.Text)
		last = c.Seq
	}
	want += appendRange(t, r, g, 81, 81)
	c := nextChunk(t, second)
	assert.EqualValues(t, 81, c.Seq, "a live chunk arrives on the open stream")
	got.WriteString(c.Text)

	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	assert.Equal(t, statusEvent{Attempt: 1, State: "succeeded"}, nextStatus(t, second, "status"))
	assert.Equal(t, statusEvent{Attempt: 1, State: "succeeded"}, nextStatus(t, second, "end"))
	assert.True(t, second.WaitClosed(eventWait), "the stream closes after end")
	assert.Equal(t, want, got.String(), "the bytes are the same as sent")
}

func TestAStreamOfAFinishedAttemptReplaysAndEnds(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	appendRange(t, r, g, 1, 5)
	require.NoError(t, r.Complete(g, failed, 1, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED))
	srv := w.env.Serve()

	s := w.owner.OpenStream(t, srv, apptest.LogStreamPath(w.org, id)+"?FromSeq=3&Attempt=1", nil)
	require.Equal(t, http.StatusOK, s.Status, s.Body)
	assert.Equal(t, "failed", nextStatus(t, s, "status").State)
	for want := int64(3); want <= 5; want++ {
		assert.Equal(t, want, nextChunk(t, s).Seq)
	}
	assert.Equal(t, "failed", nextStatus(t, s, "end").State)
	assert.True(t, s.WaitClosed(eventWait))
	assert.Zero(t, w.env.Wired.Jobs.OpenLogStreams(), "a closed stream frees its slot")
}

func TestTheStreamRefusesWhatHistoryRefuses(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.running(t)
	queued := w.job(t, apptest.SimpleSpec(), nil)
	srv := w.env.Serve()
	viewer := w.env.SeedSession(w.env.SeedUser(w.org, "viewer-stream@example.com", "a long password for viewer", "viewer"))
	tests := []struct {
		name    string
		client  *apptest.Client
		path    string
		headers map[string]string
		status  int
		code    string
	}{
		{"anonymous", &apptest.Client{Env: w.env}, apptest.LogStreamPath(w.org, id), nil, http.StatusUnauthorized, "UNAUTHENTICATED"},
		{"a viewer", viewer, apptest.LogStreamPath(w.org, id), nil, http.StatusForbidden, "FORBIDDEN"},
		{"a job that never started", w.owner, apptest.LogStreamPath(w.org, queued), nil, http.StatusNotFound, "ATTEMPT_NOT_FOUND"},
		{"another organization's id in the path", w.owner, apptest.LogStreamPath(uuid.New(), id), nil, http.StatusNotFound, "NOT_FOUND"},
		{"a negative Last-Event-ID", w.owner, apptest.LogStreamPath(w.org, id), map[string]string{"Last-Event-ID": "-1"}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"a Last-Event-ID that is not a number", w.owner, apptest.LogStreamPath(w.org, id), map[string]string{"Last-Event-ID": "x"}, http.StatusBadRequest, "BAD_REQUEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.client.OpenStream(t, srv, tt.path, tt.headers)
			assert.Equal(t, tt.status, s.Status, s.Body)
			assert.Contains(t, s.Body, `"Code":"`+tt.code+`"`)
		})
	}
	assert.Zero(t, w.env.Wired.Jobs.OpenLogStreams())
}

func TestTwoInstancesOnOneDatabaseShareTheStream(t *testing.T) {
	w := newWorld(t)
	id, r, g := w.running(t)
	other := apptest.NewWithOptions(t, apptest.Options{Pool: w.env.Pool, Profile: func(c *config.Config) { c.SecretKey = w.env.Config.SecretKey }})
	srv := other.Serve()
	watcher := &apptest.Client{Env: other, Cookie: w.env.SeedSession(w.ownerID).Cookie}

	s := watcher.OpenStream(t, srv, apptest.LogStreamPath(w.org, id), nil)
	require.Equal(t, http.StatusOK, s.Status, s.Body)
	nextStatus(t, s, "status")
	for seq := int64(1); seq <= 3; seq++ {
		r.MustAppend(g.Ref, apptest.Chunk(seq, streamText(seq)))
		start := time.Now()
		c := nextChunk(t, s)
		assert.Equal(t, seq, c.Seq)
		assert.Equal(t, streamText(seq), c.Text)
		assert.Less(t, time.Since(start), 3*time.Second, "the other instance hears NOTIFY, not the 15 second fallback")
	}
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	nextStatus(t, s, "status")
	nextStatus(t, s, "end")
}

func TestServerShutdownEndsOpenStreams(t *testing.T) {
	w := newWorld(t)
	id, _, _ := w.running(t)
	srv := w.env.Serve()
	s := w.owner.OpenStream(t, srv, apptest.LogStreamPath(w.org, id), nil)
	require.Equal(t, http.StatusOK, s.Status, s.Body)
	nextStatus(t, s, "status")
	w.env.Wired.Jobs.Drain()
	assert.True(t, s.WaitClosed(eventWait), "a draining instance lets the browser reconnect elsewhere")
	assert.Zero(t, w.env.Wired.Jobs.OpenLogStreams())
}
