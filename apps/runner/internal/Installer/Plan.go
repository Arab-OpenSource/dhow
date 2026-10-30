package installer

import (
	"context"
	"strings"
	"time"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

type Builder struct {
	Sys      engine.System
	Det      Detection
	A        Answers
	M        engine.Messages
	Exe      string
	Version  string
	Token    string
	Protocol Protocol
	Target   func(env []byte, dev, encrypted bool) (Target, error)
	Sleep    func(time.Duration)

	Registered *Registered
	changed    bool
}

func (b *Builder) sleep(d time.Duration) {
	if b.Sleep != nil {
		b.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (b *Builder) Encrypted() bool {
	switch {
	case b.Det.SealedID:
		return true
	case b.A.Identity == IdentityFile:
		return false
	case b.A.Identity == IdentityEncrypted:
		return true
	default:
		return b.Det.Creds != CredsNone
	}
}

func (b *Builder) Sandbox() bool { return b.A.Driver == config.DriverPodman }

func (b *Builder) MissingPackages() []string {
	if !b.Sandbox() {
		return nil
	}
	var out []string
	if b.Det.Podman == "" {
		out = append(out, "podman")
	}
	if !b.Det.Crun {
		out = append(out, "crun")
	}
	if !b.Det.Newuidmap {
		if b.Det.Distro.Family == engine.FamilyDnf {
			out = append(out, "shadow-utils")
		} else {
			out = append(out, "uidmap")
		}
	}
	return out
}

func (b *Builder) Build() engine.Plan {
	var p engine.Plan
	memoryBlockers, memoryNotes := b.fitMemory()
	p.Blockers = append(b.blockers(), memoryBlockers...)
	p.Notes = append(b.notes(), memoryNotes...)
	if len(p.Blockers) > 0 {
		return p
	}
	p.Steps = append(p.Steps, b.packageSteps()...)
	p.Steps = append(p.Steps,
		b.userStep(),
		b.subIDStep(),
		b.stateDirStep(),
		engine.DirStep(b.Sys, b.M.T(KeyStepConfigDir, ConfigDir), ConfigDir, modeConfigDir),
		b.track(engine.FileStep(b.Sys, b.M.T(KeyStepEnvFile), EnvFilePath, RenderEnv(b.A, b.existing(EnvFilePath)), modeEnvFile)),
		b.track(b.binaryStep()),
		b.track(b.unitStep()),
		b.track(b.dropInStep()),
		b.track(b.identityStep()),
		b.answersStep(),
		b.startStep(),
	)
	return p
}

func (b *Builder) blockers() []string {
	var out []string
	det, m := b.Det, b.M
	if !det.Systemd {
		out = append(out, m.T(KeyBlockSystemd))
	}
	if b.A.Identity == IdentityEncrypted && det.Creds == CredsNone && !det.SealedID {
		out = append(out, m.T(KeyBlockNoCreds))
	}
	if det.PlainID && det.SealedID {
		out = append(out, m.T(KeyBlockBothIdentities, IdentityPath, SealedPath))
	}
	if !b.Sandbox() {
		return out
	}
	if missing := b.MissingPackages(); len(missing) > 0 {
		list := strings.Join(missing, " ")
		switch {
		case det.Distro.Family == engine.FamilyUnknown:
			out = append(out, m.T(KeyBlockManual, det.Distro.PrettyName, list))
		case !b.A.InstallPackages:
			out = append(out, m.T(KeyBlockDeclined, list))
		}
	}
	if det.Podman != "" && !PodmanAtLeast(det.Podman, MinPodman) {
		out = append(out, m.T(KeyBlockPodmanOld, det.Podman, MinPodman))
	}
	if !det.CgroupV2 {
		out = append(out, m.T(KeyBlockCgroup))
	}
	if !det.UserNS {
		out = append(out, m.T(KeyBlockUserNS))
	}
	if det.AppArmorNS {
		out = append(out, m.T(KeyBlockAppArmor))
	}
	if det.SELinux {
		out = append(out, m.T(KeyBlockSELinux, m.T(KeySupportedHostsURL)))
	}
	return out
}

func (b *Builder) notes() []string {
	var out []string
	switch {
	case !b.Sandbox():
		out = append(out, b.M.T(KeyNoteFakeDriver))
	case b.Det.Distro.ID == "":
		out = append(out, b.M.T(KeyNoteUnknownDistro))
	}
	if !b.Encrypted() {
		if b.Det.Creds == CredsNone {
			out = append(out, b.M.T(KeyNotePlainNoCreds, IdentityPath))
		} else {
			out = append(out, b.M.T(KeyNotePlainChosen, IdentityPath))
		}
	}
	if b.Det.PlainID && b.Encrypted() {
		out = append(out, b.M.T(KeyNoteConvert, IdentityPath, SealedPath))
	}
	if b.Det.User && !b.Det.UserCreated {
		out = append(out, b.M.T(KeyNoteUserAdopted, ServiceUser))
	}
	if b.Det.Registered() && b.Token != "" {
		out = append(out, b.M.T(KeyNoteTokenUnused))
	}
	return out
}

func (b *Builder) track(s engine.Step) engine.Step {
	apply := s.Apply
	s.Apply = func(ctx context.Context) error {
		b.changed = true
		return apply(ctx)
	}
	return s
}

func (b *Builder) existing(path string) []byte {
	data, err := b.Sys.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func (b *Builder) serviceActive(ctx context.Context) bool {
	return engine.Succeeds(ctx, b.Sys, engine.Cmd("systemctl", "is-active", "--quiet", ServiceName))
}
