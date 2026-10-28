package installer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

type DatabaseChecker interface {
	ServerMajor(ctx context.Context, url string) (int, error)
}

type Interview struct {
	P   *Prompter
	M   Messages
	Det Detection
	DB  DatabaseChecker
	Out io.Writer

	KeepDatabaseURL bool
}

func (iv Interview) say(format string, args ...any) { _, _ = fmt.Fprintf(iv.Out, format, args...) }

func (iv Interview) Run(ctx context.Context, prev Answers) (Answers, error) {
	a := prev
	a.PublicURL = ""
	if err := iv.askMode(&a); err != nil {
		return a, err
	}
	domain, err := iv.P.Ask(iv.M.T(KeyAskDomain), a.Domain, func(v string) error {
		if !ValidDomain(strings.ToLower(v)) {
			return errors.New(iv.M.T(KeyInvalidDomain))
		}
		return nil
	})
	if err != nil {
		return a, err
	}
	a.Domain = strings.ToLower(domain)
	if a.Mode == ModeNative {
		if err := iv.askDatabase(ctx, &a); err != nil {
			return a, err
		}
	} else {
		a.Database = DatabaseAnswers{Mode: DatabaseInstall}
	}
	if err := iv.askProxy(&a); err != nil {
		return a, err
	}
	breached, err := iv.P.Confirm(iv.M.T(KeyAskBreached), a.BreachedCheck())
	if err != nil {
		return a, err
	}
	a.BreachedPasswordCheck = &breached
	if err := iv.askFirewall(&a); err != nil {
		return a, err
	}
	if err := iv.askPackages(&a); err != nil {
		return a, err
	}
	a.normalize()
	return a, a.Validate()
}

func (iv Interview) askMode(a *Answers) error {
	if !iv.Det.Compose {
		a.Mode = ModeNative
		return nil
	}
	iv.say("\n%s\n", iv.M.T(KeyDockerGroupWarning))
	mode, err := iv.P.Choose(iv.M.T(KeyAskMode), []Option{
		{ModeNative, iv.M.T(KeyModeNative)},
		{ModeCompose, iv.M.T(KeyModeCompose)},
	}, a.Mode)
	a.Mode = mode
	return err
}

func (iv Interview) askDatabase(ctx context.Context, a *Answers) error {
	var options []Option
	if iv.Det.CanInstallPostgres() {
		options = append(options, Option{DatabaseInstall, iv.M.T(KeyDatabaseInstall)})
	}
	options = append(options, Option{DatabaseExisting, iv.M.T(KeyDatabaseExisting)})
	def := a.Database.Mode
	if !iv.Det.CanInstallPostgres() {
		def = DatabaseExisting
	}
	mode, err := iv.P.Choose(iv.M.T(KeyAskDatabase), options, def)
	if err != nil {
		return err
	}
	a.Database = DatabaseAnswers{Mode: mode}
	if mode == DatabaseInstall {
		return nil
	}
	if iv.KeepDatabaseURL {
		iv.say("  %s\n", iv.M.T(KeyDatabaseKept, DatabaseURLPath))
		a.KeepDatabaseURL()
		return nil
	}
	iv.say("%s\n", iv.M.T(KeySSLModeGuidance))
	raw, err := iv.P.Secret(iv.M.T(KeyAskDatabaseURL), func(v string) error {
		if err := config.ValidateDatabaseURL(v); err != nil {
			return errors.New(iv.M.T(KeyInvalidDatabaseURL, err.Error()))
		}
		return iv.checkDatabase(ctx, v)
	})
	if err != nil {
		return err
	}
	a.SetDatabaseURL(raw)
	return nil
}

func (iv Interview) checkDatabase(ctx context.Context, raw string) error {
	if iv.DB == nil {
		return nil
	}
	major, err := iv.DB.ServerMajor(ctx, raw)
	if err != nil {
		return errors.New(iv.M.T(KeyDatabaseUnreachable, err.Error()))
	}
	if major < RequiredPostgres {
		return errors.New(iv.M.T(KeyDatabaseTooOld, major, RequiredPostgres))
	}
	iv.say("  %s\n", iv.M.T(KeyDatabaseOK, major, sslModeOf(raw)))
	return nil
}

func sslModeOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Query().Get("sslmode")
}

func (iv Interview) proxyOptions() []Option {
	d := iv.Det
	var options []Option
	if d.Caddy || (d.PackagesInstallable() && d.Distro.CaddyInstallable()) {
		options = append(options, Option{ProxyCaddy, iv.M.T(KeyProxyCaddy)})
	}
	if (d.Nginx && d.Certbot) || (d.PackagesInstallable() && d.Distro.ExtraPackagesInstallable()) {
		options = append(options, Option{ProxyNginx, iv.M.T(KeyProxyNginx)})
	}
	return append(options,
		Option{ProxyExisting, iv.M.T(KeyProxyExisting)},
		Option{ProxyNone, iv.M.T(KeyProxyNone)})
}

func (iv Interview) askProxy(a *Answers) error {
	options := iv.proxyOptions()
	def := a.Proxy.Kind
	if !hasOption(options, def) {
		def = options[0].Value
	}
	kind, err := iv.P.Choose(iv.M.T(KeyAskProxy), options, def)
	if err != nil {
		return err
	}
	prev := a.Proxy
	a.Proxy = ProxyAnswers{Kind: kind}
	switch kind {
	case ProxyNginx:
		a.Proxy.Email, err = iv.P.Ask(iv.M.T(KeyAskEmail), prev.Email, func(v string) error {
			if !emailRe.MatchString(v) {
				return errors.New(iv.M.T(KeyInvalidEmail))
			}
			return nil
		})
	case ProxyExisting, ProxyNone:
		def := prev.TrustedProxies
		if def == "" && kind == ProxyExisting {
			def = loopbackProxies
		}
		a.Proxy.TrustedProxies, err = iv.P.Ask(iv.M.T(KeyAskTrustedProxies), def, func(v string) error {
			if err := config.ValidateTrustedProxies(v); err != nil {
				return errors.New(iv.M.T(KeyInvalidTrustedProxies, err.Error()))
			}
			return nil
		})
	}
	return err
}

func hasOption(options []Option, v string) bool {
	for _, o := range options {
		if o.Value == v {
			return true
		}
	}
	return false
}

func (iv Interview) askFirewall(a *Answers) error {
	a.Firewall = false
	if !a.ManagedProxy() || iv.Det.Firewall == FirewallNone {
		return nil
	}
	open, err := iv.P.Confirm(iv.M.T(KeyAskFirewall, iv.Det.Firewall), false)
	a.Firewall = open
	return err
}

func (iv Interview) askPackages(a *Answers) error {
	missing := MissingPackages(iv.Det, *a)
	a.InstallPackages = false
	if len(missing) == 0 {
		return nil
	}
	if !iv.Det.PackagesInstallable() {
		iv.say("%s\n", iv.M.T(KeyBlockManual, iv.Det.Distro.PrettyName, strings.Join(missing, " ")))
		return nil
	}
	ok, err := iv.P.Confirm(iv.M.T(KeyAskPackages, strings.Join(missing, " ")), true)
	a.InstallPackages = ok
	return err
}
