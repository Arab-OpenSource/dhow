# Install a runner

The runner is a single static Linux binary, `dhow-runner`, for `amd64` and `arm64`. A packaged install with the `runner` mode of the interactive installer comes with M3.10. Until then, set it up by hand as below.

## Requirements

- **Linux** on `amd64` or `arm64`. Windows and macOS runners are not supported.
- **Outbound HTTPS** to your Dhow server, directly or through an HTTP proxy (`HTTPS_PROXY` is honored, and TLS still runs end to end through it).
- **A dedicated user**, `dhow-runner`, that owns nothing else. The runner never needs root.
- **For the Podman driver (M3.6):** Podman 5.4 or newer, `crun`, `uidmap`, cgroup v2 with delegation, and user namespaces. Debian 13, Ubuntu 26.04, Fedora and RHEL 9.6+ (Rocky, Alma) ship these. Debian 12 and Ubuntu 24.04 do not, from their own repositories. Ubuntu 24.04 also blocks user namespaces for programs without an AppArmor profile; set `kernel.apparmor_restrict_unprivileged_userns=0` in `/etc/sysctl.d/` or give Podman a profile, or the runner's start-up checks fail.
- **Sub-ids for the runner user:** at least 65536 per job the runner runs at once in `/etc/subuid` and `/etc/subgid` (each job gets its own user namespace).

## Get the binary

Build it from a checkout of the repository with the Go version pinned in `go.mod`:

```sh
make build-runner
sudo install -m 0755 bin/dhow-runner /usr/local/bin/dhow-runner
dhow-runner version
```

## Create the user and directories

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
```

- `/var/lib/dhow-runner` is the state directory. It holds the identity (the private keys), the lease journal and one directory per running attempt. The runner refuses to start if other users can write to it, or if its `identity` or `journal` file is readable by anyone but the runner user.
- `/etc/dhow-runner/runner.env` holds the [configuration](configuration.md). It has no secrets, but keep it writable by root only.

## A systemd unit

```ini
[Unit]
Description=Dhow runner
Wants=network-online.target
After=network-online.target

[Service]
User=dhow-runner
Group=dhow-runner
EnvironmentFile=/etc/dhow-runner/runner.env
ExecStart=/usr/local/bin/dhow-runner run
Restart=on-failure
RestartSec=10s
StateDirectory=dhow-runner
StateDirectoryMode=0700
Delegate=yes
MemorySwapMax=0
LimitCORE=0
TimeoutStopSec=60s

[Install]
WantedBy=multi-user.target
```

- `StateDirectoryMode=0700` matters: systemd's default of `0755` makes the runner refuse to start.
- `MemorySwapMax=0` keeps job memory, and the tmpfs that holds secret files, off swap. `LimitCORE=0` keeps keys out of core dumps.
- `Delegate=yes` hands the cgroup subtree to the runner so the Podman driver can set per-job limits.
- On `systemctl stop` the runner stops taking work, kills running sandboxes and reports them as lost within 15 seconds, so the server can retry them elsewhere.

Next: [register the runner](register.md).
