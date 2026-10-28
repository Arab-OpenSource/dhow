package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	benchImage   = "docker.io/library/busybox:1.37@sha256:0000000000000000000000000000000000000000000000000000000000000001"
	jobPoll      = time.Second
	jobDeadline  = 2 * time.Minute
	sampleEvery  = time.Second
	settleBefore = 15 * time.Second
)

type Phase struct {
	Name       string
	Duration   time.Duration
	Users      int
	Think      time.Duration
	JobWorkers int
	Runners    []int
}

type World struct {
	API      *API
	Org      string
	Owner    Session
	Members  []string
	Password string
	Admins   int
	Runners  *Runners
	Stack    *Stack
}

var terminal = map[string]bool{"succeeded": true, "failed": true, "canceled": true, "timed_out": true, "lost": true}

func clientIP(i int) string { return fmt.Sprintf("198.18.%d.%d", i/250, i%250+1) }

func (w *World) browse(ctx context.Context, api *API, member int, think time.Duration) {
	session, err := api.SignIn(ctx, w.Members[member], w.Password, clientIP(member))
	if err != nil {
		return
	}
	s := &session
	org := "/api/v1/organizations/" + w.Org
	pages := []struct{ path, route string }{
		{"/api/v1/me", "/me"},
		{org + "/runners", "/organizations/{id}/runners"},
		{org, "/organizations/{id}"},
		{org + "/members", "/organizations/{id}/members"},
		{org + "/jobs?limit=20", "/organizations/{id}/jobs"},
		{org + "/teams", "/organizations/{id}/teams"},
		{"/api/v1/me/preferences", "/me/preferences"},
		{org + "/jobs?limit=50&state=succeeded", "/organizations/{id}/jobs"},
		{"/api/v1/me/passkeys", "/me/passkeys"},
	}
	for i := 0; ctx.Err() == nil; i++ {
		p := pages[i%len(pages)]
		_, _ = api.Do(ctx, http.MethodGet, p.path, p.route, nil, s, nil)
		if sleep(ctx, think) != nil {
			return
		}
	}
}

type jobDetail struct {
	Job struct {
		ID    string
		State string
	}
}

func (w *World) jobCycle(ctx context.Context, api *API, s *Session, n int) {
	jobs := "/api/v1/organizations/" + w.Org + "/jobs"
	var created jobDetail
	_, err := api.Do(ctx, http.MethodPost, jobs, "/organizations/{id}/jobs", map[string]any{
		"Name": fmt.Sprintf("bench-%d", n),
		"Spec": map[string]any{
			"Image": benchImage,
			"Steps": []map[string]string{{"Name": "build", "Run": "make build"}},
		},
	}, s, &created)
	if err != nil {
		_ = sleep(ctx, jobPoll)
		return
	}
	one := jobs + "/" + created.Job.ID
	if !w.Runners.Enabled() {
		_, _ = api.Do(ctx, http.MethodGet, one, "/organizations/{id}/jobs/{jobId}", nil, s, nil)
		_, _ = api.Do(ctx, http.MethodPost, one+"/cancel", "/organizations/{id}/jobs/{jobId}/cancel", nil, s, nil)
		_, _ = api.Do(ctx, http.MethodGet, jobs+"?limit=20", "/organizations/{id}/jobs", nil, s, nil)
		_ = sleep(ctx, jobPoll)
		return
	}
	deadline := time.Now().Add(jobDeadline)
	state := ""
	for ctx.Err() == nil && !terminal[state] && time.Now().Before(deadline) {
		if sleep(ctx, jobPoll) != nil {
			return
		}
		var d jobDetail
		if _, err := api.Do(ctx, http.MethodGet, one, "/organizations/{id}/jobs/{jobId}", nil, s, &d); err == nil {
			state = d.Job.State
		}
	}
	if ctx.Err() != nil {
		return
	}
	switch state {
	case "succeeded":
	case "":
		api.Rec.Fail("job did not finish in " + jobDeadline.String())
	default:
		api.Rec.Fail("job ended " + state)
	}
	_, _ = api.Do(ctx, http.MethodGet, one+"/logs?limit=500", "/organizations/{id}/jobs/{jobId}/logs", nil, s, nil)
	_, _ = api.Do(ctx, http.MethodGet, jobs+"?limit=20", "/organizations/{id}/jobs", nil, s, nil)
}

