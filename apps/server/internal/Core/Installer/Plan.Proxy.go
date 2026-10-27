package installer

import (
	"context"
	"errors"
	"strings"
)

const seBoolProxy = "httpd_can_network_connect"

func (b *Builder) proxySteps() []Step {
	var steps []Step
	switch b.A.Proxy.Kind {
	case ProxyCaddy:
		steps = append(steps, b.caddyStep())
	case ProxyNginx:
		steps = append(steps, b.nginxSteps()...)
	default:
		return nil
	}
	if b.Det.SELinux {
		steps = append(steps, b.seBoolStep())
	}
	return steps
}

func (b *Builder) caddyStep() Step {
	caddyfile := WithCaddyImport(b.readExisting(CaddyfilePath))
	files := []fileSpec{
		{Path: CaddySitePath, Data: CaddySite(b.A), Mode: modeRootPublic},
		{Path: CaddyfilePath, Data: caddyfile, Mode: modeRootPublic},
	}
	after := []Command{
		Cmd("caddy", "validate", "--config", CaddyfilePath, "--adapter", "caddyfile"),
		Cmd("systemctl", "enable", "caddy"),
		Cmd("systemctl", "reload-or-restart", "caddy"),
	}
	s := b.ConfigStep(b.M.T(KeyStepCaddy, b.A.Domain), files, after, func(ctx context.Context) (bool, error) {
		ok, err := b.filesMatch(files)
		return ok && b.serviceActive(ctx, "caddy"), err
	})
	return b.reloadOnUndo(s, "caddy")
}

func (b *Builder) reloadOnUndo(s Step, service string) Step {
	undo := s.Undo
	s.Undo = func(ctx context.Context) error {
		err := undo(ctx)
		_, rerr := run(ctx, b.Sys, Cmd("systemctl", "reload-or-restart", service))
		return errors.Join(err, rerr)
	}
	return s
}

func (b *Builder) certPath() string {
	return LetsEncryptLive + "/" + b.A.Domain + "/fullchain.pem"
}

func (b *Builder) nginxSteps() []Step {
	reload := []Command{
		Cmd("nginx", "-t"),
		Cmd("systemctl", "enable", "--now", "nginx"),
		Cmd("systemctl", "reload", "nginx"),
	}
	httpOnly := []fileSpec{{Path: NginxSitePath, Data: NginxHTTPOnly(b.A), Mode: modeRootPublic}}
	challenge := b.ConfigStep(b.M.T(KeyStepNginxChallenge, b.A.Domain), httpOnly, reload, func(ctx context.Context) (bool, error) {
		if exists(b.Sys, b.certPath()) {
			return true, nil
		}
		ok, err := b.filesMatch(httpOnly)
		return ok && b.serviceActive(ctx, "nginx"), err
	})
	certbot := Cmd("certbot", "certonly", "--webroot", "--webroot-path", AcmeWebroot,
		"--non-interactive", "--agree-tos", "--keep-until-expiring",
		"--email", b.A.Proxy.Email, "--domain", b.A.Domain)
	tls := []fileSpec{{Path: NginxSitePath, Data: NginxTLS(b.A), Mode: modeRootPublic}}
	return []Step{
		DirStep(b.Sys, b.M.T(KeyStepAcmeDir), AcmeWebroot, modePublicDir),
		b.reloadOnUndo(challenge, "nginx"),
		{
			Title:    b.M.T(KeyStepCertbot, b.A.Domain),
			Files:    []string{LetsEncryptLive + "/" + b.A.Domain + "/"},
			Commands: []string{certbot.String()},
			Done:     func(context.Context) (bool, error) { return exists(b.Sys, b.certPath()), nil },
			Apply: func(ctx context.Context) error {
				_, err := run(ctx, b.Sys, certbot)
				return err
			},
		},
		FileStep(b.Sys, b.M.T(KeyStepCertbotHook), CertbotHookPath, CertbotReloadHook(), modeRootExec),
		b.reloadOnUndo(b.ConfigStep(b.M.T(KeyStepNginxTLS, b.A.Domain), tls, reload, func(ctx context.Context) (bool, error) {
			ok, err := b.filesMatch(tls)
			return ok && b.serviceActive(ctx, "nginx"), err
		}), "nginx"),
	}
}

func (b *Builder) seBoolStep() Step {
	set := Cmd("setsebool", "-P", seBoolProxy, "on")
	return Step{
		Title:    b.M.T(KeyStepSELinux),
		Commands: []string{set.String()},
		Done: func(ctx context.Context) (bool, error) {
			out, err := b.Sys.Run(ctx, Cmd("getsebool", seBoolProxy))
			return err == nil && strings.Contains(string(out.Stdout), "--> on"), nil
		},
		Apply: func(ctx context.Context) error {
			_, err := run(ctx, b.Sys, set)
			return err
		},
	}
}

func (b *Builder) firewallSteps() []Step {
	if !b.A.Firewall || !b.A.ManagedProxy() {
		return nil
	}
	switch b.Det.Firewall {
	case FirewallUfw:
		return []Step{CommandStep(b.Sys, b.M.T(KeyStepFirewall, FirewallUfw), nil,
			[]Command{Cmd("ufw", "allow", "80/tcp"), Cmd("ufw", "allow", "443/tcp")},
			[]Command{Cmd("ufw", "delete", "allow", "80/tcp"), Cmd("ufw", "delete", "allow", "443/tcp")})}
	case FirewallFirewalld:
		return []Step{CommandStep(b.Sys, b.M.T(KeyStepFirewall, FirewallFirewalld), nil,
			[]Command{
				Cmd("firewall-cmd", "--permanent", "--add-service=http", "--add-service=https"),
				Cmd("firewall-cmd", "--reload"),
			},
			[]Command{
				Cmd("firewall-cmd", "--permanent", "--remove-service=http", "--remove-service=https"),
				Cmd("firewall-cmd", "--reload"),
			})}
	}
	return nil
}
