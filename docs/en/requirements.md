[العربية](../requirements.md)

# Hardware requirements

This page says how much CPU, memory and disk Dhow needs, and how those numbers were measured. The installer and `dhow-server doctor` check the same numbers on the host (see [Below the minimum](#below-the-minimum)).

A Dhow install has two parts that size differently:

- **The control plane:** the `dhow-server` process and PostgreSQL 18. One host runs both with the [Compose stack](install/compose.md) or the [installer](install/installer.md).
- **Runners**, which run the jobs in a rootless Podman sandbox, on [hosts of their own](runner/install.md).

## Server and PostgreSQL

| | Minimum | Recommended | Larger installs |
|---|---|---|---|
| **CPU cores** | 1 | 2 | 4 or more |
| **Memory** | 2 GiB | 4 GiB | 8 GiB or more |
| **Free disk** (in `/var/lib`) | 5 GiB | 20 GiB | 50 GiB or more, on SSD |
| **Fits** | About 10 people and a few jobs at once | About 50 people and 20 jobs at once, the heaviest load measured | More than that; consider PostgreSQL on its own host |

The minimum and recommended columns rest on the measurements below. The larger column is an estimate, not a measurement: scale it from the per-job and per-sign-in numbers, and run the [benchmark](#how-the-numbers-were-measured) with your own load.

These numbers are for the whole host, so they leave room for the operating system, Docker and the reverse proxy. Dhow and PostgreSQL themselves used less than this in the [benchmark](#how-the-numbers-were-measured):

| Load | Server memory, peak | PostgreSQL memory, peak | Server CPU, average / peak | PostgreSQL CPU, average / peak |
|---|---|---|---|---|
| Idle after setup | 101 MiB | 25 MiB | 0.00 / 0.04 cores | 0.02 / 0.10 cores |
| Light: 10 people, 3 jobs at once | 161 MiB | 88 MiB | 0.05 / 0.86 cores | 0.05 / 0.27 cores |
| Heavy: 50 people, 20 jobs at once | 201 MiB | 129 MiB | 0.19 / 1.94 cores | 0.13 / 0.40 cores |

The smallest memory limit each load still ran under, without an error or an out-of-memory kill:

| Load | Server alone | PostgreSQL alone | Both together | First failure |
|---|---|---|---|---|
| Idle | 128 MiB | 128 MiB | 320 MiB (192 + 128) | not reached at 128 MiB |
| Light | 192 MiB | 128 MiB | 320 MiB (192 + 128) | server at 128 MiB |
| Heavy | 256 MiB | 192 MiB | 512 MiB (256 + 256) | server at 192 MiB; PostgreSQL at 128 MiB, alone and with the server at 192 MiB |

What the numbers mean:

- **Memory peaks come from sign-ins.** Passwords are hashed with Argon2id, which takes 19 MiB per sign-in and runs as many at once as the host has CPU cores. Fifty people signing in within a few seconds is what pushed the server to its peak and what killed it at 192 MiB. Browsing and job traffic keep it near 30 to 60 MiB.
- **The Compose limits fit the heavy load.** [`compose.yaml`](../../deploy/compose/compose.yaml) limits the server to 256 MiB and PostgreSQL to 512 MiB, and the heavy load passed at 256 MiB with peaks of 193 to 226 MiB. Raise the server limit before you grow past about 50 people.
- **Idle memory depends on the last burst.** The idle server used between 13 and 120 MiB across runs, depending on how much memory the Go runtime had handed back to the system after setup.
- **CPU is idle most of the time.** The averages stay under a quarter of a core even under the heavy load. Short peaks of about two cores during sign-in bursts make responses slower on a one-core host, but no request failed (see [CPU](#cpu)).
- **Why 2 GiB and not less.** Under the heavy load the two containers need up to about 0.5 GiB together. The operating system, Docker or systemd, the reverse proxy, the page cache that keeps PostgreSQL fast, and PostgreSQL's own background work need the rest. The benchmark did not measure a whole host, so the minimum keeps a wide margin: on a 1 GiB host without swap the kernel kills the server or PostgreSQL as soon as several people sign in at once.

### Disk

- **Images:** the server image takes 40 MiB and the PostgreSQL 18 image 614 MiB.
- **Database:** an empty database takes 10 MiB, and the data directory 49 MiB with its write-ahead log (WAL). The WAL alone can grow to about 1 GiB before a checkpoint (PostgreSQL's default `max_wal_size`), so keep at least that much free at all times.
- **Growth:** in the benchmark, a job with one step and about 19 KiB of log added about 40 KiB to the database (logs, attempts and audit events). A team that runs 1,000 such jobs a day adds roughly 40 MiB a day, before compaction of old logs (M4).
- **Logs:** the Compose stack keeps at most 30 MiB of container logs per service.

`dhow-server doctor` fails when less than 1 GiB is free in `/var/lib` or in the PostgreSQL data directory, and warns below 5 GiB.

### CPU

The benchmark ran each container with a limit of 2 cores and then repeated the reference run with less:

| CPU limit per container | Heavy load result | p95 / p99 response time |
|---|---|---|
| 2 cores | passed | 7 / 122 ms |
| 1 core, with the Compose memory limits (256 and 512 MiB) | passed | 6 / 108 ms |
| 0.5 cores | passed | 7 / 394 ms |

So one core for both containers carries the heavy load without errors. A smaller CPU share costs a slower tail: with 0.5 cores each, 1% of the requests under the light load took more than 2 seconds. Sign-ins are the requests that need the CPU (password hashing), so two cores keep a burst of sign-ins short.

## Runners

A runner host needs memory for the runner process and for every job it runs at once (`DHOW_RUNNER_CAPACITY`):

| | Per runner | Per concurrent job |
|---|---|---|
| **Runner process** (measured) | 12 MiB of memory when idle | about 1 MiB more |
| **The job itself** (its limits) | | 1 CPU, 2 GiB of memory and a 4 GiB `/workspace` by default |
| **Disk** | the runner binary and its state directory | a 256 MiB log spill file, plus the job's images in Podman's storage |

- **The job's own limits are the big number.** By default a job may use 1 CPU and 2 GiB of memory, and its `/workspace` is a 4 GiB tmpfs that is charged to memory too, so each concurrent job can take up to 6 GiB. The runner refuses to start if one job's limits are more than the host has. Lower them with `DHOW_RUNNER_MAX_CPU`, `DHOW_RUNNER_MAX_MEMORY` and `DHOW_RUNNER_MAX_DISK`, and size the host as the runner process plus `DHOW_RUNNER_CAPACITY` times the job limits. See [Runner configuration](runner/configuration.md).
- **A small runner:** with capacity 1 and the default limits, a runner host needs about 1 CPU core and 8 GiB of memory (6 GiB for the job, the rest for the system and Podman). With jobs limited to 1 GiB of memory and a 1 GiB workspace, 4 GiB is enough.
- **Not measured yet:** the memory Podman, `conmon` and `crun` use per container, and the image storage per job. The Podman driver needs cgroup v2 with delegation for rootless Podman, and the benchmark host runs cgroup v1, so the runner numbers above come from the runner process with its test-only fake driver, which runs no container. The job numbers are the configured limits, which the sandbox enforces, not measurements.

## Below the minimum

- **The installer refuses a new install.** On a host with fewer cores, less memory or less free disk than the minimum, `dhow-server install` stops before it changes anything and names the resource, the minimum and this page. The kernel keeps a little memory for itself, so a host passes when it reports at least 90% of the minimum (a "2 GB" VPS reports about 1.9 GiB). Below the recommended size it warns and continues. A re-run on a host where Dhow is already installed, which is how upgrades work, only warns, so a security upgrade is never blocked.
- **`dhow-server doctor`** fails below the minimum memory or cores and warns below the recommended size.
- **What breaks.** Below the measured limits the kernel kills the server or PostgreSQL for running out of memory, and every request fails until the container restarts (Compose and systemd restart it, but sign-ins and job updates in flight are lost, and it happens again at the next peak). A server with too little memory dies during sign-in bursts first; PostgreSQL with too little memory died under the heavy load. With too little disk, PostgreSQL stops accepting writes, so sign-ins, new jobs and job logs fail.

The numbers live in one place in the code, [`Requirements.go`](../../apps/server/internal/Core/Installer/Requirements.go).

## How the numbers were measured

- **Date:** 2026-10-28.
- **Versions:** Dhow at commit `be0f272` (the server image built from that commit), PostgreSQL 18.6 (the image pinned in `deploy/compose/compose.yaml`), Docker Engine 29.9.2 with cgroup v1, Linux 6.18 on a 4-core, 16 GiB virtual machine.
- **Stack:** the production [`compose.yaml`](../../deploy/compose/compose.yaml) with the overrides in [`deploy/bench/compose.yaml`](../../deploy/bench/compose.yaml): a memory limit with no swap for each container, 2 CPUs each, no restart, and higher [rate limits](install/configuration.md) so the load is not refused. Each run starts from an empty database.
- **Load:** a Go driver, [`deploy/bench`](../../deploy/bench), uses the real HTTP API. It takes the setup link from the server log, creates the owner, invites 70 members (20 admins who create jobs and 50 developers who browse), and signs every member in with its own session and CSRF token at the start of each phase. Each member uses its own client address through `X-Forwarded-For`, as people behind a reverse proxy would.
  - **Idle:** 30 seconds after setup, with one runner connected.
  - **Light:** 60 seconds. 10 people sign in and then read their account, the organization, members, teams, runners and job lists, one page a second each. 3 admins each create a job, follow it until it ends, read its log and list the jobs, again and again.
  - **Heavy:** 90 seconds. 50 people browse at one page every half second, and 20 admins run jobs as above.
- **Jobs** run end to end through the real runner protocol on `dhow-runner` built with the fake sandbox driver (`-tags dhowtest`): leases, heartbeats and 200 log lines of about 95 bytes over 5 seconds per job. One runner with capacity 3 serves the light load, and two with capacity 10 the heavy load.
- **Measurements:** once a second, the memory (the working set: usage without inactive page cache, as `docker stats` shows it) and CPU time of each container from the Docker Engine API, and the runner's resident memory from `/proc`. After each phase, the database size, out-of-memory events and container state. A phase fails on any failed request, out-of-memory event or stopped container.
- **Limits tried:** the server at 512, 384, 256, 192 and 128 MiB with PostgreSQL at 2 GiB; PostgreSQL at 512, 384, 256, 192 and 128 MiB with the server at 2 GiB; both at 256 and at 384 MiB; the Compose limits (256 and 512 MiB) and 192 + 128 MiB with 1 CPU each; and 2 GiB each with 0.5 CPUs each.

Reproduce it with:

```sh
make bench                                          # builds dhow:dev, then measures it
make bench-run IMAGE=ghcr.io/khaymah/dhow:1.2.3     # measures an image you already have
make bench-run BENCH_ARGS="-configs 2g:2g -baseline deploy/bench/baseline.json"
```

The first run takes about an hour. It writes `bin/bench/results.json` and prints a table. `-configs` takes `server:postgres` memory limits, `-cpus` the CPU limit of each container, and `-baseline` fails the run when a peak is more than 20% over [`baseline.json`](../../deploy/bench/baseline.json), the highest peaks of the published runs. Run it again when a change could move these numbers, and update this page and `Requirements.go` together.
