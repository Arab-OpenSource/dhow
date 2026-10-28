package logs_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logs "github.com/Arab-OpenSource/dhow/apps/runner/internal/Logs"
	masking "github.com/Arab-OpenSource/dhow/apps/runner/internal/Masking"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type server struct {
	mu        sync.Mutex
	lastSeq   int64
	stored    []*runnerv1.LogChunk
	calls     int
	fail      int
	err       error
	capBytes  int
	bytes     int
	truncated bool
	gapOnce   bool
	cancel    bool
}

func (s *server) Append(_ context.Context, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.fail > 0 {
		s.fail--
		return nil, errors.New("unavailable")
	}
	res := &runnerv1.AppendResponse{Control: &runnerv1.LeaseControl{CancelRequested: s.cancel}}
	if s.gapOnce && len(chunks) > 1 {
		s.gapOnce = false
		chunks = chunks[:1]
	}
	for _, c := range chunks {
		if c.GetSeq() <= s.lastSeq {
			continue
		}
		if c.GetSeq() != s.lastSeq+1 {
			next := s.lastSeq + 1
			res.ExpectedSeq = &next
			break
		}
		s.lastSeq = c.GetSeq()
		if s.truncated {
			continue
		}
		if s.capBytes > 0 && s.bytes+len(c.GetData()) > s.capBytes {
			s.truncated = true
			s.lastSeq++
			continue
		}
		s.bytes += len(c.GetData())
		s.stored = append(s.stored, c)
	}
	res.LastSeq, res.LogTruncated = s.lastSeq, s.truncated
	return res, nil
}

func (s *server) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, c := range s.stored {
		b.Write(c.GetData())
	}
	return b.String()
}

func (s *server) chunks() []*runnerv1.LogChunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*runnerv1.LogChunk(nil), s.stored...)
}

func fast(int) time.Duration { return 5 * time.Millisecond }

func newPipe(t *testing.T, s logs.Appender, tweak func(*logs.Options)) *logs.Pipeline {
	t.Helper()
	o := logs.Options{Appender: s, Dir: t.TempDir(), Backoff: fast, FlushInterval: 20 * time.Millisecond}
	if tweak != nil {
		tweak(&o)
	}
	p := logs.New(o)
	t.Cleanup(func() { _ = p.Abort() })
	return p
}

func closeWithin(t *testing.T, p *logs.Pipeline) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, p.Close(ctx))
}

func TestOutputArrivesInOrderWithoutGaps(t *testing.T) {
	s := &server{}
	p := newPipe(t, s, nil)
	step := int32(0)
	out := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, &step)
	errOut := p.Writer(runnerv1.LogStream_LOG_STREAM_STDERR, &step)
	for i := range 50 {
		_, _ = io.WriteString(out, "out line\n")
		if i%5 == 0 {
			_, _ = io.WriteString(errOut, "err line\n")
		}
	}
	p.Systemf(nil, "dhow: done")
	closeWithin(t, p)
	cs := s.chunks()
	require.NotEmpty(t, cs)
	for i, c := range cs {
		assert.EqualValues(t, i+1, c.GetSeq())
	}
	assert.Equal(t, 50, strings.Count(s.text(), "out line\n"))
	assert.Equal(t, 10, strings.Count(s.text(), "err line\n"))
	assert.Contains(t, s.text(), "dhow: done\n")
}

func TestAPartialLineIsSentAfterTheFlushInterval(t *testing.T) {
	s := &server{}
	p := newPipe(t, s, nil)
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "progress 50%")
	assert.Eventually(t, func() bool { return s.text() == "progress 50%" }, 2*time.Second, 10*time.Millisecond)
}

func TestChunksRespectTheSizeLimitAndRuneBoundaries(t *testing.T) {
	s := &server{}
	p := newPipe(t, s, func(o *logs.Options) { o.ChunkBytes = 16 })
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	in := strings.Repeat("داو ", 40)
	_, _ = io.WriteString(w, in)
	closeWithin(t, p)
	assert.Equal(t, in, s.text())
	for _, c := range s.chunks() {
		assert.LessOrEqual(t, len(c.GetData()), 16)
		assert.True(t, utf8.Valid(c.GetData()), "chunk %q splits a character", c.GetData())
	}
}

func TestTransientFailuresAreRetriedWithTheSameSeqs(t *testing.T) {
	s := &server{fail: 5}
	p := newPipe(t, s, nil)
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	_, _ = io.WriteString(w, "one\n")
	_, _ = io.WriteString(w, "two\n")
	closeWithin(t, p)
	assert.Equal(t, "one\ntwo\n", s.text())
	assert.Greater(t, s.calls, 5)
}

func TestTheRunnerResendsFromExpectedSeq(t *testing.T) {
	s := &server{gapOnce: true}
	p := newPipe(t, s, func(o *logs.Options) { o.ChunkBytes = 4; o.FlushInterval = time.Hour })
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "aaaabbbbccccdddd")
	closeWithin(t, p)
	assert.Equal(t, "aaaabbbbccccdddd", s.text())
}

