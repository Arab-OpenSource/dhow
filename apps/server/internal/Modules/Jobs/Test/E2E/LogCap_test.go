package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func attemptCap(t *testing.T, w world, attempt string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), `SELECT "LogMaxBytes" FROM "JobAttempts" WHERE "Id" = $1`, attempt).Scan(&n))
	return n
}

func TestANewLogCapAppliesToAttemptsThatStartAfterIt(t *testing.T) {
	w := newWorld(t)
	_, first, g := w.running(t)
	assert.EqualValues(t, config.LogMaxBytesDefault, attemptCap(t, w, g.Grant.GetAttemptId()))

	_, err := w.env.Wired.Settings.Update(t.Context(), settingspec.GroupLogs, 0,
		[]instancesettings.Change{{Field: "JobLogMaxBytes", Value: "1048576"}}, w.ownerID)
	require.NoError(t, err)

	block := strings.Repeat("a", config.LogChunkMaxBytes)
	for batch := range 2 {
		chunks := make([]*runnerv1.LogChunk, 0, 10)
		for i := range 10 {
			chunks = append(chunks, apptest.Chunk(int64(batch*10+i+1), block))
		}
		res := first.MustAppend(g.Ref, chunks...)
		assert.False(t, res.GetLogTruncated(), "the running attempt keeps the cap it started with")
	}

	w.job(t, apptest.SimpleSpec(), nil)
	second := w.runner(apptest.SeedRunnerSpec{})
	g2 := second.MustStart(0)
	assert.EqualValues(t, 1<<20, attemptCap(t, w, g2.Grant.GetAttemptId()))
}
