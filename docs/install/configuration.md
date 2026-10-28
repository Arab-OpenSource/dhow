# Configuration

Dhow reads its configuration from environment variables when it starts. It checks all of them first and refuses to start if any is invalid, listing every problem at once:

```text
dhow-server: invalid configuration:
DHOW_SECRET_KEY: is required; generate one with `openssl rand -hex 32`
DHOW_PUBLIC_URL: must use https (http is allowed only with DHOW_DEV=true)
```

There is no configuration file and no fallback value for a secret. A missing or weak secret stops the server instead of starting it insecurely.

## Server variables

| Variable | Required | Default | Example | Purpose and validation |
|---|---|---|---|---|
| `DHOW_DATABASE_URL` | Yes | none | `postgres://dhow:<password>@db.internal:5432/dhow?sslmode=verify-full` | The PostgreSQL 18 database. It must be a `postgres://` or `postgresql://` URL that names a host, either in the authority or as a `host` query parameter. Outside development, `sslmode` must be `require`, `verify-ca` or `verify-full`; `verify-full` also checks the server's certificate and name. Other [libpq parameters](https://www.postgresql.org/docs/18/libpq-connect.html#LIBPQ-PARAMKEYWORDS) are accepted, such as `sslrootcert`. Dhow opens at most 20 connections and waits 10 seconds for the first one. |
| `DHOW_SECRET_KEY` | Yes | none | the output of `openssl rand -hex 32` | Encrypts stored secrets and signs CSRF tokens. It must be hex or base64 (standard or URL-safe, padded or not) and decode to at least 32 bytes with at least 16 different byte values. A value that contains `change`, `replace`, `example`, `placeholder`, `secret`, `password`, `default` or `dummy`, in any case, is refused as a placeholder. Back it up with the database: a different key cannot read the secrets stored with the old one. |
| `DHOW_SECRET_KEY_FILE` | No | none | `/run/secrets/dhow-secret-key` | A file that holds `DHOW_SECRET_KEY`, see [Secrets in files](#secrets-in-files). |
| `DHOW_DATABASE_URL_FILE` | No | none | `/run/secrets/dhow-database-url` | A file that holds `DHOW_DATABASE_URL`, see [Secrets in files](#secrets-in-files). |
| `DHOW_PUBLIC_URL` | Yes | none | `https://dhow.example.com` | The address users open in the browser. It must be an absolute `https://` URL without credentials, a query or a fragment, and a trailing `/` is removed. `http://` is accepted only with `DHOW_DEV=true`. Every request that is not `GET`, `HEAD` or `OPTIONS` must carry an `Origin` equal to this value, and the setup link is built from it. A path is accepted by the check but [not supported](reverse-proxy.md#no-sub-path). |
| `DHOW_LISTEN_ADDR` | No | `127.0.0.1:8080` | `0.0.0.0:8080`, `[::1]:8080` | The address and port Dhow listens on for plain HTTP. It must be `host:port` with a valid port, and IPv6 addresses go in brackets. The container image sets `0.0.0.0:8080`. `dhow-server healthcheck` reads it too. |
| `DHOW_TRUSTED_PROXIES` | No | empty | `127.0.0.1`, `10.0.0.5,172.18.0.1/32` | Reverse proxies whose `X-Forwarded-For` header Dhow trusts, as a comma-separated list of IP addresses and CIDR ranges. Spaces around entries are ignored, and an IPv4-mapped IPv6 address counts as IPv4. Empty means Dhow uses the address of the TCP connection. See [Trusted proxies](reverse-proxy.md#trusted-proxies). |
| `DHOW_LOG_LEVEL` | No | `info` | `debug` | `debug`, `info`, `warn` (or `warning`) or `error`, in any case. Logs are JSON lines on standard output. |
| `DHOW_DEV` | No | `false` | `true` | Development mode. It allows an `http://` public URL and a database URL without a secure `sslmode`, and Dhow logs a warning at every start. It accepts `true` or `false` and the other spellings of Go's `strconv.ParseBool` (`1`, `0`, `t`, `f`, `TRUE`, `FALSE`, ...). Never set it on a host that other people can reach, see [Run Dhow locally](local.md). |
| `DHOW_BREACHED_PASSWORD_CHECK` | No | `true` | `false` | Rejects new passwords that appear in known breaches, using the Have I Been Pwned range API. Only the first 5 hex characters of the password's SHA-1 leave the server, and the check is skipped with a warning when the service cannot be reached within 3 seconds. Set `false` on networks without internet access. It accepts the same spellings as `DHOW_DEV`. |
| `DHOW_RUNNER_LISTEN_ADDR` | No | empty | `0.0.0.0:7443` | Moves the runner plane (`/runner-api/`, where runners register and sign in) off `DHOW_LISTEN_ADDR` onto a listener of its own, so a firewall or a Kubernetes NetworkPolicy can separate it. The path stays `/runner-api/`. Empty keeps the runner plane on the main listener. It must be `host:port` and differ from `DHOW_LISTEN_ADDR`. It needs both TLS files below, except with `DHOW_DEV=true`, where it serves plain HTTP and logs a warning. |
| `DHOW_RUNNER_TLS_CERT_FILE` | With `DHOW_RUNNER_LISTEN_ADDR` | none | `/etc/dhow/runner-tls.crt` | The PEM certificate chain of the separate runner listener. It is refused without `DHOW_RUNNER_LISTEN_ADDR`, without `DHOW_RUNNER_TLS_KEY_FILE`, or when the pair does not load. |
| `DHOW_RUNNER_TLS_KEY_FILE` | With `DHOW_RUNNER_LISTEN_ADDR` | none | `/etc/dhow/runner-tls.key` | The PEM private key of that certificate. Like a secret file, other users must not be able to read or write it (`chmod 600`). |
| `DHOW_RUNNER_MAX_POLLS` | No | `500` | `200` | How many `Lease` long-polls one server instance keeps open at once. A waiting poll holds no database connection, but every wake-up runs one short claim transaction, so the default is 25 polls for each of the 20 pooled database connections. Above the limit a runner gets `RESOURCE_EXHAUSTED` with `Retry-After` and backs off. A whole number from 1 to 10000. |
| `DHOW_JOB_LOG_MAX_BYTES` | No | `67108864` (64 MiB) | `268435456` | The most log output one job attempt may store, in bytes. The chunk that crosses it is cut at the limit, one `system` line says the log was truncated, and later output is dropped while the job keeps running. A whole number from 1048576 (1 MiB) to 1073741824 (1 GiB), without a unit. Logs live in PostgreSQL until log segments move to object storage, so the limit and the retention below decide how much they add to the database and its backups. |
| `DHOW_LOG_RETENTION_DAYS` | No | `30` | `7` | How many days of job logs to keep. Logs are stored in one PostgreSQL partition per UTC day, and every hour each instance drops the partitions older than this, under an advisory lock so instances never race. A whole number from 1 to 3650. |
| `DHOW_SSE_MAX_STREAMS` | No | `2000` | `500` | How many live log streams (server-sent events) one server instance serves at once. One user may hold 10 of them on an instance, and one organization a tenth of this value (at least 20). Above either limit the browser gets `429 TOO_MANY_STREAMS` with `Retry-After`. A whole number from 1 to 100000. |
| `DHOW_LOG_READ_CONNS` | No | `4` | `8` | Database connections set aside for reading job logs: history pages and live streams. They are a pool of their own, next to the 20 connections of the API and the runner plane, so many open streams can slow each other down but never the rest of Dhow. Each statement on them stops after 5 seconds. A whole number from 1 to 20. |
| `DHOW_ORG_CREATION` | No | `instance-owner` | `any-user` | Who may create more organizations. `instance-owner` allows only the instance owner, the account made by the first-owner setup. `any-user` allows every signed-in user. Any other value stops the server. |
| `DHOW_SMTP_HOST` | No | empty | `smtp.example.com` | The mail server for invitation emails. Empty turns email off: an admin still gets the invitation link and sends it themselves. A host name or an IP address, without a port. See [Email](#email). |
| `DHOW_SMTP_PORT` | No | `587`, or `465` with `DHOW_SMTP_TLS=tls` | `2525` | The mail server's port, from 1 to 65535. |
| `DHOW_SMTP_TLS` | No | `starttls` | `tls`, `none` | `starttls` connects in plain text and requires `STARTTLS` before anything else is sent. `tls` connects over TLS from the first byte (usually port 465). `none` sends in plain text and is accepted only without a username and password, for a relay on a private network. The server certificate is always verified. |
| `DHOW_SMTP_USERNAME` | No | empty | `dhow` | The SMTP login. Set it together with a password, or leave both empty for a relay that needs no login. |
| `DHOW_SMTP_PASSWORD` | No | empty | none | The SMTP password. It is sent only over TLS (`AUTH PLAIN`) and never logged. |
| `DHOW_SMTP_PASSWORD_FILE` | No | none | `/run/secrets/dhow-smtp-password` | A file that holds `DHOW_SMTP_PASSWORD`, see [Secrets in files](#secrets-in-files). |
| `DHOW_SMTP_FROM` | With `DHOW_SMTP_HOST` | none | `Dhow <dhow@example.com>` | The sender of invitation emails, as one RFC 5322 address with an optional display name. |
| `DHOW_OIDC_PROVIDERS` | No | empty | `google,github` | Turns on sign-in with external identity providers. A comma-separated list of provider ids (lowercase letters and digits, starting with a letter, at most 32, at most 10 providers). Each id needs its own `DHOW_OIDC_<ID>_*` variables, see [Sign-in providers](#sign-in-providers). Empty turns external sign-in off. |
| `DHOW_RATE_LIMIT_PUBLIC` | No | `300` | `1000` | How many requests one client network may send in 10 minutes to each group of public sign-in routes: invitation preview and accept, OIDC authorize and callback, and passkey sign-in options. A client network is one IPv4 address or one IPv6 `/64`, found as described below. Above it the answer is `429 RATE_LIMITED` with `Retry-After`. The counters live in the database, so every instance shares them. Raise it when many people sign in from behind one NAT. A whole number from 1 to 100000, see [Rate limits](#rate-limits). |
| `DHOW_RATE_LIMIT_INVITES_USER` | No | `50` | `200` | How many invitations one member may create or resend in an hour, across all organizations. A whole number from 1 to 100000. |
| `DHOW_RATE_LIMIT_INVITES_ORG` | No | `200` | `1000` | How many invitations one organization may create or resend in an hour, whoever sends them. A whole number from 1 to 100000. |

`DHOW_DEV=true` never turns off the `Secure` flag of the session cookie, the `Origin` check or the secret key checks.

## Rate limits

Public routes that write rows or start sign-in flows are limited per client network, and invitations are limited per member and per organization so the server is not used to send spam. Password sign-in, the second step and setup keep their own lockout per account and per address (see the [security model](../security-model.md#5-sessions-and-tokens-adr-0033)).

- **Fixed windows in the database.** Each limit counts requests in a fixed window (10 minutes for the public routes, an hour for invitations) in the `RateLimits` table, with one upsert per request. Several instances behind a load balancer therefore share one counter, and a restart does not reset it.
- **Client network.** The address comes from the TCP connection, or from `X-Forwarded-For` only when the connection comes from `DHOW_TRUSTED_PROXIES`. An IPv6 client is counted per `/64`, because one host usually controls a whole `/64`. Without a correct `DHOW_TRUSTED_PROXIES` behind a proxy, every user shares the proxy's address and its limit.
- **People behind one NAT** share one limit. An office of 100 people who all sign in with OIDC within 10 minutes sends about 200 requests, under the default of 300. Raise `DHOW_RATE_LIMIT_PUBLIC` for larger sites.
- **The answer** is `429` with the code `RATE_LIMITED`, a translated message and `Retry-After` in seconds until the window ends.
- **Each invitation** can get a new link at most 4 times (5 sends in all). After that, resending answers `409 INVITATION_SEND_LIMIT`: revoke it and invite again, which counts against the hourly limits.
- Counter rows are deleted an hour after their window ends, by the cleanup that runs every 10 minutes (see [Retention of temporary rows](../security-model.md#retention-of-temporary-rows)).

## Secrets in files

`DHOW_SECRET_KEY`, `DHOW_DATABASE_URL` and `DHOW_SMTP_PASSWORD` can come from a file instead of the environment, so they never show up in `ps`, `systemctl show` or `docker inspect`:

- `DHOW_SECRET_KEY_FILE`, `DHOW_DATABASE_URL_FILE` and `DHOW_SMTP_PASSWORD_FILE` name the file.
- Under systemd, `LoadCredential=dhow-secret-key:...`, `LoadCredential=dhow-database-url:...` and `LoadCredential=dhow-smtp-password:...` work without any variable: Dhow reads `$CREDENTIALS_DIRECTORY/dhow-secret-key`, `$CREDENTIALS_DIRECTORY/dhow-database-url` and `$CREDENTIALS_DIRECTORY/dhow-smtp-password`. The [unit file](binary-systemd.md) does this for the first two.
- Each sign-in provider's client secret works the same way: `DHOW_OIDC_<ID>_CLIENT_SECRET_FILE`, or the systemd credential `dhow-oidc-<id>-client-secret` (see [Sign-in providers](#sign-in-providers)).

The rules are the same for all of them:

- Set each secret in one place only. A variable together with its `_FILE`, or with a systemd credential, stops the server.
- The file must be a regular file of at most 64 KiB that other users cannot read or write (`chmod 600`). Group access is allowed.
- One trailing newline (`\n` or `\r\n`) is removed, so `openssl rand -hex 32 > file` works. An empty file is refused.
- The value is checked exactly like the variable: a placeholder key or a URL without a secure `sslmode` still stops the server, and the error names the file, never its content.

## Email

Email is optional. Without `DHOW_SMTP_HOST`, an admin who invites someone gets the invitation link (`https://<your host>/invite#<token>`) and sends it through any channel they trust.

With SMTP set up, Dhow also emails the link to the invited address in the inviter's language. Sending is fail-safe: if the mail server is down or refuses the message, the invitation still exists, the admin still gets the link, and the screen says the email was not sent. Dhow waits at most 10 seconds for the mail server.

- The link is the only secret in the email. It works once and expires after 7 days, and resending an invitation makes a new link and voids the old one.
- `DHOW_SMTP_FROM`, the organization name and the inviter's name are the only values that reach the headers, and a value that contains a line break never does.
- Test the settings by inviting your own address.

## Sign-in providers

Each id in `DHOW_OIDC_PROVIDERS` reads the variables below, with the id in upper case (`google` reads `DHOW_OIDC_GOOGLE_ISSUER`). Any problem with any provider stops the server. The full guide, with Google, Keycloak and GitHub examples, is [Sign in with OIDC](../guides/oidc.md).

| Variable | Required | Default | Purpose and validation |
|---|---|---|---|
| `DHOW_OIDC_<ID>_TYPE` | No | `oidc` | `oidc` for any OpenID Connect provider (Google, Keycloak, Entra ID, Authentik, ...). `github` for GitHub, which speaks OAuth 2.0 without OpenID Connect. |
| `DHOW_OIDC_<ID>_ISSUER` | With `oidc` | `https://github.com` for `github` | The issuer URL, exactly as the provider's `/.well-known/openid-configuration` reports it. `https://` only, or `http://` with `DHOW_DEV=true`, without credentials, a query or a fragment. For GitHub Enterprise Server, the server's URL. Two providers cannot share an issuer (compared without case in the host and without a trailing `/`), because linked identities are keyed by issuer. |
| `DHOW_OIDC_<ID>_CLIENT_ID` | Yes | none | The client id registered at the provider. |
| `DHOW_OIDC_<ID>_CLIENT_SECRET` | Yes | none | The client secret. Like other secrets it may come from `DHOW_OIDC_<ID>_CLIENT_SECRET_FILE` or the systemd credential `dhow-oidc-<id>-client-secret`, see [Secrets in files](#secrets-in-files), and placeholder values are refused. It is never logged or returned. |
| `DHOW_OIDC_<ID>_NAME` | No | the id, or `GitHub` | The button label on the sign-in page, at most 40 characters. |
| `DHOW_OIDC_<ID>_ALLOWED_DOMAINS` | No | empty | Comma-separated email domains. When set, an identity is accepted only if the provider reports a verified email address in one of them. Empty accepts any identity the provider signs. |

The redirect URI to register at the provider is `<DHOW_PUBLIC_URL>/oidc/callback/<id>`.

## Development and tests

| Variable | Required | Default | Example | Purpose and validation |
|---|---|---|---|---|
| `DHOW_TEST_DATABASE_URL` | Only in CI | none | `postgres://dhow:dhow@127.0.0.1:5432/dhow_test?sslmode=disable` | Used by the Go tests, never by the server. The database tests create a throwaway schema in this database for each test. Without it they are skipped, except when `CI` is set, where they fail. |

## Compose stack variables

The [Compose stack](compose.md) reads a few more variables from `.env` that never reach the server: `POSTGRES_PASSWORD`, `DHOW_PUBLISH` and `DHOW_IMAGE`, described in [its configuration table](compose.md#configuration). The [Traefik file](reverse-proxy.md#traefik-in-the-compose-stack) adds `ACME_EMAIL` and `ACME_CA_SERVER`.

## Keeping this page current

A Go test in `apps/server/internal/Config` fails when the config package declares a `DHOW_*` variable that the first two tables on this page do not list, or when a table lists one that the code no longer reads.
