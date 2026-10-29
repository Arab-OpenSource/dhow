# Rotate a runner's keys

A runner's identity is two key pairs that `dhow-runner register` made on the runner host: Ed25519, which signs its sign-in and its job starts, and MLKEM768-X25519, which the server encrypts job secrets for. `dhow-runner rotate-keys` replaces both with new pairs while the runner keeps its ID, labels and history. The private keys are made on the runner host and never leave it.

## When to rotate

- **Once a year at least.** The runner page shows the identity key version and when the keys were last rotated.
- **When the state directory may have been read** by someone else: a backup that leaked, a disk that left the building, an administrator who left. Rotating makes the copied keys useless.
- **After a host was restored from a snapshot** that other machines might also run. If two machines might already share the identity, revoke the runner and register each host with its own token instead: a rotation on one copy cuts the other off, but you cannot tell which copy rotated first.

You do not need to rotate after a server upgrade, or after a change of `DHOW_SECRET_KEY` on the server (that change needs every runner to register again).

## How

The runner must be stopped. `rotate-keys` refuses to run while `dhow-runner run` holds the state directory, and while the lease journal still lists attempts the last run did not report: start the runner once so it reports them, then stop it again.

```sh
sudo systemctl stop dhow-runner
sudo systemd-run --uid=dhow-runner --gid=dhow-runner --pipe --wait --collect \
  -p EnvironmentFile=/etc/dhow-runner/runner.env /usr/local/bin/dhow-runner rotate-keys
sudo systemctl start dhow-runner
```

`systemd-run` gives the command the same `runner.env` as the service, so a custom `DHOW_RUNNER_STATE_DIR`, `DHOW_RUNNER_URL` or `DHOW_RUNNER_CA_FILE` is used. If `runner.env` is empty, `sudo -u dhow-runner dhow-runner rotate-keys` is enough. `--dev` allows plain `http://` to a loopback address, as for `register`.

The command prints the new key version:

```
Rotated the keys of runner 0190c8a2-…: key version 2. The old key no longer works.
```

From that moment the server refuses the old key, ends every session the runner had, and encrypts new secrets for the new key only. The organization's audit log records `runner.keys_rotated` with the new key version.

## What happens on a crash

A rotation is safe to interrupt at any point: power loss, a killed process or a lost network. The runner always starts with a key the server accepts.

1. **The new keys are saved first.** `rotate-keys` writes the full new identity to `identity.pending` in the state directory (mode `0600`, written to a temporary file, flushed to disk, renamed, and the directory flushed), before the server hears of the new keys.
2. **The server is asked to switch,** in one request signed by the current key and by the new key over the same message (the runner ID, the current key version and both new public keys). The server checks both signatures, bumps the key version only if it is still the one signed, and ends the runner's sessions, all in one transaction.
3. **Once the server confirms,** the runner writes the new identity over `identity` the same safe way, built from the current identity with only the new keys taken over, and then removes `identity.pending`.

| Interrupted | On disk | On the server | What fixes it |
|---|---|---|---|
| Before or while `identity.pending` is written | old identity, maybe a half-written temporary file | old key | Nothing to fix. The next run or rotation removes the temporary file. |
| After `identity.pending` is saved, before the server switched | old identity and `identity.pending` | old key | The old key still signs in, so the saved keys were never used. `run` and `rotate-keys` both overwrite and remove `identity.pending`, and `rotate-keys` makes fresh keys. |
| After the server switched, before the reply arrived or the identity was written | old identity and `identity.pending` | new key | The old key is refused and the saved key signs in. `run` writes the new identity and goes on; `rotate-keys` does the same and stops. |
| After the identity was written, before `identity.pending` was removed | new identity and `identity.pending` with the same keys | new key | `run` and `rotate-keys` remove the leftover file. |

Saved keys are used only when the server already holds them: the current key is refused and the saved one signs in. If the current key still signs in, the server never took the saved keys (it switches only from the key version both signatures name), so they are overwritten with zeros and removed, and a new rotation makes fresh ones. Keys that sat on disk unconfirmed, perhaps long enough to be copied, are never sent to the server. If both keys are refused (the runner was revoked, or the server's data was restored from an older backup), the runner keeps retrying with backoff like any runner whose credentials are refused; revoke it and register the host again.

## When it fails

| Message | Meaning |
|---|---|
| `the runner is running; stop it first` | `dhow-runner run` holds the state directory. Stop the service and try again. |
| `the last run left attempts it has not reported yet` | Start the runner so it reports them, stop it, then rotate. |
| `the server did not confirm the new keys` | The request failed or timed out. The current key stays in use; run `rotate-keys` again. If the server did switch, the runner also finishes the rotation on its next start. |
| `the server took the new keys, but the identity file was not switched yet` | Writing the identity failed (a full or read-only disk). Fix the disk, then run `rotate-keys` again or start the runner. |
| `the pending identity belongs to another runner or server` | `identity.pending` does not match `identity`. Someone copied files between state directories; remove `identity.pending` only if you are sure no rotation of this runner is unfinished. |
| `RUNNER_REVOKED` | The runner was revoked. Register the host again. |

The design is in [ADR-0065](../../Plan/ADR/0065-runner-packaging-part-a.md).
