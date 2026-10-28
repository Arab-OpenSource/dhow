# Register a runner

Registration turns a one-time token into a runner identity: a pair of keys that stays on the runner host, and a runner record in your organization.

## 1. Create a registration token

An organization owner or admin creates it from the runners page, or with the API:

```sh
curl -X POST https://dhow.example.com/api/v1/organizations/<org-id>/runner-registration-tokens \
  -H 'Content-Type: application/json' --cookie ... --header 'X-CSRF-Token: ...' \
  -d '{"Labels": ["linux-builders"], "MaxUses": 1, "ExpiresInSeconds": 3600}'
```

- The token (`dhow_rrt_...`) is shown once. It expires after an hour by default and 24 hours at most, and works once unless you raise `MaxUses` (up to 50, for a fleet).
- The labels you choose are the runner's labels. A runner cannot pick its own: it only reports what it can do (`dhow.os`, `dhow.arch`, `dhow.isolation`), and those are added for it.

## 2. Register on the runner host

The token is read from a file or from stdin, never from the command line, where shell history and `ps` would keep it:

```sh
sudo -u dhow-runner dhow-runner register --url https://dhow.example.com --token-file -
# paste the token, then press Enter
```

| Flag | Meaning |
|---|---|
| `--url` | The Dhow server address, `https://` and a host name only. `DHOW_RUNNER_URL` works too. |
| `--token-file` | A file that holds the token, or `-` for stdin. Required. |
| `--name` | The name shown in Dhow. Defaults to the host name. |
| `--dev` | Allows plain `http://` to `localhost`, `127.0.0.1` or `[::1]`, for local development only. |

The runner prints its runner ID, its organization and its labels. It saved the identity in `$DHOW_RUNNER_STATE_DIR/identity` with mode `0600`, together with the server address and the server's signing key, which every later job grant is checked against.

If the server uses a private CA, set `DHOW_RUNNER_CA_FILE` before registering. If the runner reaches the server on a separate runner listener (`DHOW_RUNNER_LISTEN_ADDR` on the server), set `--url` to that listener and `DHOW_RUNNER_SERVER_ORIGIN` to the server's public address (`DHOW_PUBLIC_URL`), because the runner's signatures name the public origin.

## When it fails

| Message | Cause |
|---|---|
| `INVALID_REGISTRATION_TOKEN` | The token is wrong, expired, used up, revoked or from another organization. The server gives the same answer for all of them on purpose. Create a new token. |
| `this runner is already registered` | The state directory already has an identity. Revoke the old runner in Dhow and remove the directory to register again. |
| `x509: certificate signed by unknown authority` | Set `DHOW_RUNNER_CA_FILE` to the CA that signed the server certificate. The runner never skips verification. |
| `must use https://` | Plain HTTP is refused outside `--dev` on the loopback address. |
| `RUNNER_KEY_IN_USE` | This key pair is already registered. It cannot happen with a fresh registration, so check that two hosts do not share a state directory. |

## Copying a runner

Never copy a state directory to a second host. Two runners with one identity keep replacing each other's session, and after three replacements in ten minutes Dhow disables the runner as a suspected clone. Register each host with its own token.

Next: [run the runner](run.md).
