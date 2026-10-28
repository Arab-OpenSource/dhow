# Develop Dhow on your machine

This page gets you from a fresh clone to a running server, the web app with live reload, and the full test suite. It covers Linux, macOS and Windows (through WSL2), with or without Docker. To try the released image instead, see [Run Dhow locally](install/local.md#the-compose-stack).

> [!WARNING]
> Everything here runs with `DHOW_DEV=true`: plain HTTP and a database connection without TLS. It is for your own machine only. Never set it on a server or anything other people can reach.

## What you need

| Tool | Why | Notes |
|---|---|---|
| Git | The repository | |
| Go | Server, runner, tests | Any recent Go. The `go` command downloads the exact version pinned in `go.mod` by itself. |
| Node.js LTS with pnpm | The web app (`apps/web`) | Run `corepack enable`, then `corepack prepare pnpm@10.28.0 --activate` (the version in `apps/web/package.json`). |
| **PostgreSQL 18** | The server and the database tests | Exactly 18 or newer: the migrations use the built-in `uuidv7()`. Docker or Podman, or a native install, see [below](#postgresql-18). |
| Python 3 | `make check` runs the tests of the Claude Code hooks | Any Python 3. |
| GNU Make and a POSIX shell | The `make` targets | Already there on Linux and macOS, and in WSL2. |

**Windows:** work inside WSL2 with Ubuntu 24.04 or newer, and clone the repository into the Linux file system (`~/src/dhow`), not under `/mnt/c`, which is much slower. Everything on this page then works as on Linux, and the browser on Windows reaches `http://localhost:8080` in WSL2.

## Quick start

```sh
make dev-env
make dev-db
make web
make dev
```

1. `make dev-env` writes `.env.dev` once, with `DHOW_DEV=true`, a fresh secret key, `DHOW_PUBLIC_URL=http://localhost:8080` and the two database URLs. Git ignores it. Keep it: stored secrets need the same key at every start. Delete it only together with the database.
2. `make dev-db` starts PostgreSQL 18 in Docker, or Podman if Docker is missing, on `127.0.0.1:5432`, with the databases `dhow` and `dhow_test`. It is safe to run again. Without Docker or Podman, install PostgreSQL natively instead ([below](#postgresql-18)).
3. `make web` builds the web app into the server. Without it the server still runs, but `/` answers 404. For live reload, skip it and see [The web app with live reload](#the-web-app-with-live-reload).
4. `make dev` starts the server from source. It applies the migrations, logs a `setup_url` line such as `http://localhost:8080/setup#...`, and listens on `127.0.0.1:8080`.

Open the setup link in Chrome or Firefox within an hour to create the owner account. Restart the server for a new link. Check the server from another terminal with `curl -s http://localhost:8080/api/v1/health/ready`.

## PostgreSQL 18

Pick one. They all end with the same user, password and databases, so `.env.dev` works with any of them.

### Docker or Podman

`make dev-db` uses `docker` if it is installed and `podman` otherwise. Choose one with `make dev-db DEV_DB_ENGINE=podman`. If port 5432 is taken, pick another port for both the database and the URLs, before the first `make dev-env`:

```sh
make dev-env DEV_DB_PORT=5433
make dev-db DEV_DB_PORT=5433
```

`make dev-db-reset` deletes the container and its data.

### Native install

**Debian and Ubuntu (and WSL2)**, from the PostgreSQL project's own repository, because the distribution may ship an older version:

```sh
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt install -y postgresql-18
sudo service postgresql start
```

**Fedora, RHEL, Rocky and Alma:** add the repository from [postgresql.org/download/linux/redhat](https://www.postgresql.org/download/linux/redhat/), then install and start `postgresql18-server` as that page shows.

**macOS:** `brew install postgresql@18`, then `brew services start postgresql@18`. [Postgres.app](https://postgresapp.com/) with PostgreSQL 18 works too.

Then create the user and the two databases once. On Linux run `sudo -u postgres psql`, on macOS run `psql postgres`, and enter:

```sql
CREATE ROLE dhow LOGIN PASSWORD 'dhow-dev-only';
CREATE DATABASE dhow OWNER dhow;
CREATE DATABASE dhow_test OWNER dhow;
```

Check it with `psql 'postgres://dhow:dhow-dev-only@127.0.0.1:5432/dhow' -c 'SELECT version()'`. It must say 18 or newer.

### A hosted database

Any PostgreSQL 18 works if you put its URL in `.env.dev` (with `sslmode=require`). It is slower, needs the internet, and the tests create and drop a schema per test, so use a database of its own for `DHOW_TEST_DATABASE_URL`.

## The web app with live reload

Run two terminals:

```sh
make dev-vite
```

```sh
pnpm -C apps/web install
pnpm -C apps/web dev
```

Then open `http://localhost:5173`. Vite serves the web app and forwards `/api` to the server on port 8080. `make dev-vite` starts the server with `DHOW_PUBLIC_URL=http://localhost:5173`, because Dhow compares the browser's `Origin` with it character for character. Open the setup link on port 5173 too.

## Tests and checks

```sh
make dev-check
```

`make dev-check` runs `make check` (lint, workflow lint, the runner protocol checks, every Go test with the race detector, the vulnerability scan and the hooks tests) with the test database from `.env.dev`.

> [!IMPORTANT]
> Use `make dev-check`, not a bare `make check`. Without `DHOW_TEST_DATABASE_URL` the database tests **skip themselves**, so `make check` can pass on your machine while most server tests never ran. CI always sets it.

For one package, load the file first:

```sh
set -a; . ./.env.dev; set +a
go test -race -count=1 ./apps/server/internal/Modules/Session/...
```

For changes to `apps/web`, also run what its CI job runs:

```sh
pnpm -C apps/web lint
pnpm -C apps/web typecheck
pnpm -C apps/web test
pnpm -C apps/web build
pnpm -C apps/web size
```

`make gen` regenerates the code from `packages/openapi/openapi.yaml` and the SQL queries, and `pnpm -C apps/web gen:api` regenerates the web API types. Never edit generated files by hand.

## The runner

What you can do depends on your system:

| | Linux | macOS and Windows |
|---|---|---|
| Runner tests with the fake driver (`make test-runner`) | ✅ | ✅ (Windows in WSL2) |
| Register against your local server | ✅ | ✅ |
| Run jobs in the rootless Podman sandbox | ✅ with the requirements below | In a Linux VM only |

- **Tests.** `make test-runner` builds the runner with the `dhowtest` tag, which adds a fake sandbox driver that replays recorded output and runs no process. Most runner work is covered there.
- **Registration.** Create a registration token as in [Register a runner](runner/register.md), then register with `--dev`, which allows `http://` on `localhost` only, and a state directory of your own:

  ```sh
  DHOW_RUNNER_STATE_DIR="$HOME/.local/state/dhow-runner-dev" \
    go run ./apps/runner/cmd/dhow-runner register --dev --url http://localhost:8080 --token-file -
  ```

- **The sandbox.** Jobs run in rootless Podman from M3.6 on. Until that step ships, `dhow-runner run` refuses to start rather than run jobs without isolation. The sandbox needs Linux with Podman 5.4 or newer, `crun`, `uidmap`, cgroup v2 with the `cpu`, `io`, `memory` and `pids` controllers delegated to the runner (run it under systemd with `Delegate=yes`), at least 65536 sub-ids per job in `/etc/subuid` and `/etc/subgid`, and user namespaces. See [Install a runner](runner/install.md). On macOS and Windows, use a Linux VM with systemd, such as [Lima](https://lima-vm.io/) or Multipass on macOS, or WSL2 with systemd turned on, and run the runner there against the server on your machine.

## Common problems

| Symptom | Cause and fix |
|---|---|
| The migrations fail at the first start | PostgreSQL is older than 18 (no `uuidv7()`). Check `SELECT version()`. |
| `CSRF_FAILED` at setup or sign-in | The address in the browser is not exactly `DHOW_PUBLIC_URL`: `localhost` and `127.0.0.1` differ, and so do ports 8080 and 5173. |
| Sign-in seems to do nothing from another device | The session cookie is `Secure`, and only `localhost` is a secure context over HTTP. Use the machine itself, or HTTPS. See [Run Dhow locally](install/local.md#browsers-and-httplocalhost). |
| `make dev-db` says the port is in use | Another PostgreSQL listens on 5432. Stop it, or use `DEV_DB_PORT` as [above](#docker-or-podman). |
| `/` answers 404 | The web app is not built into the server. Run `make web`, or use the [live reload](#the-web-app-with-live-reload) setup. |
| The setup link no longer works | It is valid for one hour. Restart the server to get a new one. |
| Stored secrets cannot be read after a restart | `DHOW_SECRET_KEY` changed. Keep `.env.dev`, or start over with an empty database. |

## Start over

```sh
make dev-db-reset
rm .env.dev
```

Then follow the [quick start](#quick-start) again.
