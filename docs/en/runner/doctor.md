[العربية](../../runner/doctor.md)

# Check a runner with doctor

`dhow-runner doctor` checks a runner that `dhow-runner install` set up, and tells you what to fix. It needs root, changes nothing unless you ask, and never prints the registration token, the keys or the identity.

```sh
sudo dhow-runner doctor
sudo dhow-runner doctor --fix
```

On a terminal it offers each proposed fix in turn. `--fix` applies them all without asking, and `--lang ar` or `--lang en` picks the language.

## What it checks

| Check | Fails when | What fixes it |
|---|---|---|
| Runner installed | The binary, the unit, `runner.env`, the state directory or the `dhow-runner` user is missing. | `sudo dhow-runner install` |
| Podman 5.4 or newer, crun and newuidmap | Podman is missing or older than 5.4, or `crun`, `newuidmap` or `newgidmap` is missing. Skipped for the test driver. | A newer Podman from the distribution. |
| cgroup v2 and user namespaces | The kernel uses cgroup v1, user namespaces are off, or AppArmor blocks them. | The kernel settings named in the message. |
| SELinux is not enforcing | SELinux is in enforcing mode, which jobs do not support yet ([Supported hosts](install.md#supported-hosts)). | A supported host. Switching to permissive is your own choice, and we do not recommend it. |
| Sub-ids | The runner user has fewer than 65536 sub-ids per job in `/etc/subuid` or `/etc/subgid`. | `--fix` adds a free range of 1048576. |
| Memory for each job | `DHOW_RUNNER_MAX_MEMORY` plus the tmpfs disk `DHOW_RUNNER_MAX_DISK` is more than the host memory. It is the same check `install` makes before step 1 ([Memory per job](install.md#memory-per-job)). | Lower both in `runner.env`; the message suggests values that fit. |
| The systemd unit matches this release | The installed unit differs from the one in the binary (a warning). | `--fix` writes it again and restarts the runner. |
| The service is enabled and running | The unit is stopped or disabled. | `--fix` enables and restarts it. |
| Unit hardening | `systemd-analyze security` rates the unit above 6.9. | Put the shipped unit back and remove drop-ins that loosen it. |
| File ownership and permissions | A file or directory has another owner or mode than `install` gives it, or a state file is a symlink or a special file. | `--fix` sets owner and mode back through a descriptor that never follows a link. A symlink or special file is never fixed: remove it by hand, and treat the runner keys as exposed. |
| Identity storage | The identity is encrypted but its drop-in is missing, both a plain and an encrypted copy exist, or the runner has no identity. A plain `0600` identity is reported as information, with the reason. | `--fix` writes the drop-in again. |
| No unfinished key rotation | `identity.pending` or `identity.pending.cred` is left from a rotation (a warning). | Stop the runner and run `sudo dhow-runner rotate-keys`. |
| The server answers | The server cannot be reached, or its certificate is not trusted. | The address, DNS, the firewall or `DHOW_RUNNER_CA_FILE`. |
| The runner signs in | The server refuses the runner, or the service has not signed in since it started. | The message and the service log say why. |

The state directory may be `0700` or `0711`. The Podman driver opens it to `0711` on every start (ADR-0056), so the job's user namespace can pass through it to reach its mounts and control socket; others can enter it but not list or read it. `doctor` accepts both, `--fix` never turns `0711` back into `0700`, and everything inside keeps its strict mode. The owner must still be `dhow-runner`, with no read or write bit for group or others.

## How it checks the sign-in

A runner has one session at a time. If `doctor` signed in next to the running service, it would replace the service's session, and three replacements in ten minutes pause the runner as a suspected copy ([Copying a runner](register.md#copying-a-runner)). So while the service runs, `doctor` asks the server for a sign-in challenge only, which proves the server answers without opening a session, and reads the sign-in from the service log since its last start. When the service is stopped, `doctor` signs in itself.

## Output and exit status

```
  ok    Runner installed
  ok    Podman 5.4.0 or newer, crun and newuidmap
        Podman 5.4.2
  ok    Unit hardening (systemd-analyze security at most 6.9)
        5.2 MEDIUM
  ok    Identity storage
        The identity is encrypted with systemd-creds in /var/lib/dhow-runner/identity.cred; only this host can open it. (yes, with the host key)
  ok    The runner signs in
        Runner 0190c8a2-… signed in when the service started (from its log, so the running service keeps its session).

Everything looks good.
```

`doctor` exits with status 0 when everything is fine, and 1 when a warning or a failure is left, so scripts and monitoring can call it.
