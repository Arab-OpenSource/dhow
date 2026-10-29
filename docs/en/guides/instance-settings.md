[العربية](../../guides/instance-settings.md)

# Server settings

**Server settings** (`/system/settings`, under **Monitoring** in the side menu) is where the instance owner changes mail, sign-in providers, who may create organizations, the breached-password check, rate limits and job log limits, without editing a file or restarting the server. The decision is [ADR-0063](../../../Plan/ADR/0063-instance-settings-in-the-database.md), and every setting is listed in [Configuration](../install/configuration.md#dashboard-settings).

## Who can open it

Only the instance owner, the account made by the first-owner setup. Everyone else, including the owners of organizations, gets the not-found page, and the item is not in their menu. An API token can never read or change these settings.

## What each card shows

The page loads the settings in two requests, one for the sign-in providers and one for the rest, and shows one card per group: **Mail**, **Sign-in providers**, **Organizations**, **Passwords**, **Rate limits** and **Job logs**. Each card has the usual states: a skeleton while it loads, an error with **Try again**, a note when nothing is set yet (for example "Email is off"), and the values.

- **A value set in the environment** is read-only. It says **Set by an environment variable** and shows the variable, such as `DHOW_RATE_LIMIT_PUBLIC`. Change it where you set the variable, then restart.
- **A value nobody set** shows the default and says so next to its name.
- **A setting locked because of the password.** When the SMTP password comes from the environment, the mail server, port and encryption are read-only too, with a line that says why. The password must never go to another server.
- **The SMTP password** never comes back from the server. The card says **Saved** or **Not saved**. **Replace the password** opens a field for a new one, and **Remove the password** clears it when you save. If you do neither, the saved password stays.

## Saving

1. Change the fields of one card and press **Save changes**. **Undo changes** puts the card back.
2. Confirm it is you with a code from your authenticator app, a passkey or a recovery code. Saving needs two-step verification on your account. Without it, the card asks you to turn it on first.
3. The change works at once on every server instance that shares the database. A new largest attempt log applies to attempts that start after the save.

If a value is not valid, the field says why and nothing is saved. If you change the mail server while a password is saved, the card asks you to type the password again. **No encryption** is accepted here only for a mail server on the same machine, and link-local or cloud metadata addresses are refused. If someone else saved the same card a moment before you, the page asks you to reload and try again.

Every save writes `system.settings.changed` to the audit log, with the group and the names of the fields that changed. A password value is never written anywhere.

## Testing mail

Save a mail server first, then press **Send a test email**. Dhow sends one short message to your own address with the settings in effect, and tells you where it went. If the mail server refuses it, the card shows a short error and the reason is in the server log. You can send five tests in ten minutes.

## Sign-in providers

The **Sign-in providers** card lists each provider with its type, its issuer and how many accounts are linked to it. A provider without a client secret says **Off** and is not on the sign-in page.

**Add a sign-in provider** opens a panel:

1. Choose **Google**, **GitHub** or **OpenID Connect** (Keycloak, Okta, Microsoft Entra and others). Google and GitHub fill in the issuer, the name and the id.
2. Check the id. It never changes after you save, because it is part of the callback URL and of linked accounts.
3. Copy the **Callback URL** under the id and register it at the provider before you save.
4. Fill in the issuer, the client ID, the client secret and, if you want, the allowed email domains.
5. Press **Test the connection** to check that the server reaches the provider, that its addresses use https and that it serves its signing keys. The test never sends the client secret.
6. Confirm it is you with your second step and press **Add the provider**. The button is on the sign-in page at once, on every instance.

**Edit provider** changes the name, the client ID, the domains and the secret. Once accounts are linked, the service and its issuer stay as they are, because accounts are linked by issuer; add a new provider instead. **Remove the secret** turns the provider off.

**Remove provider** names how many accounts are linked and asks for your second step. Linked accounts keep the link on their account page and can unlink it, but nobody signs in with it; open sessions stay open. If some account can sign in only with this provider, the card says how many and does not offer the removal: give those accounts another way to sign in first.

When `DHOW_OIDC_PROVIDERS` is set in the environment, the whole list comes from there. The card says **Set by environment variables**, shows the providers read-only and only offers **Test the connection**. Dhow never mixes the two lists; see [Sign-in providers](../install/configuration.md#sign-in-providers).

Every add, change and removal writes `system.settings.changed` with the group `oidc`, the provider id, the operation and the names of the fields that changed. A client secret is never written anywhere.

## Locked settings and `doctor`

An environment variable always wins, so a setting managed by Kubernetes, Compose or systemd never changes behind your back. Nothing is copied from the environment into the database: when you remove a variable and restart, the setting unlocks and takes the saved value, or the default.

`dhow-server doctor` prints where each setting comes from (the environment, the database or the default), never its value, and how many sign-in providers there are and where they come from.