func TestAfterTruncationOutputIsDroppedAndTheWriterNeverBlocks(t *testing.T) {
	s := &server{capBytes: 10}
	p := newPipe(t, s, func(o *logs.Options) { o.ChunkBytes = 8; o.MemoryBytes = 16; o.SpillBytes = 64 })
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 10_000 {
			_, _ = io.WriteString(w, "12345678")
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the writer blocked after truncation")
	}
	closeWithin(t, p)
	assert.True(t, p.Truncated())
	assert.LessOrEqual(t, len(s.text()), 10)
}

func TestAServerWithoutLogsDoesNotStopTheJob(t *testing.T) {
	s := &server{err: logs.ErrUnsupported}
	p := newPipe(t, s, nil)
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "hello\n")
	closeWithin(t, p)
	assert.Nil(t, p.Err())
}

func TestALostLeaseIsRaised(t *testing.T) {
	s := &server{err: errors.Join(logs.ErrLeaseLost, errors.New("LEASE_LOST"))}
	p := newPipe(t, s, nil)
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "hello\n")
	select {
	case <-p.Fatal():
	case <-time.After(5 * time.Second):
		t.Fatal("no fatal signal")
	}
	assert.ErrorIs(t, p.Err(), logs.ErrLeaseLost)
}

func TestControlFromAppendReachesTheExecutor(t *testing.T) {
	s := &server{cancel: true}
	got := make(chan bool, 4)
	p := newPipe(t, s, func(o *logs.Options) {
		o.OnControl = func(c *runnerv1.LeaseControl) { got <- c.GetCancelRequested() }
	})
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "x\n")
	select {
	case v := <-got:
		assert.True(t, v)
	case <-time.After(5 * time.Second):
		t.Fatal("no control")
	}
}

type blocked struct{ release chan struct{} }

func (b blocked) Append(ctx context.Context, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	select {
	case <-b.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &runnerv1.AppendResponse{LastSeq: chunks[len(chunks)-1].GetSeq()}, nil
}

func TestAFullQueueAndSpillBlockTheWriter(t *testing.T) {
	b := blocked{release: make(chan struct{})}
	dir := t.TempDir()
	p := logs.New(logs.Options{Appender: b, Dir: dir, ChunkBytes: 64, MemoryBytes: 128, SpillBytes: 512, BatchChunks: 1, Backoff: fast})
	t.Cleanup(func() { _ = p.Abort() })
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	wrote := make(chan int, 1)
	go func() {
		n := 0
		for range 100 {
			_, _ = w.Write([]byte(strings.Repeat("x", 64)))
			n++
		}
		wrote <- n
	}()
	select {
	case <-wrote:
		t.Fatal("the writer never blocked")
	case <-time.After(300 * time.Millisecond):
	}
	info, err := os.Stat(filepath.Join(dir, "logs.spill"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	close(b.release)
	select {
	case n := <-wrote:
		assert.Equal(t, 100, n)
	case <-time.After(5 * time.Second):
		t.Fatal("the writer did not resume")
	}
	closeWithin(t, p)
	_, err = os.Stat(filepath.Join(dir, "logs.spill"))
	assert.ErrorIs(t, err, os.ErrNotExist, "the spill file is removed when the pipeline closes")
}

type fullDisk struct{ *os.File }

func (fullDisk) WriteAt([]byte, int64) (int, error) { return 0, syscall.ENOSPC }

func TestAFullDiskIsRaised(t *testing.T) {
	b := blocked{release: make(chan struct{})}
	p := logs.New(logs.Options{
		Appender: b, Dir: t.TempDir(), ChunkBytes: 8, MemoryBytes: 1, BatchChunks: 1, Backoff: fast,
		OpenSpill: func(path string) (logs.SpillStore, error) {
			f, err := logs.OpenSpillFile(path)
			if err != nil {
				return nil, err
			}
			osf, _ := f.(*os.File)
			return fullDisk{osf}, nil
		},
	})
	t.Cleanup(func() { _ = p.Abort() })
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	for range 4 {
		_, _ = w.Write([]byte("12345678"))
	}
	select {
	case <-p.Fatal():
	case <-time.After(5 * time.Second):
		t.Fatal("no fatal signal")
	}
	assert.ErrorIs(t, p.Err(), logs.ErrDiskFull)
	assert.ErrorIs(t, p.Err(), syscall.ENOSPC)
}

func TestOutputIsMaskedBeforeItIsQueued(t *testing.T) {
	m := masking.New()
	m.Add([]byte("hunter2-secret"))
	s := &server{}
	p := newPipe(t, s, func(o *logs.Options) { o.Masker = m; o.ChunkBytes = 5 })
	w := p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil)
	for _, part := range []string{"pass=hun", "ter2-se", "cret done\n"} {
		_, _ = io.WriteString(w, part)
	}
	closeWithin(t, p)
	assert.Equal(t, "pass=*** done\n", s.text())
}

func TestCloseGivesUpWhenTheServerIsGone(t *testing.T) {
	b := blocked{release: make(chan struct{})}
	p := logs.New(logs.Options{Appender: b, Dir: t.TempDir(), Backoff: fast})
	_, _ = io.WriteString(p.Writer(runnerv1.LogStream_LOG_STREAM_STDOUT, nil), "x\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, p.Close(ctx), logs.ErrNotDrained)
}
