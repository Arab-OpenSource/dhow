# Dhow runners

A runner is the `dhow-runner` process that takes jobs from a Dhow server and runs each one in a fresh, isolated sandbox. It only ever connects out to the server, over HTTPS, so it can sit behind a firewall or NAT. The design is in [ADR-0041](../../Plan/ADR/0041-runner-identity-and-registration.md), [ADR-0042](../../Plan/ADR/0042-runner-protocol-and-leasing.md), [ADR-0043](../../Plan/ADR/0043-job-sandbox-rootless-podman.md) and [ADR-0053](../../Plan/ADR/0053-runner-implementation.md).

| Guide | What it covers |
|---|---|
| [Install a runner](install.md) | Building or downloading the binary, the user and directories it needs, and a systemd unit |
| [Register a runner](register.md) | Creating a registration token and turning it into a runner identity |
| [Run a runner](run.md) | Starting it, what it does with each job, and what happens when the network or the runner fails |
| [Runner configuration](configuration.md) | Every `DHOW_RUNNER_*` variable, its default and its limits |

## Status

The runner in this release registers, signs in, takes jobs, fences itself, masks secrets and ships logs. The sandbox driver that actually runs a job's steps in rootless Podman arrives with the next milestone step (M3.6), and the Kubernetes driver after it (M3.11). Until then `dhow-runner run` refuses to start and says so, rather than run jobs without isolation. Test builds (`-tags dhowtest`) carry a fake driver that replays recorded output and runs no process.

## How a runner works

1. **Registration.** An organization owner or admin creates a one-time registration token. `dhow-runner register` generates two key pairs on the runner (Ed25519 for its identity, MLKEM768-X25519 for receiving secrets), sends the public halves with the token, and stores the private halves in its state directory. The private keys never leave the machine.
2. **Sign-in.** The runner signs a fresh server challenge with its private key and receives an access token that lives 15 minutes. It renews it after 10.
3. **Leasing.** The runner asks for work with a long poll of up to 25 seconds. A job comes as a lease grant signed by the server. The runner checks that signature against the server key it pinned at registration, so a compromised proxy cannot inject jobs.
4. **Running.** The runner records the lease in its journal, prepares the sandbox, starts the attempt (receiving the job's secrets encrypted for its key alone), runs each step, sends a heartbeat every 10 seconds, and reports the result.
5. **Self-fencing.** If no heartbeat gets through for 35 seconds, the runner kills the sandbox about 10 seconds before the server would give the job to another runner, so two attempts of one job never run at the same time.

## Security in one paragraph

The runner refuses plain HTTP outside local development, trusts only the system roots or the one CA you give it, never follows redirects, and refuses any grant not signed by the pinned server key. Its identity and journal files must be private to the runner user, or it does not start. Secrets live in memory only, every known encoding of a secret is masked before output is written anywhere (the spill file on disk included), and the journal holds lease and attempt IDs only. The runner turns off core dumps and ptrace on itself.
