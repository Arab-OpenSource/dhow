package installer

import (
	"context"
	"strings"
)

const AptEnv = "DEBIAN_FRONTEND=noninteractive"

func AptInstalled(ctx context.Context, sys System, pkgs []string) bool {
	for _, p := range pkgs {
		out, err := sys.Run(ctx, Cmd("dpkg-query", "-W", "-f=${Status}", p))
		if err != nil || !strings.Contains(string(out.Stdout), "install ok installed") {
			return false
		}
	}
	return true
}

func RpmInstalled(ctx context.Context, sys System, pkgs []string) bool {
	return Succeeds(ctx, sys, Cmd("rpm", append([]string{"-q"}, pkgs...)...))
}

func PackagesInstalled(ctx context.Context, sys System, family Family, pkgs []string) bool {
	if family == FamilyApt {
		return AptInstalled(ctx, sys, pkgs)
	}
	return RpmInstalled(ctx, sys, pkgs)
}

func AptInstallStep(sys System, title string, pkgs []string) Step {
	install := Command{Name: "apt-get", Args: append([]string{"install", "-y", "--no-install-recommends"}, pkgs...), Env: []string{AptEnv}}
	update := Command{Name: "apt-get", Args: []string{"update"}, Env: []string{AptEnv}}
	return Step{
		Title:    title,
		Packages: pkgs,
		Commands: []string{update.String(), install.String()},
		Done:     func(ctx context.Context) (bool, error) { return AptInstalled(ctx, sys, pkgs), nil },
		Apply: func(ctx context.Context) error {
			if _, err := Run(ctx, sys, update); err != nil {
				return err
			}
			_, err := Run(ctx, sys, install)
			return err
		},
	}
}

func DnfInstallStep(sys System, title string, pkgs []string) Step {
	install := Cmd("dnf", append([]string{"install", "-y"}, pkgs...)...)
	return Step{
		Title:    title,
		Packages: pkgs,
		Commands: []string{install.String()},
		Done:     func(ctx context.Context) (bool, error) { return RpmInstalled(ctx, sys, pkgs), nil },
		Apply: func(ctx context.Context) error {
			_, err := Run(ctx, sys, install)
			return err
		},
	}
}

func InstallStep(sys System, family Family, title string, pkgs []string) Step {
	if family == FamilyApt {
		return AptInstallStep(sys, title, pkgs)
	}
	return DnfInstallStep(sys, title, pkgs)
}
