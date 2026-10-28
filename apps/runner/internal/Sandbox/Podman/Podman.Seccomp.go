package podman

import _ "embed"

//go:embed Seccomp.json
var seccompProfile []byte

func SeccompProfile() []byte { return seccompProfile }

var BlockedSyscalls = []string{
	"add_key",
	"bpf",
	"fsconfig",
	"fsmount",
	"fsopen",
	"fspick",
	"io_uring_enter",
	"io_uring_register",
	"io_uring_setup",
	"keyctl",
	"mount",
	"mount_setattr",
	"move_mount",
	"open_tree",
	"perf_event_open",
	"pivot_root",
	"ptrace",
	"request_key",
	"setns",
	"umount",
	"umount2",
	"unshare",
	"userfaultfd",
}
