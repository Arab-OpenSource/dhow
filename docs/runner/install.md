# Install a runner

The runner is a single static Linux binary, `dhow-runner`, for `amd64` and `arm64`. Each release ships it in a signed archive together with its systemd unit. `get.sh --component runner` downloads and checks that archive and installs both files; the steps after that (the user, registration, start) are still done by hand, as below, until `dhow-runner install` arrives in a later release.

## Requirements

- **Linux** on `amd64` or `arm64`. Windows and macOS runners are not supported.
- **Outbound HTTPS** to your Dhow server, directly or through an HTTP proxy (`HTTPS_PROXY` is honored, and TLS still runs end to end through it).
- **A dedicated user**, `dhow-runner`, that owns nothing else. The runner never needs root.
- **For the Podman driver (M3.6):** Podman 5.4 or newer, `crun`, `uidmap`, cgroup v2 with delegation, and user namespaces. Debian 13, Ubuntu 26.04, Fedora and RHEL 9.6+ (Rocky, Alma) ship these. Debian 12 and Ubuntu 24.04 do not, from their own repositories. Ubuntu 24.04 also blocks user namespaces for programs without an AppArmor profile; set `kernel.apparmor_restrict_unprivileged_userns=0` in `/etc/sysctl.d/` or give Podman a profile, or the runner's start-up checks fail.
- **Sub-ids for the runner user:** at least 65536 per job the runner runs at once in `/etc/subuid` and `/etc/subgid` (each job gets its own user namespace).
- **A host of its own, or a private `/proc`.** Other local users cannot list the runner's state directory, but a container's ID shows in the `conmon` command line, and with it a local user could read the files of the job's image (never `/workspace` or secrets, which live in the container's own tmpfs). Run the runner on a host no one else logs into, or mount `/proc` with `hidepid=2`.

## Get the binary

### With get.sh

Download it, read it, then run it. Never pipe it into a shell:

```sh
curl -fsSLO https://raw.githubusercontent.com/Khaymah/dhow/main/deploy/install/get.sh
less get.sh
bash get.sh --component runner
```

`--version 1.2.3` picks a release; the default is the latest. The script:

1. downloads `dhow-runner_<version>_linux_<arch>.tar.gz`, `SHA256SUMS` and `SHA256SUMS.sigstore.json` from the GitHub release;
2. checks the cosign signature on `SHA256SUMS` against the release workflow of `Khaymah/dhow` (keyless, through Sigstore), when `cosign` is installed, and warns that only the checksum is checked when it is not;
3. checks the archive against its own line in `SHA256SUMS`, and stops if the line is missing or does not match;
4. installs `/usr/local/bin/dhow-runner` (`0755`) and `/etc/systemd/system/dhow-runner.service` (`0644`), and reloads systemd. It does not enable or start the unit.

It does not run `dhow-runner install` yet; it prints the steps below instead. Install `cosign` first to get the signature check: without it, the checksum only proves the download was not damaged, not who built it. From a mirror (`DHOW_DOWNLOAD_URL`) cosign is required, as described in [the installer guide](../install/installer.md). When run as a normal user, the script copies the files into a directory only root can write, and checks and unpacks them there before it installs anything with `sudo`.

### By hand

Download the same three files from the release page, then:

```sh
cosign verify-blob --bundle SHA256SUMS.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/Khaymah/dhow/\.github/workflows/release\.yml@refs/tags/v' \
  SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
tar -xzf dhow-runner_*_linux_amd64.tar.gz
sudo install -m 0755 dhow-runner /usr/local/bin/dhow-runner
sudo install -m 0644 dhow-runner.service /etc/systemd/system/dhow-runner.service
dhow-runner version
```

Each archive also has an SPDX SBOM (`<archive>.spdx.json`) and a SLSA build provenance attestation on the release.

### From source

With the Go version pinned in `go.mod`:

```sh
make build-runner
sudo install -m 0755 bin/dhow-runner /usr/local/bin/dhow-runner
sudo install -m 0644 deploy/systemd/dhow-runner.service /etc/systemd/system/dhow-runner.service
```

