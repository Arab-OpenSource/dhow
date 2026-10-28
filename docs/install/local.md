# Run Dhow locally

This page is for trying Dhow out or working on it on your own computer. To install it for other people, use [Docker Compose](compose.md) behind a [reverse proxy](reverse-proxy.md).

> [!WARNING]
> **`DHOW_DEV=true` is for your own machine only.** It lets Dhow start with an `http://` public URL and a database connection without TLS, so passwords, session cookies and every secret cross the network in plain text. Dhow logs `DHOW_DEV=true: plain HTTP and unencrypted database connections are allowed. Never use this in production.` at every start in this mode. Never set it on a server, a shared machine, or anything other people can reach, and never copy a development `.env` to production.

There are two ways to run Dhow locally:

| | [From source](#from-source) | [The Compose stack](#the-compose-stack) |
|---|---|---|
| For | Working on Dhow | Trying the released image |
| Needs | Go, Node.js LTS with pnpm, PostgreSQL 18 (Docker, Podman or native) | Docker with Compose |
| Address | `http://localhost:8080` | `http://127.0.0.1:8080`, and HTTPS for signing in |
| `DHOW_DEV` | `true` | not set |

## Browsers and `http://localhost`

The session cookie `__Host-dhow_session` is always `Secure`. Browsers store a `Secure` cookie only from a secure context, and Chrome and Firefox treat `http://localhost` and `http://127.0.0.1` as one. So signing in works over plain HTTP on the same machine, in those browsers. It does not work from another device on your network, for example `http://192.168.1.20:8080`: that address is not a secure context, the browser drops the cookie, and every sign-in seems to do nothing. Use HTTPS for that.

The browser also sends the address in the bar as `Origin`, and Dhow compares it with `DHOW_PUBLIC_URL` character for character. `http://localhost:8080` and `http://127.0.0.1:8080` are different origins: open exactly the address you set, or setup and sign-in fail with `CSRF_FAILED`.

## From source

The full guide for working on Dhow is [Develop Dhow on your machine](../development.md): PostgreSQL 18 with Docker, Podman or a native install on Linux, macOS and Windows (WSL2), the web app with live reload, the tests, and the runner. In short:

```sh
make dev-env
make dev-db
make web
make dev
```

Then open the `setup_url` link the server logs, on `http://localhost:8080`.

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
