package installer

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"

	engine "github.com/Khaymah/dhow/internal/installer"
)

var (
	errPodmanMissing = errors.New("podman is not installed")
	errNoToken       = errors.New("the runner is not registered yet and no registration token was given")
)

func (b *Builder) packageSteps() []engine.Step {
	missing := b.MissingPackages()
	if len(missing) == 0 {
		return nil
	}
	install := engine.InstallStep(b.Sys, b.Det.Distro.Family, b.M.T(KeyStepPackages), missing)
	check := engine.Step{
		Title: b.M.T(KeyStepPodmanCheck, MinPodman),
		Apply: func(ctx context.Context) error {
			have := podmanVersion(ctx, b.Sys)
			if have == "" {
				return errPodmanMissing
			}
			if !PodmanAtLeast(have, MinPodman) {
				return errors.New(b.M.T(KeyBlockPodmanOld, have, MinPodman))
			}
			return nil
		},
	}
	return []engine.Step{install, check}
}

func (b *Builder) userStep() engine.Step {
	add := engine.Cmd("useradd", "--system", "--user-group", "--no-create-home", "--home-dir", StateDir,
		"--shell", b.Det.Distro.NologinShell(), "--comment", UserComment, ServiceUser)
	check := engine.Cmd("getent", "passwd", ServiceUser)
	return engine.CommandStep(b.Sys, b.M.T(KeyStepUser, ServiceUser), &check, []engine.Command{add},
		[]engine.Command{engine.Cmd("userdel", ServiceUser)})
}

func (b *Builder) subIDStep() engine.Step {
	need := b.A.Capacity * SubIDsPerJob
	start := FreeSubIDStart(b.existing(SubUIDPath), b.existing(SubGIDPath))
	span := fmt.Sprintf("%d-%d", start, start+SubIDRange-1)
	add := engine.Cmd("usermod", "--add-subuids", span, "--add-subgids", span, ServiceUser)
	return engine.Step{
		Title:    b.M.T(KeyStepSubIDs, SubIDRange),
		Files:    []string{SubUIDPath, SubGIDPath},
		Commands: []string{add.String()},
		Done: func(context.Context) (bool, error) {
			return subIDCount(b.Sys, SubUIDPath) >= need && subIDCount(b.Sys, SubGIDPath) >= need, nil
		},
		Apply: func(ctx context.Context) error {
			start := FreeSubIDStart(b.existing(SubUIDPath), b.existing(SubGIDPath))
			span := fmt.Sprintf("%d-%d", start, start+SubIDRange-1)
			_, err := engine.Run(ctx, b.Sys, engine.Cmd("usermod", "--add-subuids", span, "--add-subgids", span, ServiceUser))
			return err
		},
	}
}

func (b *Builder) binaryStep() engine.Step {
	var backup []byte
	return engine.Step{
		Title: b.M.T(KeyStepBinary, b.Version),
		Files: []string{BinaryPath},
		Done: func(context.Context) (bool, error) {
			if b.Exe == BinaryPath {
				return true, nil
			}
			current, err := b.Sys.ReadFile(BinaryPath)
			if err != nil {
				return false, nil
			}
			self, err := b.Sys.ReadFile(b.Exe)
			if err != nil {
				return false, err
			}
			info, err := b.Sys.Stat(BinaryPath)
			if err != nil {
				return false, err
			}
			return sha256.Sum256(current) == sha256.Sum256(self) && info.Mode.Perm() == modeRootExec.Perm, nil
		},
		Apply: func(context.Context) error {
			self, err := b.Sys.ReadFile(b.Exe)
			if err != nil {
				return err
			}
			if old, err := b.Sys.ReadFile(BinaryPath); err == nil {
				backup = old
			}
			return b.Sys.WriteFile(BinaryPath, self, modeRootExec)
		},
		Undo: func(context.Context) error {
			if backup != nil {
				return b.Sys.WriteFile(BinaryPath, backup, modeRootExec)
			}
			return b.Sys.Remove(BinaryPath)
		},
	}
}

func (b *Builder) unitStep() engine.Step {
	files := []engine.File{{Path: UnitPath, Data: UnitFile, Mode: modeRootPublic}}
	return engine.FilesStep(b.Sys, b.M.T(KeyStepUnit), files, []engine.Command{engine.Cmd("systemctl", "daemon-reload")},
		func(context.Context) (bool, error) { return engine.FilesMatch(b.Sys, files) })
}

