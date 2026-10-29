# Register a runner

Registration turns a one-time token into a runner identity: a pair of keys that stays on the runner host, and a runner record in your organization.

## 1. Create a registration token

An organization owner or admin creates it in the dashboard: open **Runners** in the side menu and choose **Add a runner**. Pick the labels, how many machines the token may register (1 to 50) and how long it stays valid (1, 6 or 24 hours). The next screen shows the token once, next to the exact `dhow-runner register` command for your server and the command that starts the service. Copy the token before you close it: the dashboard never shows it again. A lost token is revoked from the **Registration tokens** page (`/runners/tokens`, linked at the top of **Runners**).

With the API:

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

The dashboard shows this same command with your server's address filled in. It runs as the `dhow-runner` user, so the identity belongs to the service account and not to root. On a server started with `DHOW_DEV=true`, the command shown after you create a token adds `--dev`. The command on the empty **Runners** page adds it when you open the dashboard over plain `http://` on a loopback address.

The runner prints its runner ID, its organization and its labels. It saved the identity in `$DHOW_RUNNER_STATE_DIR/identity` with mode `0600`, together with the server address and the server's signing key, which every later job grant is checked against.

If the server uses a private CA, set `DHOW_RUNNER_CA_FILE` before registering. If the runner reaches the server on a separate runner listener (`DHOW_RUNNER_LISTEN_ADDR` on the server), set `--url` to that listener and `DHOW_RUNNER_SERVER_ORIGIN` to the server's public address (`DHOW_PUBLIC_URL`), because the runner's signatures name the public origin.

## Manage runners in the dashboard

**Runners** lists every runner of the organization with its state, labels and last-seen time. Owners, admins, maintainers and developers can open it; only owners and admins can change anything.

| State | Meaning |
|---|---|
| Online | Seen in the last two minutes, and takes new jobs. |
| Offline | Active, but not seen in the last two minutes. Check the service and its network path to the server. |
| Paused | Takes no new jobs and finishes the ones it started. **Resume** lets it take jobs again. |
| Paused: copied identity | Dhow paused it because one identity was used from several machines ([copying a runner](#copying-a-runner)). Resume it only if you are sure it runs on one machine. |
| Revoked | Cut off for good. Register the machine again with a new token. |

Open a runner to see its OS and architecture, version, capacity, isolation levels, capability labels and identity key version, to rename it or change its labels, to pause or resume it, and to revoke it.

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