func (w *World) jobWorker(ctx context.Context, api *API, member int) int {
	session, err := api.SignIn(ctx, w.Members[member], w.Password, clientIP(member))
	if err != nil {
		return 0
	}
	s := &session
	n := 0
	for ctx.Err() == nil {
		w.jobCycle(ctx, api, s, n)
		n++
	}
	return n
}

type sampler struct {
	mu      sync.Mutex
	server  []Sample
	db      []Sample
	runner  []uint64
	peakRun uint64
	errs    []string
}

func (sm *sampler) run(ctx context.Context, w *World) {
	t := time.NewTicker(sampleEvery)
	defer t.Stop()
	for {
		srv, err1 := w.Stack.Sample(ctx, "dhow")
		db, err2 := w.Stack.Sample(ctx, "postgres")
		rss, peak := w.Runners.Memory()
		sm.mu.Lock()
		if err := errors.Join(err1, err2); err != nil && ctx.Err() == nil {
			sm.errs = append(sm.errs, err.Error())
		} else if err == nil {
			sm.server = append(sm.server, srv)
			sm.db = append(sm.db, db)
		}
		if rss > 0 {
			sm.runner = append(sm.runner, rss)
			sm.peakRun = max(sm.peakRun, peak)
		}
		sm.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *World) Run(ctx context.Context, p Phase) PhaseResult {
	res := PhaseResult{Phase: p.Name, Users: p.Users, JobWorkers: p.JobWorkers, Runners: p.Runners}
	if err := w.Runners.Start(p.Runners); err != nil {
		res.Problem = "start runners: " + err.Error()
		return res
	}
	rec := NewRecorder()
	api := &API{Base: w.API.Base, HC: w.API.HC, Rec: rec}
	if err := sleep(ctx, settleBefore); err != nil {
		res.Problem = err.Error()
		return res
	}
	loadCtx, stop := context.WithTimeout(ctx, p.Duration)
	defer stop()
	var sm sampler
	var wg sync.WaitGroup
	wg.Go(func() { sm.run(loadCtx, w) })
	var jobsMu sync.Mutex
	for i := range p.Users {
		member := w.Admins + i%(len(w.Members)-w.Admins)
		wg.Go(func() { w.browse(loadCtx, api, member, p.Think) })
	}
	for i := range p.JobWorkers {
		member := i % w.Admins
		wg.Go(func() {
			n := w.jobWorker(loadCtx, api, member)
			jobsMu.Lock()
			res.Jobs += n
			jobsMu.Unlock()
		})
	}
	wg.Wait()
	res.fill(&sm, rec)
	res.check(ctx, w)
	return res
}

func (res *PhaseResult) check(ctx context.Context, w *World) {
	var problems []string
	if err := w.Runners.Alive(); err != nil {
		problems = append(problems, err.Error())
	}
	if health, err := w.Stack.Health(ctx); err != nil {
		problems = append(problems, "inspect: "+err.Error())
	} else if health != "" {
		problems = append(problems, health)
	}
	if ooms, err := w.Stack.OOMEvents(ctx); err == nil {
		for svc, n := range ooms {
			res.OOMEvents += n
			problems = append(problems, fmt.Sprintf("%s had %d out-of-memory events", svc, n))
		}
	}
	if res.Errors > 0 {
		problems = append(problems, fmt.Sprintf("%d of %d requests failed", res.Errors, res.Requests))
	}
	if db, err := w.Stack.DatabaseBytes(ctx); err == nil {
		res.DatabaseBytes = db
	}
	if data, err := w.Stack.DataBytes(ctx); err == nil {
		res.DataBytes = data
	} else {
		problems = append(problems, "measure the database volume: "+err.Error())
	}
	res.Problem = strings.Join(problems, "; ")
	res.OK = res.Problem == ""
}
