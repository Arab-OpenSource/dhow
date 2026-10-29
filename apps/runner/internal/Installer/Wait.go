package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	onlineTimeout  = 2 * time.Minute
	onlineInterval = time.Second
	tailLines      = "25"
)

func invocation(ctx context.Context, sys engine.System) string {
	out, err := sys.Run(ctx, engine.Cmd("systemctl", "show", "--property", "InvocationID", "--value", ServiceName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out.Stdout))
}

func currentLog(ctx context.Context, sys engine.System) ([]byte, bool) {
	id := invocation(ctx, sys)
	if id == "" {
		return nil, false
	}
	out, err := sys.Run(ctx, engine.Cmd("journalctl", "_SYSTEMD_INVOCATION_ID="+id, "--output", "cat", "--no-pager"))
	if err != nil {
		return nil, false
	}
	return out.Stdout, true
}

func runnerReady(ctx context.Context, sys engine.System) (bool, string) {
	logs, ok := currentLog(ctx, sys)
	if !ok {
		return false, ""
	}
	ready, last := false, ""
	for _, raw := range bytes.Split(logs, []byte("\n")) {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var l map[string]any
		if err := json.Unmarshal(raw, &l); err != nil {
			last = string(raw)
			continue
		}
		msg, level, reason := field(l, "msg"), field(l, "level"), field(l, "error")
		if msg == ReadyMessage {
			ready = true
		}
		if level == "WARN" || level == "ERROR" {
			last = strings.TrimSpace(msg + " " + reason)
		}
	}
	return ready, last
}

func field(l map[string]any, key string) string {
	v, _ := l[key].(string)
	return v
}

func unitState(ctx context.Context, sys engine.System) (string, string) {
	out, err := sys.Run(ctx, engine.Cmd("systemctl", "show", "--property", "ActiveState", "--property", "SubState", "--value", ServiceName))
	if err != nil {
		return "", ""
	}
	lines := strings.Split(strings.TrimSpace(string(out.Stdout)), "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(lines[0]), ""
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
}

func waitOnline(ctx context.Context, sys engine.System, m engine.Messages, sleep func(time.Duration)) error {
	deadline := sys.Now().Add(onlineTimeout)
	for {
		ready, last := runnerReady(ctx, sys)
		if ready {
			return nil
		}
		active, sub := unitState(ctx, sys)
		failed := active == "failed" || sub == "auto-restart"
		if failed || !sys.Now().Before(deadline) {
			reason := m.T(KeyNotOnline)
			if last != "" {
				reason += ": " + last
			}
			return errors.Join(errors.New(reason), journalTail(ctx, sys, m))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		sleep(onlineInterval)
	}
}

func journalTail(ctx context.Context, sys engine.System, m engine.Messages) error {
	out, err := sys.Run(ctx, engine.Cmd("journalctl", "--unit", ServiceName, "--lines", tailLines, "--no-pager", "--output", "cat"))
	if err != nil || len(bytes.TrimSpace(out.Stdout)) == 0 {
		return nil
	}
	return errors.New(m.T(KeyJournalTail) + "\n" + string(bytes.TrimSpace(out.Stdout)))
}
