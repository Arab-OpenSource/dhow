package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

const maxEnvFile = 64 << 10

var (
	NoOwner        = Owner{UID: -1, GID: -1}
	ErrEnvFileLine = errors.New("is not a KEY=VALUE line")
)

type Owner struct {
	UID int
	GID int
}

func (o Owner) Set() bool { return o.UID >= 0 && o.GID >= 0 }

func OwnerOf(path string) (Owner, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return NoOwner, fmt.Errorf("read %s: %w", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return NoOwner, fmt.Errorf("read the owner of %s: %w", path, ErrWrongOwner)
	}
	return Owner{UID: int(st.Uid), GID: int(st.Gid)}, nil
}

func ParseEnvFile(data []byte) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			return nil, fmt.Errorf("line %d %w", n, ErrEnvFileLine)
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		out[k] = v
	}
	return out, sc.Err()
}

func WithEnvFile(getenv func(string) string, path string) (func(string) string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // The fixed path of the runner's own configuration.
	if errors.Is(err, os.ErrNotExist) {
		return getenv, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxEnvFile {
		return nil, fmt.Errorf("%s %w", path, ErrTooLarge)
	}
	vars, err := ParseEnvFile(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return vars[k]
	}, nil
}
