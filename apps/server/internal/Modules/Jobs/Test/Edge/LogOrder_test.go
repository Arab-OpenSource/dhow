package edge_test

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	orderChunks  = 300
	orderSenders = 3
	orderMaxSend = 2000
)

func lineOf(seq int64) string { return "line " + strconv.FormatInt(seq, 10) + " مرحبا\n" }

func TestLogAppendFromThreeGoroutinesStoresOneOrderedLog(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec())
	r := w.runner()
	g := r.MustStart(0)

	var acked atomic.Int64
	var calls atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, orderSenders)
	for sender := range orderSenders {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(uint64(sender), 42))
			from := int64(1)
			for acked.Load() < orderChunks && calls.Add(1) < orderMaxSend {
				start := from + int64(rng.IntN(8)) - 3
				start = min(max(start, 1), orderChunks)
				size := int64(1 + rng.IntN(12))
				var batch []*runnerv1.LogChunk
				for seq := start; seq < start+size && seq <= orderChunks; seq++ {
					batch = append(batch, apptest.Chunk(seq, lineOf(seq)))
					if rng.IntN(4) == 0 {
						batch = append(batch, apptest.Chunk(seq, lineOf(seq)))
					}
				}
				rng.Shuffle(len(batch), func(i, j int) { batch[i], batch[j] = batch[j], batch[i] })
				res, err := r.Append(g.Ref, batch...)
				if err != nil {
					errs <- err
					return
				}
				for {
					cur := acked.Load()
					if res.GetLastSeq() <= cur || acked.CompareAndSwap(cur, res.GetLastSeq()) {
						break
					}
				}
				from = res.GetLastSeq() + 1
				if res.ExpectedSeq != nil {
					from = res.GetExpectedSeq()
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, orderChunks, acked.Load(), "the senders finished within %d calls", orderMaxSend)

	rows, err := w.env.Pool.Query(t.Context(),
		`SELECT "Seq", convert_from("Data", 'UTF8') FROM "LogChunks" WHERE "AttemptId" = $1 ORDER BY "Seq", "CreatedAt"`, g.Grant.GetAttemptId())
	require.NoError(t, err)
	defer rows.Close()
	var want, got strings.Builder
	next := int64(1)
	for rows.Next() {
		var seq int64
		var text string
		require.NoError(t, rows.Scan(&seq, &text))
		require.Equal(t, next, seq, "no gap and no duplicate")
		got.WriteString(text)
		want.WriteString(lineOf(seq))
		next++
	}
	require.NoError(t, rows.Err())
	assert.EqualValues(t, orderChunks+1, next)
	assert.Equal(t, want.String(), got.String())
	assert.Equal(t, orderChunks, count(t, w, `SELECT "LastSeq"::int FROM "JobAttempts" WHERE "Id" = $1`, g.Grant.GetAttemptId()))
}
