package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const mib = 1 << 20

type Usage struct {
	AvgMiB     float64
	PeakMiB    float64
	PeakRSSMiB float64
	AvgCPU     float64
	PeakCPU    float64
}

type PhaseResult struct {
	Phase         string
	OK            bool
	Problem       string
	Users         int
	JobWorkers    int
	Runners       []int
	Server        Usage
	DB            Usage
	RunnerAvgMiB  float64
	RunnerPeakMiB float64
	Requests      int
	Errors        int
	ErrorKinds    map[string]int
	P50Ms         float64
	P95Ms         float64
	P99Ms         float64
	Jobs          int
	OOMEvents     int
	DataBytes     uint64
	DatabaseBytes uint64
	SamplerErrors []string
}

type ConfigResult struct {
	Limits  Limits
	Problem string
	Phases  []PhaseResult
}

type Report struct {
	Date               string
	ServerImage        string
	PostgresImage      string
	HostCPUs           int
	ServerImageBytes   uint64
	PostgresImageBytes uint64
	CPUsPerContainer   string
	Configs            []ConfigResult
}

func usageOf(samples []Sample) Usage {
	var u Usage
	if len(samples) == 0 {
		return u
	}
	var sum uint64
	for _, s := range samples {
		sum += s.WorkingSet
		u.PeakMiB = max(u.PeakMiB, float64(s.WorkingSet)/mib)
		u.PeakRSSMiB = max(u.PeakRSSMiB, float64(s.RSS)/mib)
	}
	u.AvgMiB = float64(sum) / float64(len(samples)) / mib
	first, last := samples[0], samples[len(samples)-1]
	if wall := last.At.Sub(first.At); wall > 0 && last.CPUNanos >= first.CPUNanos {
		u.AvgCPU = float64(last.CPUNanos-first.CPUNanos) / float64(wall.Nanoseconds())
	}
	for i := 1; i < len(samples); i++ {
		a, b := samples[i-1], samples[i]
		if wall := b.At.Sub(a.At); wall > 0 && b.CPUNanos >= a.CPUNanos {
			u.PeakCPU = max(u.PeakCPU, float64(b.CPUNanos-a.CPUNanos)/float64(wall.Nanoseconds()))
		}
	}
	return u
}

func (res *PhaseResult) fill(sm *sampler, rec *Recorder) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	res.Server = usageOf(sm.server)
	res.DB = usageOf(sm.db)
	if len(sm.runner) > 0 {
		var sum uint64
		for _, v := range sm.runner {
			sum += v
		}
		res.RunnerAvgMiB = float64(sum) / float64(len(sm.runner)) / mib
		res.RunnerPeakMiB = float64(slices.Max(sm.runner)) / mib
	}
	if len(sm.errs) > 0 {
		res.SamplerErrors = sm.errs[:min(len(sm.errs), errorKinds)]
	}
	res.Requests, res.Errors, res.ErrorKinds, res.P50Ms, res.P95Ms, res.P99Ms = rec.Summary()
}

func (r *Report) Write(path string) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) }

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func (r *Report) Table() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Dhow resource benchmark, %s, server image %s, %s CPUs per container\n\n", r.Date, r.ServerImage, r.CPUsPerContainer)
	fmt.Fprintf(&b, "Images on disk: server %s MiB, PostgreSQL %s MiB\n\n", f1(float64(r.ServerImageBytes)/mib), f1(float64(r.PostgresImageBytes)/mib))
	b.WriteString("| Limits (server / postgres) | Phase | Result | Server MiB avg / peak | Postgres MiB avg / peak | Server CPU avg / peak | Postgres CPU avg / peak | Runner MiB peak | Requests | Errors | p95 ms | Jobs | Data MiB |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Configs {
		limits := c.Limits.Server + " / " + c.Limits.DB
		if c.Problem != "" {
			fmt.Fprintf(&b, "| %s | setup | FAIL: %s |  |  |  |  |  |  |  |  |  |  |\n", limits, c.Problem)
			continue
		}
		for _, p := range c.Phases {
			result := "ok"
			if !p.OK {
				result = "FAIL: " + p.Problem
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s / %s | %s / %s | %s / %s | %s / %s | %s | %d | %d | %s | %d | %s |\n",
				limits, p.Phase, result, f1(p.Server.AvgMiB), f1(p.Server.PeakMiB), f1(p.DB.AvgMiB), f1(p.DB.PeakMiB),
				f2(p.Server.AvgCPU), f2(p.Server.PeakCPU), f2(p.DB.AvgCPU), f2(p.DB.PeakCPU), f1(p.RunnerPeakMiB),
				p.Requests, p.Errors, f1(p.P95Ms), p.Jobs, f1(float64(p.DataBytes)/mib))
		}
	}
	return b.String()
}

type BaselinePhase struct {
	ServerPeakMiB float64
	DBPeakMiB     float64
}

type Baseline struct {
	Tolerance float64
	Phases    map[string]BaselinePhase
}

func (r *Report) Compare(path string) ([]string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // The baseline file named on the command line.
	if err != nil {
		return nil, err
	}
	var base Baseline
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(r.Configs) == 0 {
		return []string{"no configuration ran"}, nil
	}
	ref := r.Configs[0]
	if ref.Problem != "" {
		return []string{"the reference configuration failed: " + ref.Problem}, nil
	}
	got := map[string]PhaseResult{}
	for _, p := range ref.Phases {
		got[p.Phase] = p
	}
	var out []string
	for name, want := range base.Phases {
		p, ok := got[name]
		switch {
		case !ok:
			out = append(out, name+": did not run")
		case !p.OK:
			out = append(out, name+": "+p.Problem)
		default:
			out = append(out, over(name+" server", p.Server.PeakMiB, want.ServerPeakMiB, base.Tolerance)...)
			out = append(out, over(name+" postgres", p.DB.PeakMiB, want.DBPeakMiB, base.Tolerance)...)
		}
	}
	slices.Sort(out)
	return out, nil
}

func over(what string, got, want, tolerance float64) []string {
	if want <= 0 || got <= want*(1+tolerance) {
		return nil
	}
	return []string{fmt.Sprintf("%s peak memory %s MiB is more than %s%% over the published %s MiB", what, f1(got), f1(tolerance*100), f1(want))}
}

func today() string { return time.Now().UTC().Format(time.DateOnly) }
