package apptest

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	smallSocketBuffer = 4096
	streamEventBuffer = 1 << 14
)

var (
	ErrStreamClosed  = errors.New("the stream is closed")
	ErrStreamTimeout = errors.New("no event in time")
)

func Chunk(seq int64, text string) *runnerv1.LogChunk {
	return &runnerv1.LogChunk{Seq: seq, Stream: runnerv1.LogStream_LOG_STREAM_STDOUT, Data: []byte(text)}
}

func (r *TestRunner) Append(ref *runnerv1.LeaseRef, chunks ...*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	res, err := r.Client.Logs.Append(context.Background(), connect.NewRequest(&runnerv1.AppendRequest{Lease: ref, Chunks: chunks}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (r *TestRunner) MustAppend(ref *runnerv1.LeaseRef, chunks ...*runnerv1.LogChunk) *runnerv1.AppendResponse {
	r.T.Helper()
	res, err := r.Append(ref, chunks...)
	require.NoError(r.T, err)
	return res
}

func (r *TestRunner) MustStart(wait int32) *Grant {
	r.T.Helper()
	g := r.MustLease(wait)
	_, err := r.Start(g)
	require.NoError(r.T, err)
	return g
}

type smallBufferListener struct{ net.Listener }

func (l smallBufferListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok && err == nil {
		if serr := tc.SetWriteBuffer(smallSocketBuffer); serr != nil {
			return nil, serr
		}
	}
	return c, err
}

func (e *Env) Serve() *httptest.Server {
	e.T.Helper()
	srv := httptest.NewServer(e.Handler)
	e.T.Cleanup(srv.Close)
	return srv
}

func (e *Env) ServeWithSmallBuffers() *httptest.Server {
	e.T.Helper()
	srv := httptest.NewUnstartedServer(e.Handler)
	srv.Listener = smallBufferListener{srv.Listener}
	srv.Start()
	e.T.Cleanup(srv.Close)
	return srv
}

func (e *Env) ServeWithTimeouts(read, write time.Duration) *httptest.Server {
	e.T.Helper()
	srv := httptest.NewUnstartedServer(e.Handler)
	srv.Config.ReadTimeout, srv.Config.WriteTimeout = read, write
	srv.Start()
	e.T.Cleanup(srv.Close)
	return srv
}

type Event struct {
	Name string
	ID   string
	Data string
}

type Stream struct {
	Status int
	Header http.Header
	Body   string
	events chan Event
	closed chan struct{}
	cancel context.CancelFunc
}

func LogStreamPath(org, job uuid.UUID) string {
	return config.APIPrefix + "/organizations/" + org.String() + "/jobs/" + job.String() + "/logs/stream"
}

func (c *Client) OpenStream(t testing.TB, srv *httptest.Server, path string, headers map[string]string) *Stream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")
	if c.Cookie != "" {
		req.Header.Set("Cookie", config.SessionCookieName+"="+c.Cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	s := &Stream{Status: res.StatusCode, Header: res.Header, events: make(chan Event, streamEventBuffer), closed: make(chan struct{}), cancel: cancel}
	t.Cleanup(s.Close)
	if res.StatusCode != http.StatusOK {
		buf := make([]byte, 4096)
		n, _ := res.Body.Read(buf)
		s.Body = string(buf[:n])
		_ = res.Body.Close()
		close(s.closed)
		return s
	}
	go s.read(res)
	return s
}

func (s *Stream) read(res *http.Response) {
	defer close(s.closed)
	defer func() { _ = res.Body.Close() }()
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 1<<16), 1<<22)
	var ev Event
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if ev.Name != "" {
				s.events <- ev
			}
			ev = Event{}
		case strings.HasPrefix(line, "event: "):
			ev.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "id: "):
			ev.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			ev.Data = strings.TrimPrefix(line, "data: ")
		case strings.HasPrefix(line, ":"):
			ev.Name = "comment"
		}
	}
}

func (s *Stream) Next(timeout time.Duration) (Event, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case ev := <-s.events:
		return ev, nil
	case <-s.closed:
		select {
		case ev := <-s.events:
			return ev, nil
		default:
			return Event{}, ErrStreamClosed
		}
	case <-timer.C:
		return Event{}, ErrStreamTimeout
	}
}

func (s *Stream) WaitClosed(timeout time.Duration) bool {
	select {
	case <-s.closed:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *Stream) Close() { s.cancel() }

func (c *Client) StallingStream(t testing.TB, srv *httptest.Server, path string) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", srv.Listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	tc, ok := conn.(*net.TCPConn)
	require.True(t, ok)
	raw, err := tc.SyscallConn()
	require.NoError(t, err)
	var serr error
	require.NoError(t, raw.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, smallSocketBuffer)
	}))
	require.NoError(t, serr)
	request := "GET " + path + " HTTP/1.1\r\nHost: dhow.example.com\r\nAccept: text/event-stream\r\n" +
		"Cookie: " + config.SessionCookieName + "=" + c.Cookie + "\r\n\r\n"
	_, err = conn.Write([]byte(request))
	require.NoError(t, err)
	return conn
}
