package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	logLines     = 200
	logLineDelay = 25
	stopTimeout  = 30 * time.Second
)

type fakeOutput struct {
	Stream      string
	Data        string
	DelayMillis int
}

type fakeStep struct {
	Output   []fakeOutput
	ExitCode int32
}

type Runners struct {
	Binary string
	Dir    string
	URL    string
	count  int
	procs  []*exec.Cmd
	done   []chan error
}

func (r *Runners) Enabled() bool { return r.Binary != "" }

func (r *Runners) recording() string { return filepath.Join(r.Dir, "recording.json") }

func (r *Runners) stateDir(i int) string { return filepath.Join(r.Dir, "runner-"+strconv.Itoa(i)) }

func (r *Runners) env(i, capacity int) []string {
	return []string{
		"DHOW_RUNNER_STATE_DIR=" + r.stateDir(i),
		"DHOW_RUNNER_DRIVER=fake",
		"DHOW_RUNNER_FAKE_RECORDING=" + r.recording(),
		"DHOW_RUNNER_SERVER_ORIGIN=" + publicURL,
		"DHOW_RUNNER_CAPACITY=" + strconv.Itoa(capacity),
		"DHOW_RUNNER_LOG_LEVEL=warn",
	}
}

func writeRecording(path string) error {
	step := fakeStep{}
	for i := range logLines {
		step.Output = append(step.Output, fakeOutput{
			Stream:      "stdout",
			Data:        fmt.Sprintf("bench %03d: compiling package %03d of %d, nothing to see here, just a typical build line\n", i, i, logLines),
			DelayMillis: logLineDelay,
		})
	}
	raw, err := json.Marshal(map[string][]fakeStep{"Steps": {step}})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func (r *Runners) Register(ctx context.Context, api *API, owner *Session, org string, count int) error {
	if !r.Enabled() {
		return nil
	}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return err
	}
	if err := writeRecording(r.recording()); err != nil {
		return err
	}
	for i := range count {
		if err := os.RemoveAll(r.stateDir(i)); err != nil {
			return err
		}
		if err := os.Mkdir(r.stateDir(i), 0o700); err != nil {
			return err
		}
		token, err := api.RegistrationToken(ctx, owner, org)
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, r.Binary, "register", "--url", r.URL, "--token-file", "-", "--name", "bench-"+strconv.Itoa(i), "--dev") //nolint:gosec // The runner binary the bench built, with a fixed argv.
		cmd.Env = append(cmd.Environ(), r.env(i, 1)...)
		cmd.Stdin = strings.NewReader(token)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("register runner %d: %w: %s", i, err, strings.TrimSpace(stderr.String()))
		}
	}
	r.count = count
	return nil
}

func (r *Runners) Start(capacities []int) error {
	r.Stop()
	if !r.Enabled() {
		return nil
	}
	if len(capacities) > r.count {
		return fmt.Errorf("%d runners requested, %d registered", len(capacities), r.count)
	}
	for i, capacity := range capacities {
		logFile, err := os.OpenFile(filepath.Join(r.Dir, "runner-"+strconv.Itoa(i)+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		cmd := exec.Command(r.Binary, "run", "--dev") //nolint:gosec,noctx // The runner binary the bench built; it is stopped with SIGTERM, not a context.
		cmd.Env = append(cmd.Environ(), r.env(i, capacity)...)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			return err
		}
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
			_ = logFile.Close()
		}()
		r.procs = append(r.procs, cmd)
		r.done = append(r.done, done)
	}
	return nil
}

func (r *Runners) Stop() {
	for i, cmd := range r.procs {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-time.After(stopTimeout):
			_ = cmd.Process.Kill()
			r.log(i, <-r.done[i])
		case err := <-r.done[i]:
			r.log(i, err)
		}
	}
	r.procs, r.done = nil, nil
}

func (r *Runners) log(i int, err error) {
	if err == nil {
		return
	}
	f, ferr := os.OpenFile(filepath.Join(r.Dir, "runner-"+strconv.Itoa(i)+".log"), os.O_APPEND|os.O_WRONLY, 0o600)
	if ferr != nil {
		return
	}
	_, _ = f.WriteString("bench: the runner stopped with " + err.Error() + "\n")
	_ = f.Close()
}

func (r *Runners) Alive() error {
	for i := range r.procs {
		select {
		case err := <-r.done[i]:
			r.done[i] <- err
			return fmt.Errorf("runner %d exited: %w (see %s)", i, err, filepath.Join(r.Dir, "runner-"+strconv.Itoa(i)+".log"))
		default:
		}
	}
	return nil
}

func (r *Runners) Memory() (rss, peak uint64) {
	for _, cmd := range r.procs {
		cur, hwm, err := procMemory(cmd.Process.Pid)
		if err != nil {
			continue
		}
		rss += cur
		peak += hwm
	}
	return rss, peak
}

func procMemory(pid int) (rss, hwm uint64, err error) {
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		kb, perr := strconv.ParseUint(fields[0], 10, 64)
		if perr != nil {
			continue
		}
		switch key {
		case "VmRSS":
			rss = kb << 10
		case "VmHWM":
			hwm = kb << 10
		}
	}
	if rss == 0 {
		return 0, 0, errors.New("no VmRSS for the runner")
	}
	return rss, hwm, sc.Err()
}
