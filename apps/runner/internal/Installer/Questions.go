package installer

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

type Interview struct {
	P   *engine.Prompter
	M   engine.Messages
	Det Detection
	Out io.Writer
	Env []byte
}

func (iv Interview) say(format string, args ...any) { _, _ = fmt.Fprintf(iv.Out, format, args...) }

func (iv Interview) Run(prev Answers) (Answers, string, error) {
	a := prev
	iv.say("\n")
	url, err := iv.P.Ask(iv.M.T(KeyAskURL), a.URL, func(v string) error {
		_, err := config.ParseServerURL(v, a.Dev)
		return err
	})
	if err != nil {
		return Answers{}, "", err
	}
	a.URL = url
	token := ""
	if !iv.Det.Registered() {
		if token, err = iv.P.Secret(iv.M.T(KeyAskToken), func(v string) error {
			_, err := ValidToken(v)
			return err
		}); err != nil {
			return Answers{}, "", err
		}
		if a.Name, err = iv.P.Ask(iv.M.T(KeyAskName), a.Name, nil); err != nil {
			return Answers{}, "", err
		}
	}
	if err := iv.askIdentity(&a); err != nil {
		return Answers{}, "", err
	}
	capacity, err := iv.P.Ask(iv.M.T(KeyAskCapacity, config.MaxCapacity), strconv.Itoa(a.Capacity), func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < config.MinCapacity || n > config.MaxCapacity {
			return errors.New(iv.M.T(KeyCapacityInvalid, config.MaxCapacity))
		}
		return nil
	})
	if err != nil {
		return Answers{}, "", err
	}
	a.Capacity, _ = strconv.Atoi(capacity)
	if err := iv.askMemory(&a); err != nil {
		return Answers{}, "", err
	}
	b := Builder{Det: iv.Det, A: a}
	if missing := b.MissingPackages(); len(missing) > 0 && iv.Det.Distro.Family != engine.FamilyUnknown {
		if a.InstallPackages, err = iv.P.Confirm(iv.M.T(KeyAskPackages, fmt.Sprint(missing)), true); err != nil {
			return Answers{}, "", err
		}
	}
	return a, token, a.Validate()
}

func (iv Interview) askIdentity(a *Answers) error {
	if iv.Det.SealedID || iv.Det.Creds == CredsNone {
		return nil
	}
	def := IdentityEncrypted
	if a.Identity == IdentityFile {
		def = IdentityFile
	}
	choice, err := iv.P.Choose(iv.M.T(KeyAskIdentity), []engine.Option{
		{Value: IdentityEncrypted, Label: iv.M.T(KeyOptionEncrypted)},
		{Value: IdentityFile, Label: iv.M.T(KeyOptionFile)},
	}, def)
	if err != nil {
		return err
	}
	a.Identity = choice
	return nil
}

func (iv Interview) askMemory(a *Answers) error {
	c := CheckMemory(RenderEnv(*a, iv.Env), iv.Det.MemoryBytes)
	if c.Err != nil || c.Fits || c.Explicit || !c.CanFit {
		return nil
	}
	iv.say("%s\n", iv.M.T(KeyAskMemoryIntro, hostSize(c.Machine), FormatSize(config.DefaultJobMemory().Total()), hostSize(Headroom(c.Machine))))
	memoryDef, diskDef := FormatSize(c.Fitted.MemoryBytes), FormatSize(c.Fitted.DiskBytes)
	memory, err := iv.P.Ask(iv.M.T(KeyAskMaxMemory), memoryDef, func(v string) error {
		return iv.checkLimits(v, diskDef, false)
	})
	if err != nil {
		return err
	}
	disk, err := iv.P.Ask(iv.M.T(KeyAskMaxDisk), diskDef, func(v string) error {
		return iv.checkLimits(memory, v, true)
	})
	if err != nil {
		return err
	}
	if memory == memoryDef && disk == diskDef {
		return nil
	}
	env := maps.Clone(a.Env)
	if env == nil {
		env = map[string]string{}
	}
	env[config.EnvMaxMemory], env[config.EnvMaxDisk] = memory, disk
	a.Env = env
	return nil
}

func (iv Interview) checkLimits(memory, disk string, fit bool) error {
	vars := map[string]string{config.EnvMaxMemory: memory, config.EnvMaxDisk: disk}
	j, err := config.ReadJobMemory(func(k string) string { return vars[k] })
	if err != nil || !safeEnvValue(memory) || !safeEnvValue(disk) {
		return errors.New(iv.M.T(KeySizeInvalid))
	}
	if !fit {
		return nil
	}
	if config.CheckJobMemory(j, iv.Det.MemoryBytes) != nil {
		return errors.New(MemoryCheck{Machine: iv.Det.MemoryBytes, Current: j}.TooHigh(iv.M))
	}
	return nil
}
