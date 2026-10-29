package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie  = "__Host-dhow_session"
	requestTimeout = 30 * time.Second
	errorKinds     = 8
)

type Session struct {
	Cookie string
	CSRF   string
	IP     string
}

type Recorder struct {
	mu       sync.Mutex
	latency  []time.Duration
	errors   int
	kinds    map[string]int
	requests int
}

func NewRecorder() *Recorder { return &Recorder{kinds: map[string]int{}} }

func (r *Recorder) add(d time.Duration, kind string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests++
	r.latency = append(r.latency, d)
	if kind != "" {
		r.failLocked(kind)
	}
}

func (r *Recorder) Fail(kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failLocked(kind)
}

func (r *Recorder) failLocked(kind string) {
	r.errors++
	if _, ok := r.kinds[kind]; ok || len(r.kinds) < errorKinds {
		r.kinds[kind]++
	}
}

func (r *Recorder) Summary() (requests, errs int, kinds map[string]int, p50, p95, p99 float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lat := slices.Clone(r.latency)
	slices.Sort(lat)
	pct := func(p float64) float64 {
		if len(lat) == 0 {
			return 0
		}
		i := min(int(p*float64(len(lat))), len(lat)-1)
		return float64(lat[i].Microseconds()) / 1000
	}
	kinds = map[string]int{}
	for k, v := range r.kinds {
		kinds[k] = v
	}
	return r.requests, r.errors, kinds, pct(0.50), pct(0.95), pct(0.99)
}

type API struct {
	Base string
	HC   *http.Client
	Rec  *Recorder
}

type envelope struct {
	Data  json.RawMessage
	Error *struct{ Code string }
}

func (a *API) Do(ctx context.Context, method, path, route string, body any, s *Session, into any) (http.Header, error) {
	var payload io.Reader = http.NoBody
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, a.Base+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Language", "en")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Origin", publicURL)
	}
	if s != nil && s.IP != "" {
		req.Header.Set("X-Forwarded-For", s.IP)
	}
	if s != nil && s.Cookie != "" {
		req.Header.Set("Cookie", sessionCookie+"="+s.Cookie)
		if method != http.MethodGet {
			req.Header.Set("X-CSRF-Token", s.CSRF)
		}
	}
	start := time.Now()
	res, err := a.HC.Do(req)
	if err != nil {
		if parent.Err() != nil {
			return nil, err
		}
		a.Rec.add(time.Since(start), method+" "+route+": "+transportKind(err))
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	took := time.Since(start)
	if err != nil && parent.Err() != nil {
		return nil, err
	}
	if err != nil {
		a.Rec.add(took, method+" "+route+": read body")
		return nil, err
	}
	var env envelope
	_ = json.Unmarshal(raw, &env)
	if res.StatusCode >= http.StatusBadRequest {
		code := ""
		if env.Error != nil {
			code = " " + env.Error.Code
		}
		kind := fmt.Sprintf("%s %s: %d%s", method, route, res.StatusCode, code)
		a.Rec.add(took, kind)
		return res.Header, errors.New(kind)
	}
	a.Rec.add(took, "")
	if into != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, into); err != nil {
			return res.Header, fmt.Errorf("%s %s: decode: %w", method, route, err)
		}
	}
	return res.Header, nil
}

func transportKind(err error) string {
	msg := err.Error()
	for _, k := range []string{"connection refused", "connection reset", "EOF", "deadline exceeded", "timeout"} {
		if strings.Contains(msg, k) {
			return k
		}
	}
	return "transport error"
}

func (a *API) Setup(ctx context.Context, token, email, password string) (string, error) {
	var out struct{ Organization struct{ ID string } }
	_, err := a.Do(ctx, http.MethodPost, "/api/v1/setup", "/setup", map[string]string{
		"SetupToken": token, "OwnerName": "Bench Owner", "Email": email, "Password": password,
		"OrganizationName": "Bench", "OrganizationSlug": "bench",
	}, nil, &out)
	if err == nil && out.Organization.ID == "" {
		err = errors.New("setup answered without an organization id")
	}
	return out.Organization.ID, err
}

func (a *API) SignIn(ctx context.Context, email, password, ip string) (Session, error) {
	var out struct{ CsrfToken string }
	header, err := a.Do(ctx, http.MethodPost, "/api/v1/sessions", "/sessions", map[string]string{
		"Email": email, "Password": password,
	}, &Session{IP: ip}, &out)
	if err != nil {
		return Session{}, err
	}
	resp := http.Response{Header: header}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			return Session{Cookie: c.Value, CSRF: out.CsrfToken, IP: ip}, nil
		}
	}
	return Session{}, errors.New("sign-in set no session cookie")
}

func (a *API) AddMember(ctx context.Context, owner *Session, org, email, role, password string) error {
	var inv struct{ Link string }
	path := "/api/v1/organizations/" + org + "/invitations"
	if _, err := a.Do(ctx, http.MethodPost, path, "/organizations/{id}/invitations", map[string]string{
		"Email": email, "Role": role,
	}, owner, &inv); err != nil {
		return err
	}
	_, token, ok := strings.Cut(inv.Link, "#")
	if !ok {
		return errors.New("the invitation has no link token")
	}
	_, err := a.Do(ctx, http.MethodPost, "/api/v1/invitations/accept", "/invitations/accept", map[string]string{
		"Token": token, "Name": "Bench Member", "Password": password, "Locale": "en",
	}, nil, nil)
	return err
}

func (a *API) RegistrationToken(ctx context.Context, owner *Session, org string) (string, error) {
	var out struct{ Token string }
	path := "/api/v1/organizations/" + org + "/runner-registration-tokens"
	_, err := a.Do(ctx, http.MethodPost, path, "/organizations/{id}/runner-registration-tokens", map[string]any{
		"Labels": []string{"pool=bench"},
	}, owner, &out)
	return out.Token, err
}
