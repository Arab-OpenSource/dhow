package installer

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"strings"

	engine "github.com/Khaymah/dhow/internal/installer"
)

type permCheck struct {
	Path string
	Perm fs.FileMode
	UID  int
	GID  int
	Mode engine.FileMode
}

func (d doctor) permissions() engine.Finding {
	uid, gid := d.det.UID, d.det.GID
	checks := []permCheck{
		{BinaryPath, modeRootExec.Perm, 0, 0, modeRootExec},
		{UnitPath, modeRootPublic.Perm, 0, 0, modeRootPublic},
		{ConfigDir, modeConfigDir.Perm, 0, gid, modeConfigDir},
		{EnvFilePath, modeEnvFile.Perm, 0, gid, modeEnvFile},
		{AnswersPath, modeRootPrivate.Perm, 0, 0, modeRootPrivate},
		{StateDir, modeStateDir.Perm, uid, gid, modeStateDir},
		{IdentityPath, modeRootPrivate.Perm, uid, gid, engine.FileMode{Perm: 0o600, Owner: ServiceUser, Group: ServiceUser}},
		{SealedPath, modeRootPrivate.Perm, 0, 0, modeRootPrivate},
	}
	var wrong []permCheck
	var details []string
	for _, c := range checks {
		info, err := d.app.Sys.Stat(c.Path)
		if err != nil {
			continue
		}
		if info.Mode.Perm() != c.Perm || info.UID != c.UID || info.GID != c.GID {
			wrong = append(wrong, c)
			details = append(details, fmt.Sprintf("%s %04o uid=%d gid=%d (%s %04o uid=%d gid=%d)",
				c.Path, info.Mode.Perm(), info.UID, info.GID, d.m.T(engine.KeyWant), c.Perm, c.UID, c.GID))
		}
	}
	title := d.m.T(KeyCheckPermissions)
	if len(wrong) == 0 {
		return engine.OK(title)
	}
	fix := engine.Step{Title: d.m.T(KeyFixPermissions), Apply: func(context.Context) error {
		for _, c := range wrong {
			if err := d.app.Sys.Chmod(c.Path, c.Mode); err != nil {
				return err
			}
		}
		return nil
	}}
	for _, c := range wrong {
		owner, group := c.Mode.Owner, c.Mode.Group
		if owner == "" {
			owner, group = "root", "root"
		}
		fix.Commands = append(fix.Commands, fmt.Sprintf("chown %s:%s %s && chmod %04o %s", owner, group, c.Path, c.Perm, c.Path))
	}
	return d.fail(title, strings.Join(details, "; "), "", &fix)
}

func (d doctor) identity() engine.Finding {
	title := d.m.T(KeyCheckIdentity)
	switch {
	case d.det.PlainID && d.det.SealedID:
		return d.fail(title, d.m.T(KeyBlockBothIdentities, IdentityPath, SealedPath), d.m.T(KeyAdviceBoth), nil)
	case d.det.SealedID:
		dropIn, err := d.app.Sys.ReadFile(CredDropInPath)
		if err != nil || !bytes.Equal(dropIn, CredDropIn()) {
			d.b.A.Identity = IdentityEncrypted
			fix := d.b.dropInStep()
			fix.Done = nil
			return d.fail(title, d.m.T(KeyDropInMissing, CredDropInPath), "", &fix)
		}
		f := engine.OK(title)
		f.Detail = d.m.T(KeyIdentitySealed, SealedPath) + " (" + credsLabel(d.det.Creds, d.m) + ")"
		if d.det.Creds == CredsNone {
			return d.fail(title, d.m.T(KeyCredsGone), "", nil)
		}
		return f
	case d.det.PlainID:
		detail := d.m.T(KeyIdentityPlain, IdentityPath) + " " + d.m.T(KeyPlainNoCreds)
		if d.det.Creds != CredsNone {
			detail = d.m.T(KeyIdentityPlain, IdentityPath) + " " + d.m.T(KeyPlainCanEncrypt)
		}
		return engine.Finding{Severity: engine.SeverityInfo, Title: title, Detail: detail}
	default:
		return d.fail(title, d.m.T(KeyNotRegistered), d.m.T(KeyAdviceReinstall), nil)
	}
}

func (d doctor) pending() engine.Finding {
	title := d.m.T(KeyCheckPending)
	for _, p := range []string{PendingPath, SealedPendingPath} {
		if engine.Exists(d.app.Sys, p) {
			return engine.Finding{Severity: engine.SeverityWarn, Title: title, Detail: d.m.T(KeyPendingLeft, p), Advice: d.m.T(KeyAdvicePending)}
		}
	}
	return engine.OK(title)
}

func (d doctor) server(ctx context.Context) []engine.Finding {
	reach, signIn := d.m.T(KeyCheckReach), d.m.T(KeyCheckSignIn)
	if !d.det.Registered() || (d.det.PlainID && d.det.SealedID) {
		return nil
	}
	t, err := d.app.target(d.env, d.b.A.Dev, d.det.SealedID)
	if err != nil {
		return []engine.Finding{d.fail(reach, err.Error(), d.m.T(KeyAdviceEnv, EnvFilePath), nil)}
	}
	if d.b.serviceActive(ctx) {
		id, err := d.app.Protocol.Reach(ctx, t)
		if err != nil {
			return []engine.Finding{d.fail(reach, err.Error(), d.m.T(KeyAdviceReach), nil)}
		}
		out := []engine.Finding{withDetail(engine.OK(reach), serverAddress(t))}
		ready, last := runnerReady(ctx, d.app.Sys)
		if !ready {
			return append(out, d.fail(signIn, strings.TrimSpace(d.m.T(KeyNotSignedIn)+" "+last), "journalctl -u "+ServiceName+" -n 50", nil))
		}
		return append(out, withDetail(engine.OK(signIn), d.m.T(KeySignedInLog, id)))
	}
	id, err := d.app.Protocol.SignIn(ctx, t)
	if err != nil {
		return []engine.Finding{d.fail(signIn, err.Error(), d.m.T(KeyAdviceReach), nil)}
	}
	return []engine.Finding{withDetail(engine.OK(reach), serverAddress(t)), withDetail(engine.OK(signIn), d.m.T(KeySignedIn, id))}
}

func withDetail(f engine.Finding, detail string) engine.Finding {
	f.Detail = detail
	return f
}

func serverAddress(t Target) string {
	if t.Config.URL == nil {
		return ""
	}
	return t.Config.URL.String()
}