## Create the user and directories

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo usermod --add-subuids 1000000-2048575 --add-subgids 1000000-2048575 dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
sudo install -m 0640 -o root -g dhow-runner /dev/null /etc/dhow-runner/runner.env
```

- The sub-id range gives the runner 1048576 ids, enough for 16 jobs at once (65536 each). Pick a range that no other line in `/etc/subuid` and `/etc/subgid` overlaps.
- `/var/lib/dhow-runner` is the state directory. It holds the identity (the private keys), the lease journal and one directory per running attempt. The runner refuses to start if other users can write to it, or if its `identity` or `journal` file is readable by anyone but the runner user.
- `/etc/dhow-runner/runner.env` holds the [configuration](configuration.md). It may stay empty. It has no secrets, but keep it writable by root only. The unit refuses to start without it.

## The systemd unit

The release ships [`deploy/systemd/dhow-runner.service`](../../deploy/systemd/dhow-runner.service). It runs `dhow-runner run` as `dhow-runner`, reads `/etc/dhow-runner/runner.env`, restarts on failure and gives the runner 60 seconds to stop. Do not edit the shipped file; change it with a drop-in (`sudo systemctl edit dhow-runner`), for example to add `ReadWritePaths=` when `DHOW_RUNNER_STATE_DIR` is not `/var/lib/dhow-runner`.

What the unit sets, and why:

- `Delegate=yes` hands the cgroup subtree to the runner so the Podman driver can set per-job limits. `KillMode=control-group` kills every job process with the runner.
- `MemorySwapMax=0` keeps job memory, and the tmpfs that holds secret files, off swap. `LimitCORE=0` keeps keys out of core dumps.
- `StateDirectory=dhow-runner` with `StateDirectoryMode=0700`: systemd's default of `0755` would make the runner refuse to start.
- On `systemctl stop` the runner stops taking work, kills running sandboxes and reports them as lost within 15 seconds, so the server can retry them elsewhere.

### Hardening

`systemd-analyze security dhow-runner.service` rates the unit **5.2 MEDIUM** (systemd 255 and 259), and CI fails if it rises above 6.9. The bar is lower than the server's because the runner is a container manager: rootless Podman needs `newuidmap`, a setuid (or file-capability) helper, and every namespace type. Each directive below was tested with rootless Podman running a job image inside the unit, and CI runs the escape and limits suites inside the same sandbox.

| Directive | Effect |
|---|---|
| `ProtectSystem=strict`, `ReadWritePaths=/var/lib/dhow-runner` | The whole file system is read-only except the state directory. |
| `ProtectHome=yes`, `PrivateTmp=yes` | No home directories; a private `/tmp` and `/var/tmp`. |
| `ProtectProc=invisible` | The runner sees only its own processes and those of its jobs (it owns their user namespaces), not other users' processes. |
| `CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_DAC_OVERRIDE CAP_SYS_ADMIN CAP_SYS_PTRACE` | The runner itself has no capabilities. The bounding set only limits what a setuid program it runs can gain. These five are what the setuid `newuidmap` of Debian and Ubuntu needs: the first four to write the job's id map, and `CAP_SYS_PTRACE` to see the new process when `/proc` hides other users (below, or `hidepid=` on the host). Every other capability, such as `CAP_SYS_MODULE`, `CAP_NET_ADMIN` or `CAP_SYS_RAWIO`, is gone. A job's own user namespace starts with a fresh set, so jobs are not affected. |
| `AmbientCapabilities=` | No inherited capabilities. |
| `PrivateIPC=yes` | Its own System V IPC and POSIX message queues. |
| `ProtectKernelModules=yes`, `ProtectClock=yes` | No module loading, no clock changes. |
| `RestrictRealtime=yes`, `LockPersonality=yes` | No realtime scheduling, no personality changes. |
| `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK` | The same socket families the job sandbox allows. |
| `SystemCallFilter=~@clock @module @obsolete @raw-io @swap`, `SystemCallErrorNumber=EPERM` | Calls that need host capabilities jobs never have, or that are obsolete, return `EPERM` for the runner and everything it starts. |
| `RemoveIPC=yes`, `KeyringMode=private` | No IPC objects left behind; a private kernel keyring. |

Left out, each for a reason:

| Directive | Why it is not set |
|---|---|
| `NoNewPrivileges=yes` | `newuidmap` and `newgidmap` gain their privilege through setuid or file capabilities, which `no_new_privs` blocks, so Podman cannot map the job's sub-ids. Jobs still run with `no-new-privileges` inside their own sandbox. |
| `PrivateUsers=yes` | The unit's own user namespace would map only the runner's user, so the job's 65536 sub-ids could not be mapped. |
| `RestrictNamespaces=` | Podman creates user, mount, PID, network, UTS, IPC and cgroup namespaces for every job. |
| `ProtectKernelTunables=yes`, `ProtectKernelLogs=yes`, `ProtectHostname=yes`, `ProcSubset=pid` | They cover parts of `/proc`, and the kernel then refuses the fresh `/proc` mount that crun makes inside the job (`mount proc: Operation not permitted`). `ProcSubset=pid` also hides `/proc/sys/kernel/cap_last_cap`, which Podman reads. |
| `ProtectControlGroups=yes` | It makes `/sys/fs/cgroup` read-only, and the runner writes the per-job limits into its delegated subtree. |
| `PrivateDevices=yes`, `DevicePolicy=closed` | A private `/dev` and the closed device list apply to every job container too; rootless Podman fails to start a container with either. |
| `MemoryDenyWriteExecute=yes` | Seccomp filters are inherited, so every job would lose writable executable memory: Node.js, the JVM, .NET and other JIT runtimes would crash. |
| `SystemCallArchitectures=native` | Inherited by jobs; it would block the 32-bit x86 and ARM binaries the job sandbox allows. |
| `SystemCallFilter=@system-service` and the deny groups `@mount`, `@privileged`, `@debug`, `@cpu-emulation`, `@reboot`, `@resources`, `@keyring` | Podman and crun need mount, namespace, setuid and keyring calls, and the rest would take away calls jobs are allowed today (`modify_ldt`, `pidfd_getfd`, `sched_setaffinity`, `reboot` inside the job's PID namespace). |
| `RestrictSUIDSGID=yes` | Inherited by Podman when it unpacks images: an image with any setuid file (most distribution images have `su` or `passwd`) fails to pull. |
| `UMask=0077` | Little to gain: the runner already creates its own files `0600` and directories `0700`. The job's user namespace must traverse some of the directories Podman creates, and the full driver has not been tested with a tighter mask. |
| `PrivateNetwork=yes`, `IPAddressDeny=` | The runner must reach the server, and image registries for jobs. |

Next: [register the runner](register.md).
