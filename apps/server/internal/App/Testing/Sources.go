package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/stretchr/testify/require"

	triggers "github.com/Khaymah/dhow/apps/server/internal/Core/Triggers"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

type FakeTriggers struct {
	mu       sync.Mutex
	received []triggers.Trigger
	err      error
}

func (f *FakeTriggers) Deliver(_ context.Context, _ *sqlc.Queries, t triggers.Trigger) (triggers.Accepted, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return triggers.Accepted{}, f.err
	}
	f.received = append(f.received, t)
	return triggers.Accepted{Created: true}, nil
}

func (f *FakeTriggers) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *FakeTriggers) Received() []triggers.Trigger {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]triggers.Trigger(nil), f.received...)
}

func (e *Env) Webhook(method, path string, headers map[string]string, body []byte) Response {
	e.T.Helper()
	r := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
	r.RemoteAddr = "198.51.100.20:4444"
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.Handler.ServeHTTP(rec, r)
	res := Response{Status: rec.Code, Header: rec.Header(), Raw: rec.Body.Bytes()}
	if len(res.Raw) > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		require.NoError(e.T, json.Unmarshal(res.Raw, &res), "response body: %s", res.Raw)
	}
	return res
}

func (e *Env) CountRows(query string, args ...any) int {
	e.T.Helper()
	var n int
	require.NoError(e.T, e.Pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}
