package installer

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	IdentityAuto      = "auto"
	IdentityEncrypted = "encrypted"
	IdentityFile      = "file"
	TokenStdin        = "-"
	answersHeader     = "# Written by dhow-runner install. It holds no secrets and no token.\n"
	maxNameLength     = 100
)

var (
	ErrInvalidAnswers = engine.ErrInvalidAnswers
	envKeyRe          = regexp.MustCompile(`^DHOW_RUNNER_[A-Z0-9_]+$`)
	managedEnv        = []string{config.EnvURL, config.EnvServerOrigin, config.EnvCAFile, config.EnvDriver, config.EnvCapacity, config.EnvStateDir}
)

type Answers struct {
	Lang            string            `yaml:"lang,omitempty"`
	URL             string            `yaml:"url"`
	TokenFile       string            `yaml:"token_file,omitempty"`
	Name            string            `yaml:"name,omitempty"`
	ServerOrigin    string            `yaml:"server_origin,omitempty"`
	CAFile          string            `yaml:"ca_file,omitempty"`
	Driver          string            `yaml:"driver,omitempty"`
	Capacity        int               `yaml:"capacity,omitempty"`
	Identity        string            `yaml:"identity,omitempty"`
	Dev             bool              `yaml:"dev,omitempty"`
	InstallPackages bool              `yaml:"install_packages,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"`
}

func DefaultAnswers() Answers {
	return Answers{Driver: config.DriverPodman, Capacity: config.DefaultCapacity, Identity: IdentityAuto}
}

func ParseAnswers(r io.Reader) (Answers, error) {
	a := DefaultAnswers()
	if err := engine.DecodeAnswers(r, &a); err != nil {
		return Answers{}, err
	}
	a.normalize()
	if err := a.Validate(); err != nil {
		return Answers{}, err
	}
	return a, nil
}

func (a *Answers) normalize() {
	a.Lang = strings.ToLower(strings.TrimSpace(a.Lang))
	a.URL = strings.TrimSuffix(strings.TrimSpace(a.URL), "/")
	a.TokenFile = strings.TrimSpace(a.TokenFile)
	a.Name = strings.TrimSpace(a.Name)
	a.ServerOrigin = strings.TrimSuffix(strings.TrimSpace(a.ServerOrigin), "/")
	a.CAFile = strings.TrimSpace(a.CAFile)
	a.Driver = strings.ToLower(strings.TrimSpace(a.Driver))
	if a.Driver == "" {
		a.Driver = config.DriverPodman
	}
	if a.Capacity == 0 {
		a.Capacity = config.DefaultCapacity
	}
	a.Identity = strings.ToLower(strings.TrimSpace(a.Identity))
	if a.Identity == "" {
		a.Identity = IdentityAuto
	}
}

func (a Answers) Validate() error {
	var errs []error
	add := func(field, msg string) { errs = append(errs, fmt.Errorf("%s: %s", field, msg)) }
	if a.Lang != "" && a.Lang != engine.LangEn && a.Lang != engine.LangAr {
		add("lang", "must be en or ar")
	}
	if a.URL == "" {
		add("url", "is required: the Dhow server address, for example https://dhow.example.com")
	} else if _, err := config.ParseServerURL(a.URL, a.Dev); err != nil {
		add("url", err.Error())
	}
	if a.ServerOrigin != "" {
		if _, err := config.ParseServerURL(a.ServerOrigin, a.Dev); err != nil {
			add("server_origin", err.Error())
		}
	}
	if a.TokenFile != "" && a.TokenFile != TokenStdin && !filepath.IsAbs(a.TokenFile) {
		add("token_file", "must be an absolute path, or - to read the token from standard input")
	}
	if a.CAFile != "" && !filepath.IsAbs(a.CAFile) {
		add("ca_file", "must be an absolute path")
	}
	if len([]rune(a.Name)) > maxNameLength {
		add("name", fmt.Sprintf("must be at most %d characters", maxNameLength))
	}
	if a.Driver != config.DriverPodman && a.Driver != config.DriverFake {
		add("driver", "must be podman (fake exists only in test builds)")
	}
	if a.Capacity < config.MinCapacity || a.Capacity > config.MaxCapacity {
		add("capacity", fmt.Sprintf("must be from %d to %d", config.MinCapacity, config.MaxCapacity))
	}
	if !slices.Contains([]string{IdentityAuto, IdentityEncrypted, IdentityFile}, a.Identity) {
		add("identity", "must be auto, encrypted or file")
	}
	for _, k := range slices.Sorted(maps.Keys(a.Env)) {
		switch {
		case !envKeyRe.MatchString(k):
			add("env."+k, "only DHOW_RUNNER_* settings belong in runner.env")
		case slices.Contains(managedEnv, k):
			add("env."+k, "is set from the other answers; use them instead")
		case strings.ContainsAny(a.Env[k], "\n\r"):
			add("env."+k, "must be one line")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w:\n%w", ErrInvalidAnswers, errors.Join(errs...))
	}
	return nil
}

func (a Answers) Marshal() ([]byte, error) {
	saved := a
	saved.TokenFile = ""
	return engine.EncodeAnswers(answersHeader, saved)
}

func (a Answers) ManagedEnv() []engine.EnvEntry {
	out := []engine.EnvEntry{{Key: config.EnvURL, Value: a.URL}}
	if a.ServerOrigin != "" {
		out = append(out, engine.EnvEntry{Key: config.EnvServerOrigin, Value: a.ServerOrigin})
	}
	if a.CAFile != "" {
		out = append(out, engine.EnvEntry{Key: config.EnvCAFile, Value: a.CAFile})
	}
	out = append(out,
		engine.EnvEntry{Key: config.EnvDriver, Value: a.Driver},
		engine.EnvEntry{Key: config.EnvCapacity, Value: fmt.Sprint(a.Capacity)},
	)
	for _, k := range slices.Sorted(maps.Keys(a.Env)) {
		out = append(out, engine.EnvEntry{Key: k, Value: a.Env[k]})
	}
	return out
}

func RenderEnv(a Answers, existing []byte) []byte {
	var kept []engine.EnvEntry
	for _, e := range engine.ParseEnv(existing) {
		if !slices.Contains(managedEnv, e.Key) {
			kept = append(kept, e)
		}
	}
	return engine.RenderEnv(a.ManagedEnv(), kept)
}

func EnvGetter(data []byte) func(string) string {
	vars := map[string]string{}
	for _, e := range engine.ParseEnv(data) {
		vars[e.Key] = e.Value
	}
	return func(k string) string { return vars[k] }
}
