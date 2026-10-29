package fakesys

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Khaymah/dhow/internal/installer"
)

const defaultFree = 100 << 30

var defaultChecks = []string{"getent ", "systemctl is-", "rpm -q", "dpkg-query ", "systemd-creds has-tpm2"}

type File struct {
	Link string
	Data []byte
	Mode installer.FileMode
	Dir  bool
	UID  int
	GID  int
}

type Sys struct {
	mu       sync.Mutex
	Files    map[string]*File
	Bins     map[string]bool
	Root     bool
	OK       map[string]bool
	Fail     map[string]bool
	FailOnce map[string]bool
	Stdout   map[string]string
	Checks   []string
	OnRun    func(s *Sys, c installer.Command) (installer.Output, bool, error)
	HTTPFn   func(p installer.Probe) (installer.ProbeResult, error)
	Fetched  map[string][]byte
	Free     uint64
	Clock    time.Time
	Exe      string
	Owners   map[string]int
	commands []installer.Command
	Shredded []string
}

var _ installer.SafeSystem = (*Sys)(nil)

func New() *Sys {
	return &Sys{
		Files:    map[string]*File{},
		Bins:     map[string]bool{},
		OK:       map[string]bool{},
		Fail:     map[string]bool{},
		FailOnce: map[string]bool{},
		Stdout:   map[string]string{},
		Checks:   append([]string(nil), defaultChecks...),
		Fetched:  map[string][]byte{},
		Owners:   map[string]int{"root": 0},
		Root:     true,
		Free:     defaultFree,
		Clock:    time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		Exe:      "/tmp/get/dhow",
	}
}

func (s *Sys) Put(p, content string, perm fs.FileMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[p] = &File{Data: []byte(content), Mode: installer.FileMode{Perm: perm}}
}

func (s *Sys) PutDir(p string, mode installer.FileMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[p] = &File{Mode: mode, Dir: true, UID: s.Owners[mode.Owner], GID: s.Owners[mode.Group]}
}

func (s *Sys) PutLink(p, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[p] = &File{Link: target, Mode: installer.FileMode{Perm: 0o777}}
}

func (s *Sys) follow(p string) (*File, bool) {
	f, ok := s.Files[p]
	for range 8 {
		if !ok || f.Link == "" {
			return f, ok
		}
		f, ok = s.Files[f.Link]
	}
	return nil, false
}

func (s *Sys) Content(p string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.Files[p]; ok {
		return string(f.Data)
	}
	return ""
}

func (s *Sys) Has(p string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.Files[p]
	return ok
}

func (s *Sys) ModeOf(p string) installer.FileMode {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.Files[p]; ok {
		return f.Mode
	}
	return installer.FileMode{}
}

func (s *Sys) Commands() []installer.Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]installer.Command(nil), s.commands...)
}

func (s *Sys) CommandLines() []string {
	var out []string
	for _, c := range s.Commands() {
		out = append(out, c.String())
	}
	return out
}

