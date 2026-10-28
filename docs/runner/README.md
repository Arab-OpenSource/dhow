# Dhow runners

A runner is the `dhow-runner` process that takes jobs from a Dhow server and runs each one in a fresh, isolated sandbox. It only ever connects out to the server, over HTTPS, so it can sit behind a firewall or NAT. The design is in [ADR-0041](../../Plan/ADR/0041-runner-identity-and-registration.md), [ADR-0042](../../Plan/ADR/0042-runner-protocol-and-leasing.md), [ADR-0043](../../Plan/ADR/0043-job-sandbox-rootless-podman.md) and [ADR-0053](../../Plan/ADR/0053-runner-implementation.md).

| Guide | What it covers |
|---|---|
| [Install a runner](install.md) | Building or downloading the binary, the user and directories it needs, and a systemd unit |
| [Register a runner](register.md) | Creating a registration token and turning it into a runner identity |
| [Run a runner](run.md) | Starting it, what it does with each job, and what happens when the network or the runner fails |
| [Runner configuration](configuration.md) | Every `DHOW_RUNNER_*` variable, its default and its limits |

## Status

The runner registers, signs in, takes jobs, fences itself, masks secrets and ships logs, and runs each job in rootless Podman at the `container` isolation level (M3.6), with network egress only through its own proxy (M3.7). gVisor (M3.8) and the Kubernetes driver (M3.11) come next. Test builds (`-tags dhowtest`) carry a fake driver that replays recorded output and runs no process.

## The sandbox

Each job gets a fresh container that the runner creates with its own Podman configuration, never the machine's:

- **No privileges.** No capabilities, `no-new-privileges`, an embedded seccomp profile that also blocks `unshare` and `setns`, keyrings, BPF, `io_uring`, `perf_event_open`, `ptrace`, every mount call and every socket family but Unix, IPv4, IPv6 and netlink (so no vsock), and a user namespace of its own with 65536 sub-ids that never maps the runner's user or host root.
- **Nothing from the host.** A read-only root filesystem, no network, private IPC, PID, UTS and cgroup namespaces, `/workspace` and `/tmp` on size-limited tmpfs, and only two read-only mounts: the runner binary as the agent and the control channel. The host's default mounts (such as `/run/secrets` on RHEL) are turned off.
- **Nothing from the image but its files.** The image is pulled from its registry by its full name (short names are refused) and the container is created from the image ID, so no other transport can stand in. The image's environment never reaches the agent, and an image that sets a runtime annotation (`run.oci.*` and the like) is refused.
- **Egress through the runner.** A job with a `network.egress` list gets a proxy on `127.0.0.1:3128` inside the container. The agent carries each connection over a per-attempt Unix socket to the runner. The runner matches the host against the list, resolves the name itself, refuses private, host and metadata addresses, and connects to the address it checked, so DNS rebinding cannot slip through. Each decision shows up once a minute as a `dhow: egress …` line in the job log. A job without a list has no proxy at all.
- **Limits that fail closed.** CPU, memory without swap, processes and disk come from cgroup v2. After `podman create` the runner inspects the container and refuses to run anything if one setting differs from what it asked for.
- **An agent as PID 1.** The runner binary runs inside as the agent: it turns off ptrace on itself, ignores the signals a job could send it, runs each step as `/bin/sh -e -s` with the script on stdin, keeps secret files on a `noexec` tmpfs only for the step, and exits when the runner's control channel closes, so the container dies with the runner. When a step's shell exits, the agent kills every process the step left behind before it reports the step, so nothing started in one step can read the secrets of a later one. `/workspace` is shared, though: a step can still change files a later step runs.

The escape and limits suites (`apps/runner/Test/Isolation`, `make runner-isolation`) run a probe inside a real container and fail if any escape works. CI runs them on every pull request that touches the runner. The decisions are in [ADR-0043](../../Plan/ADR/0043-job-sandbox-rootless-podman.md) and [ADR-0055](../../Plan/ADR/0055-podman-driver-and-agent-implementation.md) and [ADR-0056](../../Plan/ADR/0056-sandbox-hardening-after-the-m3-6-review.md).

## How a runner works

1. **Registration.** An organization owner or admin creates a one-time registration token. `dhow-runner register` generates two key pairs on the runner (Ed25519 for its identity, MLKEM768-X25519 for receiving secrets), sends the public halves with the token, and stores the private halves in its state directory. The private keys never leave the machine.
2. **Sign-in.** The runner signs a fresh server challenge with its private key and receives an access token that lives 15 minutes. It renews it after 10.
3. **Leasing.** The runner asks for work with a long poll of up to 25 seconds. A job comes as a lease grant signed by the server. The runner checks that signature against the server key it pinned at registration, so a compromised proxy cannot inject jobs.
4. **Running.** The runner records the lease in its journal, prepares the sandbox, starts the attempt (receiving the job's secrets encrypted for its key alone), runs each step, sends a heartbeat every 10 seconds, and reports the result.
5. **Self-fencing.** If no heartbeat gets through for 35 seconds, the runner kills the sandbox about 10 seconds before the server would give the job to another runner, so two attempts of one job never run at the same time.

## Security in one paragraph

The runner refuses plain HTTP outside local development, trusts only the system roots or the one CA you give it, never follows redirects, and refuses any grant not signed by the pinned server key. Its identity and journal files must be private to the runner user, or it does not start. Secrets live in memory only, every known encoding of a secret is masked before output is written anywhere (the spill file on disk included), and the journal holds lease and attempt IDs only. The runner turns off core dumps and ptrace on itself.
