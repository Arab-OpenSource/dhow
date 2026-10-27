# Run Dhow locally

This page is for trying Dhow out or working on it on your own computer. To install it for other people, use [Docker Compose](compose.md) behind a [reverse proxy](reverse-proxy.md).

> [!WARNING]
> **`DHOW_DEV=true` is for your own machine only.** It lets Dhow start with an `http://` public URL and a database connection without TLS, so passwords, session cookies and every secret cross the network in plain text. Dhow logs `DHOW_DEV=true: plain HTTP and unencrypted database connections are allowed. Never use this in production.` at every start in this mode. Never set it on a server, a shared machine, or anything other people can reach, and never copy a development `.env` to production.

There are two ways to run Dhow locally:

| | [From source](#from-source) | [The Compose stack](#the-compose-stack) |
|---|---|---|
| For | Working on Dhow | Trying the released image |
| Needs | Go, Node.js LTS with pnpm, Docker for PostgreSQL | Docker with Compose |
| Address | `http://localhost:8080` | `http://127.0.0.1:8080`, and HTTPS for signing in |
| `DHOW_DEV` | `true` | not set |

## Browsers and `http://localhost`

The session cookie `__Host-dhow_session` is always `Secure`. Browsers store a `Secure` cookie only from a secure context, and Chrome and Firefox treat `http://localhost` and `http://127.0.0.1` as one. So signing in works over plain HTTP on the same machine, in those browsers. It does not work from another device on your network, for example `http://192.168.1.20:8080`: that address is not a secure context, the browser drops the cookie, and every sign-in seems to do nothing. Use HTTPS for that.

The browser also sends the address in the bar as `Origin`, and Dhow compares it with `DHOW_PUBLIC_URL` character for character. `http://localhost:8080` and `http://127.0.0.1:8080` are different origins: open exactly the address you set, or setup and sign-in fail with `CSRF_FAILED`.

## From source

You need the Go version pinned in `go.mod` (the `go` command downloads it), Node.js LTS with pnpm through Corepack, Docker, and `openssl`. Run every command from the root of a clone of the repository.

1. Start PostgreSQL 18 on the loopback interface:

   ```sh
   docker run -d --name dhow-dev-postgres \
     -e POSTGRES_USER=dhow -e POSTGRES_PASSWORD=dhow-dev-only -e POSTGRES_DB=dhow \
     -p 127.0.0.1:5432:5432 -v dhow-dev-pgdata:/var/lib/postgresql \
     postgres:18
   ```

2. Build the web app into the server's embed directory. `make web` runs pnpm from the repository root, where Corepack finds no `packageManager` field, so activate the version that `apps/web/package.json` names first. Skip the two `corepack` lines if `pnpm --version` already prints that version:

   ```sh
   corepack enable
   corepack prepare pnpm@10.28.0 --activate
   make web
   ```

3. Create a secret key once and keep it outside the repository. Stored secrets need the same key at every start:

   ```sh
   mkdir -p ~/.config/dhow
   (umask 077 && openssl rand -hex 32 > ~/.config/dhow/dev-secret-key)
   ```

4. Start the server:

   ```sh
   DHOW_DEV=true \
   DHOW_DATABASE_URL='postgres://dhow:dhow-dev-only@127.0.0.1:5432/dhow?sslmode=disable' \
   DHOW_SECRET_KEY="$(cat ~/.config/dhow/dev-secret-key)" \
   DHOW_PUBLIC_URL=http://localhost:8080 \
   go run ./apps/server/cmd/dhow-server
   ```

   Dhow applies the migrations, logs a `setup_url` line with a link such as `http://localhost:8080/setup#...`, and listens on `127.0.0.1:8080`.

5. Check it from another terminal with `curl -s http://localhost:8080/api/v1/health/ready`, then open the setup link in Chrome or Firefox to create the owner account. The link is valid for one hour. Restart the server to get a new one.

After a change to `apps/web`, run `make web` again and restart the server. For live reloading instead, run `pnpm dev` in `apps/web` and open `http://localhost:5173`: Vite serves the web app and forwards `/api` to the server on port 8080. The browser then sends `http://localhost:5173` as `Origin`, so start the server with `DHOW_PUBLIC_URL=http://localhost:5173` for that, and open the setup link on port 5173.

To start over with an empty database:

```sh
docker rm -f dhow-dev-postgres
docker volume rm dhow-dev-pgdata
```

## The Compose stack

This runs the released image the same way as a real install, so `DHOW_DEV` is not set and the database connection uses TLS.

1. Follow the [Compose quick start](compose.md#quick-start) and keep the default public URL:

   ```sh
   ./init.sh
   docker compose up -d
   ```

   `init.sh` sets `DHOW_PUBLIC_URL=https://dhow.localhost`. Names under `.localhost` point to your own machine.

2. Check it with `curl -s http://127.0.0.1:8080/api/v1/health/ready`, and open `http://127.0.0.1:8080` to see the web app.

3. Setup and sign-in need the browser on `https://dhow.localhost`, because that is the `DHOW_PUBLIC_URL` that `Origin` must match. Put [Caddy](https://caddyserver.com/docs/install) in front of it. For a `.localhost` name, Caddy creates a local certificate authority and asks for your password once to add it to the system's trust store:

   ```sh
   sudo caddy reverse-proxy --from dhow.localhost --to 127.0.0.1:8080
   ```

   Then open the setup link from `docker compose logs dhow | grep setup_url`.

`caddy reverse-proxy` is a quick command for your own machine. For a real server, use a configuration from [Put Dhow behind a reverse proxy](reverse-proxy.md).
