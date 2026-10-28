//go:build linux

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const agentPID = 1

func syscallProbes() []result {
	return []result{
		newUserNamespace(),
		blockedUnless("mount a tmpfs", unix.Mount("none", "/tmp", "tmpfs", 0, ""), "mount"),
		keyring(),
		bpf(),
		ioURing(),
		perfEvent(),
		ptraceAgent(),
		rawSocket("raw IPv4 socket", unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_ICMP),
		rawSocket("packet socket", unix.AF_PACKET, unix.SOCK_RAW, 0),
		blockedUnless("load a kernel module", unix.InitModule([]byte{0}, ""), "init_module"),
	}
}

func newUserNamespace() result {
	self, err := os.Executable()
	if err != nil {
		return escaped("new user namespace", "cannot find the probe: %v", err)
	}
	pid, err := syscall.ForkExec(self, []string{self, "sleep", "1"}, &syscall.ProcAttr{ //nolint:gosec // The probe exists to try this from inside the sandbox.
		Sys: &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUSER},
	})
	if err != nil {
		return blocked("new user namespace")
	}
	var ws syscall.WaitStatus
	_, _ = syscall.Wait4(pid, &ws, 0, nil)
	return escaped("new user namespace", "clone(CLONE_NEWUSER) succeeded")
}

func keyring() result {
	_, err := unix.AddKey("user", "dhow-probe", []byte("x"), unix.KEY_SPEC_PROCESS_KEYRING)
	return blockedUnless("add a key to a keyring", err, "add_key")
}

func bpf() result {
	attr := make([]byte, 128)
	fd, _, errno := unix.Syscall(unix.SYS_BPF, 0, uintptr(unsafe.Pointer(&attr[0])), uintptr(len(attr))) //nolint:gosec // The probe exists to try this from inside the sandbox.
	if errno != 0 {
		return blocked("bpf")
	}
	_ = unix.Close(int(fd))
	return escaped("bpf", "bpf(BPF_MAP_CREATE) returned a descriptor")
}

func ioURing() result {
	params := make([]byte, 120)
	fd, _, errno := unix.Syscall(unix.SYS_IO_URING_SETUP, 1, uintptr(unsafe.Pointer(&params[0])), 0) //nolint:gosec // The probe exists to try this from inside the sandbox.
	if errno != 0 {
		return blocked("io_uring_setup")
	}
	_ = unix.Close(int(fd))
	return escaped("io_uring_setup", "io_uring_setup returned a descriptor")
}

func perfEvent() result {
	attr := unix.PerfEventAttr{Type: unix.PERF_TYPE_SOFTWARE, Config: unix.PERF_COUNT_SW_CPU_CLOCK}
	attr.Size = uint32(unsafe.Sizeof(attr))
	fd, err := unix.PerfEventOpen(&attr, 0, -1, -1, 0)
	if err != nil {
		return blocked("perf_event_open")
	}
	_ = unix.Close(fd)
	return escaped("perf_event_open", "perf_event_open returned a descriptor")
}

func ptraceAgent() result {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := unix.PtraceAttach(agentPID); err != nil {
		return blocked("ptrace the agent")
	}
	_ = unix.PtraceDetach(agentPID)
	return escaped("ptrace the agent", "attached to PID 1")
}

func rawSocket(name string, domain, typ, proto int) result {
	fd, err := unix.Socket(domain, typ, proto)
	if err != nil {
		return blocked(name)
	}
	_ = unix.Close(fd)
	return escaped(name, "socket(%d, %d, %d) succeeded", domain, typ, proto)
}
