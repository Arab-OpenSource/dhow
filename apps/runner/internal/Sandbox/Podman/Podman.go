package podman

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	controlDirName    = "control"
	controlSocketName = "control.sock"
	workspaceDirName  = "workspace"
)

type Driver struct {
	binary      string
	attemptsDir string
	agentPath   string
	paths       paths
	levels      []runnerv1.Isolation
	skipPull    bool
	log         *slog.Logger
}

type Options struct {
	Config     config.Config
	Log        *slog.Logger
	Binary     string
	AgentPath  string
	Levels     []runnerv1.Isolation
	SkipChecks bool
	SkipPull   bool
}

func New(ctx context.Context, o Options) (*Driver, error) {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	binary := o.Binary
	if binary == "" {
		resolved, err := exec.LookPath("podman")
		if err != nil {
			return nil, fmt.Errorf("find podman on PATH: %w", err)
		}
		binary = resolved
	}
	agentPath := o.AgentPath
	if agentPath == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("find the runner binary to mount as the agent: %w", err)
		}
		agentPath = self
	}
	if !o.SkipChecks {
		if bad := failed(Preflight(ctx, binary)); len(bad) > 0 {
			return nil, &PreflightError{Failed: bad}
		}
	}
	p, err := writeConfig(o.Config.StateDir)
	if err != nil {
		return nil, err
	}
	levels := o.Levels
	if len(levels) == 0 {
		levels = []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_CONTAINER}
	}
	return &Driver{binary: binary, attemptsDir: o.Config.AttemptsDir(), agentPath: agentPath, paths: p, levels: levels, skipPull: o.SkipPull, log: o.Log}, nil
}

func (d *Driver) Name() string { return "podman" }

func (d *Driver) Isolation() []runnerv1.Isolation { return d.levels }

func (d *Driver) runner() runner { return d.paths.env(d.binary) }

func (d *Driver) Podman(ctx context.Context, args ...string) ([]byte, error) {
	return d.runner().run(ctx, args...)
}

func (d *Driver) Cleanup(ctx context.Context, runnerID string) error {
	r := d.runner()
	out, err := r.run(ctx, "ps", "--all", "--filter", "label="+sandbox.LabelRunner+"="+runnerID, "--format", "{{.ID}}")
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(string(out)) {
		if _, err := r.run(ctx, "rm", "--force", "--volumes", "--", id); err != nil {
			d.log.Warn("could not remove a sandbox left by an earlier run", "error", err.Error())
		}
	}
	return d.removeOwned(ctx, d.attemptsDir)
}

func (d *Driver) removeOwned(ctx context.Context, dir string) error {
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := d.runner().run(ctx, "unshare", "rm", "-rf", "--", dir); err != nil {
		return fmt.Errorf("remove %s, which job user namespaces own parts of: %w", dir, err)
	}
	return nil
}

func (d *Driver) Prepare(ctx context.Context, spec sandbox.PrepareSpec) (sandbox.Sandbox, error) {
	r := d.runner()
	l := layout{
		name:       "dhow-" + spec.AttemptID,
		agentPath:  d.agentPath,
		controlDir: filepath.Join(spec.Dir, controlDirName),
		workDir:    filepath.Join(spec.Dir, workspaceDirName),
	}
	if err := config.EnsurePrivateDir(l.controlDir); err != nil {
		return nil, err
	}
	if spec.DiskMode == config.DiskModeDirectory {
		if err := config.EnsurePrivateDir(l.workDir); err != nil {
			return nil, err
		}
	}
	if err := d.pull(ctx, spec.Image); err != nil {
		return nil, fmt.Errorf("%w: %w", sandbox.ErrImagePull, err)
	}
	if _, err := r.run(ctx, createArgs(spec, l, d.paths.seccompFile)...); err != nil {
		return nil, fmt.Errorf("%w: %w", sandbox.ErrSandbox, err)
	}
	in, err := inspect(ctx, r, l.name)
	if err != nil {
		d.remove(ctx, l.name)
		return nil, fmt.Errorf("%w: %w", sandbox.ErrSandbox, err)
	}
	if err := verify(in, spec); err != nil {
		d.remove(ctx, l.name)
		return nil, fmt.Errorf("%w: %w", sandbox.ErrSandbox, err)
	}
	box, err := newBox(ctx, d, l, spec)
	if err != nil {
		d.remove(ctx, l.name)
		return nil, fmt.Errorf("%w: %w", sandbox.ErrSandbox, err)
	}
	return box, nil
}

func (d *Driver) pull(ctx context.Context, image string) error {
	r := d.runner()
	if d.skipPull {
		_, err := r.run(ctx, "image", "exists", "--", image)
		return err
	}
	_, err := r.run(ctx, "pull", "--", image)
	return err
}

func (d *Driver) remove(ctx context.Context, name string) {
	if _, err := d.runner().run(ctx, "rm", "--force", "--volumes", "--", name); err != nil {
		d.log.Warn("could not remove a half-created sandbox", "error", err.Error())
	}
}
