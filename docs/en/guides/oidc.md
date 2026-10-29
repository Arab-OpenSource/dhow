[العربية](../../guides/oidc.md)

# Sign in with OIDC

Dhow can let people sign in with an external identity provider: any OpenID Connect provider (Google, Keycloak, Microsoft Entra ID, Authentik, ...) and GitHub. This guide shows how to register Dhow at the provider, how to configure it, and how people link their accounts.

The variables are listed in [Configuration](../install/configuration.md#sign-in-providers). The security design is in [the security model](../security-model.md#4-authentication-adr-0013).

## How it works

- **A provider signs in an existing account, never creates one.** People still join Dhow by invitation and set a password. After that, they open **Account** and link a provider. From then on, the provider's button on the sign-in page opens their account.
- **Accounts are matched by the provider's subject, never by email.** A provider identity whose email equals a Dhow account's email does not open that account until the owner links it. This stops anyone who controls an email address at some provider from taking over an account.
- **Two-step sign-in still applies.** An account with TOTP or a passkey is asked for it after the provider, exactly as after a password.
- **The last sign-in method stays.** Dhow refuses to unlink an identity if the account would have no way left to sign in.

## 1. Pick an id

Every provider has a short id: lowercase letters and digits, starting with a letter. The id appears in variable names and in the redirect URI, so pick it once and keep it: linked identities remember the id they were linked under, and the account page shows them by it. The examples use `google`, `corp` and `github`.

## 2. Register Dhow at the provider

Register a web application (a confidential client) with:

- **Redirect URI:** `<DHOW_PUBLIC_URL>/oidc/callback/<id>`, for example `https://dhow.example.com/oidc/callback/google`. It must match exactly.
- **Grant type:** authorization code. Dhow always sends PKCE (`S256`), so you can require it.
- **Scopes:** `openid email profile` for OIDC, `read:user user:email` for GitHub. Dhow asks for nothing else.

Keep the client id and the client secret for the next step.

### Google

1. In the Google Cloud console, open **APIs & Services → Credentials → Create credentials → OAuth client ID**, application type **Web application**.
2. Add the redirect URI under **Authorized redirect URIs**.
3. The issuer is `https://accounts.google.com`.

### Keycloak

1. In your realm, create a client with **Client authentication** on and **Standard flow** on.
2. Add the redirect URI under **Valid redirect URIs**. Under **Advanced**, set **Proof Key for Code Exchange Code Challenge Method** to `S256`.
3. The secret is under **Credentials**. The issuer is `https://<keycloak-host>/realms/<realm>`.

### GitHub

GitHub speaks OAuth 2.0 without OpenID Connect, so it has its own type.

1. Open **Settings → Developer settings → OAuth Apps → New OAuth App** (for an organization, the organization's settings).
2. Set **Authorization callback URL** to the redirect URI.
3. Generate a client secret.

Dhow identifies a GitHub account by its numeric id, not by its login, because a login can be renamed and then taken by someone else.

For GitHub Enterprise Server, set `DHOW_OIDC_<ID>_ISSUER` to the server's URL, for example `https://github.example.com`. Dhow calls its API under `/api/v3`.

## 3. Configure Dhow

The simplest way is the dashboard: open **Server settings → Sign-in providers**, press **Add a sign-in provider** and fill in what you got in step 2. See [Server settings](instance-settings.md#sign-in-providers). The rest of this step is for installs that manage configuration in files, such as Kubernetes or GitOps. When `DHOW_OIDC_PROVIDERS` is set, the whole list comes from the environment and the dashboard shows it read-only.

List the ids in `DHOW_OIDC_PROVIDERS`, then give each id its variables, with the id in upper case:

```sh
DHOW_OIDC_PROVIDERS=google,corp,github

DHOW_OIDC_GOOGLE_ISSUER=https://accounts.google.com
DHOW_OIDC_GOOGLE_CLIENT_ID=1234567890-abc.apps.googleusercontent.com
DHOW_OIDC_GOOGLE_CLIENT_SECRET_FILE=/run/secrets/dhow-oidc-google-client-secret
DHOW_OIDC_GOOGLE_ALLOWED_DOMAINS=example.com

DHOW_OIDC_CORP_ISSUER=https://sso.example.com/realms/staff
DHOW_OIDC_CORP_CLIENT_ID=dhow
DHOW_OIDC_CORP_CLIENT_SECRET_FILE=/run/secrets/dhow-oidc-corp-client-secret
DHOW_OIDC_CORP_NAME=Example SSO

DHOW_OIDC_GITHUB_TYPE=github
DHOW_OIDC_GITHUB_CLIENT_ID=Ov23liExample
DHOW_OIDC_GITHUB_CLIENT_SECRET_FILE=/run/secrets/dhow-oidc-github-client-secret
```

- **The client secret** is a secret like `DHOW_SECRET_KEY`. Prefer `_CLIENT_SECRET_FILE`, or under systemd `LoadCredential=dhow-oidc-<id>-client-secret:...`, so it never shows up in `ps` or `docker inspect`. Setting more than one source stops the server.
- **The button label** comes from `DHOW_OIDC_<ID>_NAME`, or the id when it is empty (`GitHub` for the `github` type). It appears as "Sign in with <name>".
- **Allowed domains** (`DHOW_OIDC_<ID>_ALLOWED_DOMAINS`) accept an identity only if the provider reports a verified email whose domain is exactly one in the list. `example.com` does not allow `team.example.com`; list both if you need both. Use it for providers anyone can sign up at, such as Google or GitHub, to keep out identities from outside your company.

Restart Dhow. **Any mistake in any provider stops the server** with a message that names the variable: a missing client id or secret, a placeholder secret, an issuer that is not `https://` (plain `http://` only with `DHOW_DEV=true`), an unknown type or a malformed domain. Dhow does not contact the provider at start-up, so a provider that is down does not stop Dhow; only its button fails until it is back.

## 4. Link an account

1. Sign in with your password.
2. Open **Account → Linked sign-in accounts** and press **Link** next to the provider.
3. Confirm with your current factor (password, TOTP code or passkey). Dhow sends you to the provider and back.

Linking signs you out of your other devices. Each Dhow account can link one identity per provider, and an identity can belong to one Dhow account only.

To unlink, press **Unlink** and confirm with your current factor.

## Troubleshooting

| What you see | Why | What to do |
|---|---|---|
| The server does not start and names a `DHOW_OIDC_*` variable | That provider's configuration is invalid | Fix the variable the message names |
| "The sign-in provider cannot be reached right now" | Dhow could not reach the provider's discovery document or token endpoint. Dhow waits 30 seconds before it tries discovery again | Check that the Dhow host can reach the issuer over HTTPS, and that the issuer is exactly what `/.well-known/openid-configuration` reports, including any trailing `/` |
| The provider says the redirect URI does not match | The URI registered at the provider differs from `<DHOW_PUBLIC_URL>/oidc/callback/<id>` | Register the exact URI, with the same scheme, host and id |
| "No Dhow account is linked to this identity" | Nobody linked this identity to a Dhow account | Sign in with a password and link it from **Account** |
| "This identity's email domain is not allowed" | The identity has no verified email in `ALLOWED_DOMAINS` | Use an account in an allowed domain, or change the list |
| "The sign-in took too long or was started in another browser" | The attempt expired (10 minutes), was finished already, or started in another browser | Start again from the sign-in page in the same browser |
| "Signing in with this provider did not work" | The provider refused the code or its answer failed a check (signature, issuer, audience, expiry or nonce), or the account is disabled | Look at the server log and the provider's log; try again |
| "This identity is already linked to an account" | Someone else already linked it | Unlink it there first |
| "Your account is already linked to this provider" | Each account links one identity per provider | Unlink the current one first |

Failed provider sign-ins after a valid attempt are recorded in the audit log as `auth.oidc_login_failed`, with the reason.
