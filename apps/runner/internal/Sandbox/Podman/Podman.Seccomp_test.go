package podman

import (
	"encoding/json"
	"slices"
	"testing"
)

type seccompArg struct {
	Index    int    `json:"index"`
	Value    uint64 `json:"value"`
	ValueTwo uint64 `json:"valueTwo"`
	Op       string `json:"op"`
}

type seccompRule struct {
	Names    []string     `json:"names"`
	Action   string       `json:"action"`
	ErrnoRet *int         `json:"errnoRet"`
	Args     []seccompArg `json:"args"`
}

type seccompDoc struct {
	DefaultAction   string        `json:"defaultAction"`
	DefaultErrnoRet int           `json:"defaultErrnoRet"`
	Syscalls        []seccompRule `json:"syscalls"`
}

const (
	actErrno = "SCMP_ACT_ERRNO"
	actAllow = "SCMP_ACT_ALLOW"
	eperm    = 1
	enosys   = 38
)

var adr0043Blocked = []string{
	"add_key", "bpf", "io_uring_enter", "io_uring_register", "io_uring_setup", "keyctl", "mount", "perf_event_open",
	"pivot_root", "ptrace", "request_key", "setns", "umount2", "unshare", "userfaultfd",
}

func loadProfile(t *testing.T) seccompDoc {
	t.Helper()
	var doc seccompDoc
	if err := json.Unmarshal(seccompProfile, &doc); err != nil {
		t.Fatalf("the embedded seccomp profile does not parse: %v", err)
	}
	return doc
}

func TestTheDefaultActionDeniesWithENOSYS(t *testing.T) {
	doc := loadProfile(t)
	if doc.DefaultAction != actErrno || doc.DefaultErrnoRet != enosys {
		t.Fatalf("the default action must be ENOSYS, got %s/%d", doc.DefaultAction, doc.DefaultErrnoRet)
	}
}

func TestEverySyscallADR0043RequiresBlockedIsBlocked(t *testing.T) {
	blocked := blockedWithEPERM(t)
	for _, name := range adr0043Blocked {
		if !slices.Contains(blocked, name) {
			t.Errorf("ADR-0043 requires %s to be blocked, but the profile does not block it with EPERM", name)
		}
	}
}

func TestExtraMountSyscallsBeyondADR0043AreAlsoBlocked(t *testing.T) {
	blocked := blockedWithEPERM(t)
	for _, name := range []string{"fsopen", "fsmount", "fsconfig", "fspick", "move_mount", "open_tree", "mount_setattr", "umount"} {
		if !slices.Contains(blocked, name) {
			t.Errorf("ADR-0055 also blocks %s so the new mount API cannot stand in for mount()", name)
		}
	}
}

func TestTheBlockedListMatchesTheEmbeddedProfile(t *testing.T) {
	doc := loadProfile(t)
	var dedicated []string
	for _, r := range doc.Syscalls {
		if r.Action == actErrno && r.ErrnoRet != nil && *r.ErrnoRet == eperm && len(r.Args) == 0 &&
			slices.Contains(r.Names, "ptrace") && slices.Contains(r.Names, "bpf") {
			dedicated = r.Names
		}
	}
	want := append([]string(nil), BlockedSyscalls...)
	got := append([]string(nil), dedicated...)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("BlockedSyscalls (%v) does not match the dedicated EPERM rule (%v)", want, got)
	}
	for _, name := range adr0043Blocked {
		if !slices.Contains(BlockedSyscalls, name) {
			t.Errorf("BlockedSyscalls is missing the ADR-0043 syscall %s", name)
		}
	}
}

func TestClone3ReturnsENOSYSAndCloneIsNamespaceRestricted(t *testing.T) {
	doc := loadProfile(t)
	var clone3ok, cloneMasked bool
	for _, r := range doc.Syscalls {
		if slices.Contains(r.Names, "clone3") && r.Action == actErrno && r.ErrnoRet != nil && *r.ErrnoRet == enosys {
			clone3ok = true
		}
		if slices.Contains(r.Names, "clone") && r.Action == actAllow {
			for _, a := range r.Args {
				if a.Op == "SCMP_CMP_MASKED_EQ" && a.Value != 0 {
					cloneMasked = true
				}
			}
		}
	}
	if !clone3ok {
		t.Error("clone3 must return ENOSYS so libc falls back to the filterable clone")
	}
	if !cloneMasked {
		t.Error("clone must be allowed only when it creates no new namespace (a masked-equal arg)")
	}
}

func TestNoAllowRuleReopensABlockedSyscall(t *testing.T) {
	doc := loadProfile(t)
	forbidden := append([]string(nil), BlockedSyscalls...)
	forbidden = append(forbidden, "clone3")
	for _, r := range doc.Syscalls {
		if r.Action != actAllow || len(r.Args) > 0 {
			continue
		}
		for _, name := range r.Names {
			if slices.Contains(forbidden, name) {
				t.Errorf("an unconditional allow rule reopens the blocked syscall %s", name)
			}
		}
	}
}

func blockedWithEPERM(t *testing.T) []string {
	doc := loadProfile(t)
	var out []string
	for _, r := range doc.Syscalls {
		if r.Action == actErrno && r.ErrnoRet != nil && *r.ErrnoRet == eperm && len(r.Args) == 0 {
			out = append(out, r.Names...)
		}
	}
	return out
}