func (s *Sys) Ran(prefix string) bool {
	for _, c := range s.CommandLines() {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (s *Sys) failed(line string) bool {
	for k := range s.FailOnce {
		if strings.HasPrefix(line, k) {
			delete(s.FailOnce, k)
			return true
		}
	}
	for k := range s.Fail {
		if strings.HasPrefix(line, k) {
			return true
		}
	}
	return false
}

func (s *Sys) isCheck(line string) bool {
	for _, p := range s.Checks {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

func (s *Sys) Run(_ context.Context, c installer.Command) (installer.Output, error) {
	line := c.String()
	s.mu.Lock()
	s.commands = append(s.commands, c)
	hook := s.OnRun
	s.mu.Unlock()
	if hook != nil {
		if out, handled, err := hook(s, c); handled {
			return out, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := installer.Output{Stdout: []byte(s.Stdout[line])}
	if s.failed(line) || (s.isCheck(line) && !s.OK[line]) {
		out.ExitCode = 1
		out.Stderr = []byte("simulated failure")
		return out, fmt.Errorf("%w with exit code 1", installer.ErrCommandFailed)
	}
	return out, nil
}

func (s *Sys) LookPath(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Bins[name]
}

func (s *Sys) ReadFile(p string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.follow(p)
	if !ok || f.Dir {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), f.Data...), nil
}

func (s *Sys) WriteFile(p string, data []byte, mode installer.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[p] = &File{Data: append([]byte(nil), data...), Mode: mode, UID: s.Owners[mode.Owner], GID: s.Owners[mode.Group]}
	return nil
}

func (s *Sys) Chmod(p string, mode installer.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.follow(p)
	if !ok {
		return &fs.PathError{Op: "chmod", Path: p, Err: fs.ErrNotExist}
	}
	f.Mode, f.UID, f.GID = mode, s.Owners[mode.Owner], s.Owners[mode.Group]
	return nil
}

func infoOf(f *File) installer.FileInfo {
	mode := f.Mode.Perm
	switch {
	case f.Link != "":
		mode |= fs.ModeSymlink
	case f.Dir:
		mode |= fs.ModeDir
	}
	return installer.FileInfo{Mode: mode, UID: f.UID, GID: f.GID, Size: int64(len(f.Data)), IsDir: f.Dir}
}

func (s *Sys) Stat(p string) (installer.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.follow(p)
	if !ok {
		return installer.FileInfo{}, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}
	return infoOf(f), nil
}

func (s *Sys) Lstat(p string) (installer.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.Files[p]
	if !ok {
		return installer.FileInfo{}, &fs.PathError{Op: "lstat", Path: p, Err: fs.ErrNotExist}
	}
	return infoOf(f), nil
}

func (s *Sys) noFollow(p string) (*File, error) {
	f, ok := s.Files[p]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
	}
	if f.Link != "" {
		return nil, fmt.Errorf("%s %w", p, installer.ErrNotRegular)
	}
	return f, nil
}

func (s *Sys) ChmodNoFollow(p string, mode installer.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.noFollow(p)
	if err != nil {
		return err
	}
	f.Mode, f.UID, f.GID = mode, s.Owners[mode.Owner], s.Owners[mode.Group]
	return nil
}

func (s *Sys) ReadFileNoFollow(p string, limit int64) ([]byte, installer.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.noFollow(p)
	if err != nil {
		return nil, installer.FileInfo{}, err
	}
	if f.Dir {
		return nil, installer.FileInfo{}, fmt.Errorf("%s %w", p, installer.ErrNotRegular)
	}
	if int64(len(f.Data)) > limit {
		return nil, installer.FileInfo{}, fmt.Errorf("%s is larger than %d bytes", p, limit)
	}
	return append([]byte(nil), f.Data...), infoOf(f), nil
}

func (s *Sys) ShredNoFollow(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.Files[p]
	if !ok {
		return nil
	}
	if f.Link == "" && !f.Dir {
		s.Shredded = append(s.Shredded, p)
	}
	delete(s.Files, p)
	return nil
}

func (s *Sys) Remove(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Files, p)
	return nil
}

func (s *Sys) RemoveAll(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.Files {
		if k == p || strings.HasPrefix(k, p+"/") {
			delete(s.Files, k)
		}
	}
	return nil
}

func (s *Sys) MkdirAll(p string, mode installer.FileMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[p] = &File{Mode: mode, Dir: true, UID: s.Owners[mode.Owner], GID: s.Owners[mode.Group]}
	return nil
}

func (s *Sys) Glob(pattern string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.Files {
		if ok, _ := path.Match(pattern, k); ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s *Sys) Fetch(_ context.Context, url string, _ int64) ([]byte, error) {
	if data, ok := s.Fetched[url]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("download %s: status 404", url)
}

func (s *Sys) HTTP(_ context.Context, p installer.Probe) (installer.ProbeResult, error) {
	if s.HTTPFn != nil {
		return s.HTTPFn(p)
	}
	return installer.ProbeResult{}, fmt.Errorf("dial %s: connection refused", p.URL)
}

func (s *Sys) FreeBytes(string) (uint64, error) { return s.Free, nil }

func (s *Sys) IsRoot() bool { return s.Root }

func (s *Sys) Executable() (string, error) { return s.Exe, nil }

func (s *Sys) Now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Clock = s.Clock.Add(time.Second)
	return s.Clock
}
