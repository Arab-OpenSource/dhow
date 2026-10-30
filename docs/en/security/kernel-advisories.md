[العربية](../../security/kernel-advisories.md)

# Kernel security advisories

Dhow runs on your Linux host, so a flaw in the host's kernel can reach it. This page lists the kernel advisories that affect Dhow, what Dhow changed for each, and what you must do on the host. `dhow-server doctor` and `dhow-runner doctor` check the host for the advisories that a setting can show.

## CVE-2026-98432: multipath TCP (MPTCP)

### What it is

CVE-2026-98432 is a use-after-free race in the Linux kernel's IPv4 TCP code, rated CVSS 9.8, and present since kernel 4.4. The public report says a remote attacker without an account can get root on a server that has a multipath TCP (MPTCP) listener.

Since Go 1.24, a Go program that listens on TCP opens an MPTCP listener when the kernel enables MPTCP (`net.mptcp.enabled=1`, the default on most distributions). Dhow is built with Go, so the server's HTTP listener, which also serves the runners, and the runner's TCP listeners, such as the job egress proxy, were MPTCP listeners on those hosts.

The upstream kernel releases that fix it:

| Kernel line | First fixed release |
|---|---|
| 5.10 | 5.10.272 |
| 5.15 | 5.15.223 |
| 6.1 | 6.1.190 |
| 6.6 | 6.6.159 |
| 6.12 | 6.12.113 |
| 6.18 | 6.18.56 |
| 7.2 | 7.2.10 |
| 7.3 | 7.3 |

Distributions backport fixes into their own kernel versions, so the version number alone does not tell you whether your kernel is fixed. Your distribution's advisory does.

### What Dhow changed

Every Dhow binary now turns MPTCP off: `dhow-server`, `dhow-runner` (and the job agent, which is the same binary) and the `dhow` CLI. The setting is `godebug multipathtcp=0` in the project's `go.mod`, so it is built into each binary, and every listener and outgoing connection uses plain TCP. The decision is in [ADR-0073](../../../Plan/ADR/0073-disable-mptcp-in-all-binaries.md).

This protects Dhow's own listeners only. Other services on the same host, such as the reverse proxy, PostgreSQL, SSH or another Go program, may still open MPTCP listeners. The real fix is the kernel update.

Do not set `GODEBUG=multipathtcp=1` or `GODEBUG=multipathtcp=2` in the environment of a Dhow service: the environment wins over the built-in value and turns MPTCP back on.

If Go is installed, you can see the setting in a binary:

```sh
go version -m /usr/local/bin/dhow-server | grep DefaultGODEBUG
```

It prints `DefaultGODEBUG=multipathtcp=0`.

### Check your host

```sh
uname -r
sysctl net.mptcp.enabled
```

- `net.mptcp.enabled = 0`, or `sysctl` says the key does not exist: MPTCP is off or not built in, and no listener on the host can use it.
- `net.mptcp.enabled = 1`: MPTCP is on. Find your kernel release in your distribution's advisory for CVE-2026-98432, for example the Debian security tracker, the Ubuntu CVE tracker or the Red Hat CVE database. If your kernel is not fixed yet, apply the mitigation below.

`doctor` shows a warning when MPTCP is on. It prints the kernel release but never decides from it, because of the backports. The warning does not change the exit status, and Dhow never changes this setting for you, because it affects every service on the host.

### Mitigate until the kernel update arrives

Turn MPTCP off now, and keep it off after a reboot:

```sh
sudo sysctl -w net.mptcp.enabled=0
echo 'net.mptcp.enabled = 0' | sudo tee /etc/sysctl.d/90-disable-mptcp.conf
```

Then check that it holds:

```sh
sysctl net.mptcp.enabled
```

New listeners then use plain TCP. A service that was already listening keeps its MPTCP socket until it restarts, so restart the services on the host, or reboot.

Turning MPTCP off breaks nothing in Dhow. Only a service that you set up to use MPTCP on purpose loses it.

### The fix

The fix is the kernel update from your distribution. Install it, reboot into the new kernel, and check with `uname -r` that the new kernel is running. After that you may remove `/etc/sysctl.d/90-disable-mptcp.conf` if you need MPTCP for something else. Dhow keeps MPTCP off either way.
