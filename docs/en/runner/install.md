[العربية](../../runner/install.md)

# Install a runner

The runner is a single static Linux binary, `dhow-runner`, for `amd64` and `arm64`. Each release ships it in a signed archive together with its systemd unit. `get.sh --component runner` downloads and checks that archive and installs both files. Then `sudo dhow-runner install` does the rest: it checks the host, creates the runner user, its sub-ids and directories, registers the runner, keeps its identity encrypted with `systemd-creds` when the host can, and starts it.

## Requirements

- **Linux** on `amd64` or `arm64`, with systemd. Windows and macOS runners are not supported.
- **Outbound HTTPS** to your Dhow server, directly or through an HTTP proxy (`HTTPS_PROXY` is honored, and TLS still runs end to end through it).
- **A registration token** from the **Runners** page of your server ([Register a runner](register.md#1-create-a-registration-token)).
- **For the Podman driver (M3.6):** Podman 5.4 or newer, `crun`, `uidmap`, cgroup v2 with delegation, and user namespaces. Debian 13, Ubuntu 26.04, Fedora and RHEL 9.6+ or 10 (Rocky, Alma) ship these. Debian 12 and Ubuntu 24.04 do not, from their own repositories, and `install` stops on them with the version it found. Ubuntu also blocks user namespaces for programs without an AppArmor profile; set `kernel.apparmor_restrict_unprivileged_userns=0` in `/etc/sysctl.d/` or give Podman a profile, or `install` stops and says so.
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

At the end it tells you to run `sudo dhow-runner install`. Install `cosign` first to get the signature check: without it, the checksum only proves the download was not damaged, not who built it. From a mirror (`DHOW_DOWNLOAD_URL`) cosign is required, as described in [the installer guide](../install/installer.md). When run as a normal user, the script copies the files into a directory only root can write, and checks and unpacks them there before it installs anything with `sudo`.

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

## Install with dhow-runner install

```sh
sudo dhow-runner install
```

It shows what it found on the host, asks for the server address, the registration token (hidden while you type), the runner name, where to keep the private keys and how many jobs may run at once, prints the full plan, and asks once before it changes anything. `--dry-run` stops after the plan, `--yes` skips the final question, and `--lang ar` or `--lang en` picks the language (the default follows the locale).

### What it does

Every step checks first whether it is already done, so running `install` again changes only what is missing:

1. **Checks** Podman 5.4+, `crun`, `newuidmap`, cgroup v2, user namespaces and the AppArmor switch, and stops with a clear reason if one is missing. With `install_packages: true` it installs `podman`, `crun` and `uidmap` (or `shadow-utils`) from the distribution first, then checks the Podman version again.
2. **Creates the system user** `dhow-runner`, without a login shell, with `/var/lib/dhow-runner` as its home.
3. **Gives it 1048576 sub-ids** (16 jobs at once, 65536 each) in `/etc/subuid` and `/etc/subgid`, from the first free range at or above 1048576.
4. **Creates** `/var/lib/dhow-runner` (`0700 dhow-runner`; the Podman driver opens it to `0711` when it starts) and `/etc/dhow-runner` (`0750 root:dhow-runner`).
5. **Writes** `/etc/dhow-runner/runner.env` (`0640 root:dhow-runner`) with the server address, origin, CA file, driver, capacity and any extra [settings](configuration.md). Lines you added by hand that the answers do not manage are kept.
6. **Installs** the binary in `/usr/local/bin/dhow-runner` and the unit in `/etc/systemd/system/dhow-runner.service`.
7. **Registers** the runner with the token. The token is read from the file, standard input or the hidden prompt, and only ever kept in memory: it is never put on a command line, in the environment, in a log or on disk.
8. **Keeps the identity encrypted** with `systemd-creds` in `/var/lib/dhow-runner/identity.cred`, and adds the drop-in `/etc/systemd/system/dhow-runner.service.d/10-dhow-identity.conf` that hands it to the service ([the encrypted identity](keys.md#the-encrypted-identity)). When `systemd-creds` cannot encrypt on the host (no TPM2 and no usable host key), or you chose `identity: file`, the identity stays in `/var/lib/dhow-runner/identity` with mode `0600`, and the plan and the result say so. A plain identity left by a manual setup is encrypted in place.
9. **Saves the answers**, without the token, in `/etc/dhow-runner/answers.yaml` (`0600 root`).
10. **Enables and starts** the unit, then waits up to two minutes for the runner to sign in and ask for work. If it does not, it prints the reason and the last lines of the service log.

### Unattended: the answers file

`--answers <file>` reads every answer from a YAML file and asks nothing:

```yaml
url: https://dhow.example.com
token_file: /root/dhow-runner-token
name: build-01
identity: auto
capacity: 2
install_packages: true
env:
  DHOW_RUNNER_MAX_MEMORY: 4GiB
```

```sh
sudo dhow-runner install --answers runner.yaml
```

| Key | Default | Meaning |
|---|---|---|
| `url` | (required) | The server address, as for `DHOW_RUNNER_URL`. |
| `token_file` | (required for a first install) | A file that holds the registration token, or `-` to read it from standard input. It is not needed once the runner is registered, and it is never saved. |
| `name` | the host name | The name shown in Dhow. |
| `server_origin` | the origin of `url` | `DHOW_RUNNER_SERVER_ORIGIN`, for a separate runner listener. |
| `ca_file` | system roots | `DHOW_RUNNER_CA_FILE`. |
| `driver` | `podman` | The sandbox driver. Test builds also accept `fake`. |
| `capacity` | `1` | `DHOW_RUNNER_CAPACITY`, from 1 to 16. |
| `identity` | `auto` | `encrypted` (needs `systemd-creds`), `file`, or `auto`, which encrypts when the host can. |
| `install_packages` | `false` | Install missing packages from the distribution. |
| `env` | none | Other `DHOW_RUNNER_*` settings for `runner.env`. |
| `dev` | `false` | Allows plain `http://` to a loopback address, for local development only. |
| `lang` | the locale | `en` or `ar`. |

Unknown keys, a second YAML document or a file over 64 KiB are refused. `--token` on the command line is refused too. Under root the answers file must belong to root, be writable by root only and sit in a directory others cannot write to (or a sticky one like `/tmp`); neither it nor the token file may be a symlink, the token file is refused when others can write to it, and you get a warning when others can read it. Values written to `runner.env` may not hold control characters, quotes or backslashes, and `env` accepts only the settings listed in [Runner configuration](configuration.md).

## Check the runner

```sh
sudo dhow-runner doctor
```

It checks the sandbox prerequisites, the unit and its hardening, the files and their permissions, where the identity is kept, a rotation left unfinished, and whether the server answers and the runner signs in. It exits non-zero when something is wrong, and `--fix` applies the fixes it proposes. See [Check a runner with doctor](doctor.md).

## Remove the runner

```sh
sudo dhow-runner uninstall
```

It stops and disables the unit, then revokes the runner on the server: the runner signs a revoke request with its own key, and the server ends its sessions and records `runner.revoked`. If the runner cannot reach the server or sign in, it prints the runner ID and tells you to revoke it in the dashboard. Then it removes the unit and its drop-in, `/var/lib/dhow-runner` (the identity, the credential and the journal), `/etc/dhow-runner` and the binary. The `dhow-runner` user and its sub-ids stay, unless you add `--remove-user`, which deletes the user only if `install` created it (its comment says so); a user that existed before is kept, and `install` says when it adopts one. Running it again finds nothing to remove. `--yes` skips the question.

## Set up by hand

`install` does all of this for you. To do it yourself instead:

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo usermod --add-subuids 1048576-2097151 --add-subgids 1048576-2097151 dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
sudo install -m 0640 -o root -g dhow-runner /dev/null /etc/dhow-runner/runner.env
```

- The sub-id range gives the runner 1048576 ids, enough for 16 jobs at once (65536 each). Pick a range that no other line in `/etc/subuid` and `/etc/subgid` overlaps.
- `/var/lib/dhow-runner` is the state directory. It holds the identity (the private keys), the lease journal and one directory per running attempt. The runner refuses to start if other users can write to it, or if its `identity` or `journal` file is readable by anyone but the runner user.
- `/etc/dhow-runner/runner.env` holds the [configuration](configuration.md). It may stay empty. It has no secrets, but keep it writable by root only. The unit refuses to start without it.

Then [register the runner](register.md#2-register-on-the-runner-host) and start it with `sudo systemctl enable --now dhow-runner`. A manual registration keeps the identity in a plain `0600` file; running `sudo dhow-runner install` later encrypts it.

## The systemd unit

The release ships [`deploy/systemd/dhow-runner.service`](../../../deploy/systemd/dhow-runner.service). It runs `dhow-runner run` as `dhow-runner`, reads `/etc/dhow-runner/runner.env`, restarts on failure and gives the runner 60 seconds to stop. Do not edit the shipped file; change it with a drop-in (`sudo systemctl edit dhow-runner`), for example to add `ReadWritePaths=` when `DHOW_RUNNER_STATE_DIR` is not `/var/lib/dhow-runner`.

What the unit sets, and why:

- `Delegate=yes` hands the cgroup subtree to the runner so the Podman driver can set per-job limits. `KillMode=control-group` kills every job process with the runner.
- `MemorySwapMax=0` keeps job memory, and the tmpfs that holds secret files, off swap. `LimitCORE=0` keeps keys out of core dumps.
- `StateDirectory=dhow-runner` with `StateDirectoryMode=0700`: systemd's default of `0755` would make the runner refuse to start.
- On `systemctl stop` the runner stops taking work, kills running sandboxes and reports them as lost within 15 seconds, so the server can retry them elsewhere.

### Hardening

`systemd-analyze security dhow-runner.service` rates the unit **5.2 MEDIUM** (systemd 255, 257 and 259), and CI fails if it rises above 6.9. The bar is lower than the server's because the runner is a container manager: rootless Podman needs `newuidmap`, a setuid (or file-capability) helper, and every namespace type. Each directive below was tested with rootless Podman running a job image inside the unit, and CI runs the escape and limits suites inside the same sandbox.

| Directive | Effect |
|---|---|
| `ProtectSystem=strict`, `ReadWritePaths=/var/lib/dhow-runner` | The whole file system is read-only except the state directory. |
| `InaccessiblePaths=` `sudo`, `su`, `pkexec`, `passwd`, `chsh`, `chfn`, `newgrp`, `gpasswd`, `mount`, `umount` | The setuid programs the runner never needs are hidden, so none of them can use the bounding set. Podman's own helpers, `newuidmap` and `newgidmap`, stay; crun and the native overlay driver need neither `mount` nor `fusermount3`. |
| `ProtectHome=yes`, `PrivateTmp=yes` | No home directories; a private `/tmp` and `/var/tmp`. |
| `ProtectProc=invisible` | The runner sees only its own processes and those of its jobs (it owns their user namespaces), not other users' processes. |
| `CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_DAC_OVERRIDE CAP_SYS_ADMIN CAP_SYS_PTRACE` | The runner itself has no capabilities. The bounding set only limits what a setuid program it runs can gain. These five are what the setuid `newuidmap` of Debian and Ubuntu needs; dropping any one of `CAP_SYS_ADMIN`, `CAP_DAC_OVERRIDE` or `CAP_SYS_PTRACE` was tried, and Podman then cannot set up the job's user namespace. `CAP_SETUID` and `CAP_SETGID` write the id maps, `CAP_DAC_OVERRIDE` opens the new process's `uid_map` as root, `CAP_SYS_ADMIN` is checked by the kernel when a map is written for another process, and `CAP_SYS_PTRACE` lets it see that process while `/proc` hides other users (below, or `hidepid=` on the host). On Fedora and RHEL, where `newuidmap` has file capabilities instead of setuid, only the first two are used. Every other capability, such as `CAP_SYS_MODULE`, `CAP_NET_ADMIN` or `CAP_SYS_RAWIO`, is gone. A job's own user namespace starts with a fresh set, so jobs are not affected. |
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

Next: [check the runner](doctor.md), and [run it](run.md).