func (b *Builder) dropInStep() engine.Step {
	reload := engine.Cmd("systemctl", "daemon-reload")
	if !b.Encrypted() {
		s := engine.RemoveStep(b.Sys, b.M.T(KeyStepDropInRemove), CredDropInPath)
		remove := s.Apply
		s.Apply = func(ctx context.Context) error {
			if err := remove(ctx); err != nil {
				return err
			}
			_, err := engine.Run(ctx, b.Sys, reload)
			return err
		}
		return s
	}
	files := []engine.File{{Path: CredDropInPath, Data: CredDropIn(), Mode: modeRootPublic}}
	s := engine.FilesStep(b.Sys, b.M.T(KeyStepDropIn), files, []engine.Command{reload},
		func(context.Context) (bool, error) { return engine.FilesMatch(b.Sys, files) })
	apply := s.Apply
	s.Apply = func(ctx context.Context) error {
		if err := b.Sys.MkdirAll(DropInDir, modePublicDir); err != nil {
			return err
		}
		return apply(ctx)
	}
	return s
}

func (b *Builder) identityStep() engine.Step {
	if b.Det.PlainID && b.Encrypted() {
		return b.convertStep()
	}
	path := IdentityPath
	if b.Encrypted() {
		path = SealedPath
	}
	return engine.Step{
		Title: b.M.T(KeyStepRegister, b.A.URL),
		Files: []string{path},
		Done: func(context.Context) (bool, error) {
			return engine.Exists(b.Sys, IdentityPath) || engine.Exists(b.Sys, SealedPath), nil
		},
		Apply: func(ctx context.Context) error {
			if b.Token == "" {
				return errNoToken
			}
			t, err := b.Target(RenderEnv(b.A, b.existing(EnvFilePath)), b.A.Dev, b.Encrypted())
			if err != nil {
				return err
			}
			reg, err := b.Protocol.Register(ctx, t, b.Token, b.A.Name)
			if err != nil {
				return err
			}
			b.Registered = &reg
			return nil
		},
	}
}

func (b *Builder) convertStep() engine.Step {
	stop := engine.Cmd("systemctl", "stop", ServiceName)
	return engine.Step{
		Title:    b.M.T(KeyStepConvert),
		Files:    []string{SealedPath},
		Commands: []string{stop.String()},
		Done:     func(context.Context) (bool, error) { return engine.Exists(b.Sys, SealedPath), nil },
		Apply: func(ctx context.Context) error {
			if engine.Exists(b.Sys, UnitPath) {
				if _, err := engine.Run(ctx, b.Sys, stop); err != nil {
					return err
				}
			}
			t, err := b.Target(b.existing(EnvFilePath), b.A.Dev, true)
			if err != nil {
				return err
			}
			return b.Protocol.Seal(ctx, t)
		},
	}
}

func (b *Builder) answersStep() engine.Step {
	data, err := b.A.Marshal()
	if err != nil {
		return engine.Step{Title: b.M.T(KeyStepAnswers), Apply: func(context.Context) error { return err }}
	}
	return engine.FileStep(b.Sys, b.M.T(KeyStepAnswers), AnswersPath, data, modeRootPrivate)
}

func (b *Builder) startStep() engine.Step {
	enable := engine.Cmd("systemctl", "enable", ServiceName)
	restart := engine.Cmd("systemctl", "restart", ServiceName)
	return engine.Step{
		Title:    b.M.T(KeyStepStart),
		Commands: []string{enable.String(), restart.String()},
		Done: func(ctx context.Context) (bool, error) {
			if b.changed || !b.serviceActive(ctx) {
				return false, nil
			}
			ready, _ := runnerReady(ctx, b.Sys)
			return ready, nil
		},
		Apply: func(ctx context.Context) error {
			if _, err := engine.Run(ctx, b.Sys, enable); err != nil {
				return err
			}
			if _, err := engine.Run(ctx, b.Sys, restart); err != nil {
				return errors.Join(err, journalTail(ctx, b.Sys, b.M))
			}
			return waitOnline(ctx, b.Sys, b.M, b.sleep)
		},
	}
}

func StateDirModeOK(perm fs.FileMode) bool { return perm == modeStateDir.Perm || perm == stateDirOpen }

func (b *Builder) stateDirStep() engine.Step {
	s := engine.DirStep(b.Sys, b.M.T(KeyStepStateDir, StateDir), StateDir, modeStateDir)
	s.Done = func(context.Context) (bool, error) {
		info, err := b.Sys.Stat(StateDir)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return info.IsDir && StateDirModeOK(info.Mode.Perm()), nil
	}
	return s
}
