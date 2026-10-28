package edge_test

import (
	"encoding/json"
	"net/http"
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

const (
	edgeEventWait  = 5 * time.Second
	slowWriteWait  = 300 * time.Millisecond
	revokeDeadline = 16 * time.Second
)

func skipEvent(t *testing.T, s *apptest.Stream, name string) apptest.Event {
	t.Helper()
	for {
		ev, err := s.Next(edgeEventWait)
		require.NoError(t, err, "waiting for %s", name)
		if ev.Name == name {
			return ev
		}
	}
}

func chunkSeq(t *testing.T, ev apptest.Event) int64 {
	t.Helper()
	var c struct{ Seq int64 }
	require.NoError(t, json.Unmarshal([]byte(ev.Data), &c))
	return c.Seq
}

func eventually(t *testing.T, within time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

func runningJob(t *testing.T, w world) (uuid.UUID, *apptest.TestRunner, *apptest.Grant) {
	t.Helper()
	id := w.job(t, apptest.SimpleSpec())
	r := w.runner()
	return id, r, r.MustStart(0)
}

func TestASlowSubscriberIsClosedAndTheOthersKeepUp(t *testing.T) {
	w := setup(t, apptest.NewWithOptions(t, apptest.Options{LogStreamWriteWait: slowWriteWait}))
	id, r, g := runningJob(t, w)
	srv := w.env.ServeWithSmallBuffers()
	path := apptest.LogStreamPath(w.org, id)

	stalled := w.owner.StallingStream(t, srv, path)
	healthy := w.owner.OpenStream(t, srv, path, nil)
	require.Equal(t, http.StatusOK, healthy.Status, healthy.Body)
	skipEvent(t, healthy, "status")
	eventually(t, edgeEventWait, func() bool { return w.env.Wired.Jobs.OpenLogStreams() == 2 }, "both streams are open")

	block := strings.Repeat("s", config.LogChunkMaxBytes)
	const total = 48
	for start := int64(1); start <= total; start += 8 {
		var batch []*runnerv1.LogChunk
		for seq := start; seq < start+8; seq++ {
			batch = append(batch, apptest.Chunk(seq, block))
		}
		r.MustAppend(g.Ref, batch...)
	}
	for want := int64(1); want <= total; want++ {
		assert.Equal(t, want, chunkSeq(t, skipEvent(t, healthy, "chunk")))
	}
	eventually(t, edgeEventWait, func() bool { return w.env.Wired.Jobs.OpenLogStreams() == 1 },
		"the subscriber that stopped reading is closed")

	r.MustAppend(g.Ref, apptest.Chunk(total+1, "still here\n"))
	assert.EqualValues(t, total+1, chunkSeq(t, skipEvent(t, healthy, "chunk")), "the healthy stream is unaffected")
	assert.Contains(t, w.env.Logs.String(), "a log stream was closed")
	require.NoError(t, stalled.Close())
}

func TestARevokedSessionClosesItsStreamWithinSixteenSeconds(t *testing.T) {
	w := newWorld(t)
	id, r, g := runningJob(t, w)
	srv := w.env.Serve()
	path := apptest.LogStreamPath(w.org, id)
	developer := w.env.SeedUser(w.org, "developer-stream@example.com", "a long password for developer", "developer")
	revoked := w.env.SeedSession(developer)
	kept := w.env.SeedSession(developer)

	opened := time.Now()
	gone := revoked.OpenStream(t, srv, path, nil)
	stays := kept.OpenStream(t, srv, path, nil)
	require.Equal(t, http.StatusOK, gone.Status, gone.Body)
	require.Equal(t, http.StatusOK, stays.Status, stays.Body)
	skipEvent(t, gone, "status")
	skipEvent(t, stays, "status")

	revokedAt := time.Now()
	require.Equal(t, http.StatusOK, w.env.Logout(revoked).Status)
	require.True(t, gone.WaitClosed(revokeDeadline), "the revoked session's stream closes within 16 seconds")
	assert.Less(t, time.Since(revokedAt), revokeDeadline)

	assert.Greater(t, time.Since(opened), config.APITimeout, "the stream outlived the API timeout")
	r.MustAppend(g.Ref, apptest.Chunk(1, "after the revocation\n"))
	assert.EqualValues(t, 1, chunkSeq(t, skipEvent(t, stays, "chunk")), "the other session keeps streaming")
	assert.Equal(t, 1, w.env.Wired.Jobs.OpenLogStreams())
}

func TestTheStreamOutlivesTheServerReadAndWriteTimeouts(t *testing.T) {
	w := newWorld(t)
	id, r, g := runningJob(t, w)
	srv := w.env.ServeWithTimeouts(500*time.Millisecond, 500*time.Millisecond)
	s := w.owner.OpenStream(t, srv, apptest.LogStreamPath(w.org, id), nil)
	require.Equal(t, http.StatusOK, s.Status, s.Body)
	skipEvent(t, s, "status")
	time.Sleep(2 * time.Second)
	r.MustAppend(g.Ref, apptest.Chunk(1, "late but delivered\n"))
	assert.EqualValues(t, 1, chunkSeq(t, skipEvent(t, s, "chunk")))
}

func TestStreamsAreLimitedPerUserAndPerInstance(t *testing.T) {
	w := setup(t, apptest.NewWithOptions(t, apptest.Options{Profile: func(c *config.Config) {
		c.Logs = config.Logs{MaxBytes: config.LogMaxBytesDefault, RetentionDays: 30, MaxStreams: config.LogStreamsPerUser + 2}
	}}))
	id, _, _ := runningJob(t, w)
	srv := w.env.Serve()
	path := apptest.LogStreamPath(w.org, id)
	open := func(c *apptest.Client) *apptest.Stream {
		s := c.OpenStream(t, srv, path, nil)
		if s.Status == http.StatusOK {
			skipEvent(t, s, "status")
		}
		return s
	}

	var mine []*apptest.Stream
	for range config.LogStreamsPerUser {
		s := open(w.owner)
		require.Equal(t, http.StatusOK, s.Status, s.Body)
		mine = append(mine, s)
	}
	refused := open(w.owner)
	assert.Equal(t, http.StatusTooManyRequests, refused.Status)
	assert.Contains(t, refused.Body, `"Code":"TOO_MANY_STREAMS"`)
	assert.Equal(t, "5", refused.Header.Get("Retry-After"))

	other := w.env.SeedSession(w.env.SeedUser(w.org, "admin-stream@example.com", "a long password for admin", "admin"))
	require.Equal(t, http.StatusOK, open(other).Status, "another user has a budget of their own")
	require.Equal(t, http.StatusOK, open(other).Status)
	assert.Equal(t, http.StatusTooManyRequests, open(other).Status, "the instance limit holds for everyone")

	mine[0].Close()
	eventually(t, edgeEventWait, func() bool { return w.env.Wired.Jobs.OpenLogStreams() == config.LogStreamsPerUser+1 },
		"a closed stream frees its slot")
	assert.Equal(t, http.StatusOK, open(w.owner).Status)
}
