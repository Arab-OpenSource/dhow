# Server settings

**Server settings** (`/system/settings`, under **Monitoring** in the side menu) is where the instance owner changes mail, who may create organizations, the breached-password check, rate limits and job log limits, without editing a file or restarting the server. The decision is [ADR-0063](../../Plan/ADR/0063-instance-settings-in-the-database.md), and every setting is listed in [Configuration](../install/configuration.md#dashboard-settings).

## Who can open it

Only the instance owner, the account made by the first-owner setup. Everyone else, including the owners of organizations, gets the not-found page, and the item is not in their menu. An API token can never read or change these settings.

## What each card shows

The page loads every setting in one request and shows one card per group: **Mail**, **Organizations**, **Passwords**, **Rate limits** and **Job logs**. Each card has the usual states: a skeleton while it loads, an error with **Try again**, a note when nothing is set yet (for example "Email is off"), and the values.

- **A value set in the environment** is read-only. It says **Set by an environment variable** and shows the variable, such as `DHOW_RATE_LIMIT_PUBLIC`. Change it where you set the variable, then restart.
- **A value nobody set** shows the default and says so next to its name.
- **The SMTP password** never comes back from the server. The card says **Saved** or **Not saved**. **Replace the password** opens a field for a new one, and **Remove the password** clears it when you save. If you do neither, the saved password stays.

## Saving

1. Change the fields of one card and press **Save changes**. **Undo changes** puts the card back.
2. Confirm it is you with a code from your authenticator app, a passkey or a recovery code. Saving needs two-step verification on your account. Without it, the card asks you to turn it on first.
3. The change works at once on every server instance that shares the database. A new largest attempt log applies to attempts that start after the save.

If a value is not valid, the field says why and nothing is saved. If someone else saved the same card a moment before you, the page asks you to reload and try again.

Every save writes `system.settings.changed` to the audit log, with the group and the names of the fields that changed. A password value is never written anywhere.

## Testing mail

Save a mail server first, then press **Send a test email**. Dhow sends one short message to your own address with the settings in effect, and tells you where it went. If the mail server refuses it, the card shows a short error and the reason is in the server log. You can send five tests in ten minutes.

## Locked settings and `doctor`

An environment variable always wins, so a setting managed by Kubernetes, Compose or systemd never changes behind your back. Nothing is copied from the environment into the database: when you remove a variable and restart, the setting unlocks and takes the saved value, or the default.

`dhow-server doctor` prints where each setting comes from (the environment, the database or the default), never its value.
