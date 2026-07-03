# Authentication Manual

This manual describes the login methods supported by Accounting, the runtime configuration for each method, and the recommended external SSO setup for `sso.laisky.com`.

## Overview

Accounting uses its own HTTP-only session cookie after every successful login. Login methods only prove the user's identity; they do not replace the local session model.

Supported login methods:

- Email and password.
- Email/password plus TOTP for users who have enabled TOTP.
- Discoverable passkey login.
- External SSO through `sso.laisky.com`.

Supporting auth features:

- Email verification for registration.
- Password recovery through one-time email verification codes.
- Cloudflare Turnstile on public auth routes.
- Local session cookies with configurable name, TTL, and Secure flag.

The frontend discovers enabled methods from `GET /api/runtime-config`. Runtime config exposes only public method availability and public browser settings such as Turnstile site key and passkey relying-party metadata.

## Shared Session Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_SESSION_COOKIE_NAME` | `accounting_session` | Browser session cookie name. |
| `ACCOUNTING_AUTH_SESSION_COOKIE_SECURE` | `true` | Marks session and SSO state cookies as Secure. Use `false` only for local plain-HTTP development. |
| `ACCOUNTING_AUTH_SESSION_TTL` | `24h` | Local session lifetime. |
| `ACCOUNTING_AUTH_RATE_LIMIT_ENABLED` | `true` | Enables fixed-window rate limiting for public auth routes. |
| `ACCOUNTING_AUTH_RATE_LIMIT_LIMIT` | `20` | Requests allowed per auth rate-limit window. |
| `ACCOUNTING_AUTH_RATE_LIMIT_WINDOW` | `1m` | Auth rate-limit window. |

Local development usually needs:

```bash
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=false
```

Production deployments behind HTTPS should keep `ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true`.

## Email and Password

Email/password login is enabled by default.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED` | `true` | Enables existing-user email/password login. |
| `ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED` | `true` | Enables public email/password registration. |
| `ACCOUNTING_AUTH_EMAIL_ALLOWED_DOMAINS` | empty | Optional comma-separated allow-list for registration email domains. Empty allows all domains. |
| `ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED` | `true` | Requires email verification before a newly registered user can use the app. |
| `ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL` | `10m` | Lifetime for email verification and password-reset codes. |

SMTP delivery is required when email verification, password recovery, or production registration is enabled:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_EMAIL_SMTP_HOST` | empty | SMTP host. When empty, no SMTP sender is configured. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_PORT` | `587` | SMTP port. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_USERNAME` | empty | SMTP username. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_PASSWORD` | empty | SMTP password. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_FROM` | empty | Sender address. |
| `ACCOUNTING_AUTH_EMAIL_FORCE_SMTP_TLS_VERIFY` | `true` | Verifies SMTP TLS certificates. |

Development without SMTP:

```bash
ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED=false
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=false
```

Email/password-only production:

```bash
ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED=true
ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED=true
ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED=true
ACCOUNTING_AUTH_EMAIL_SMTP_HOST=smtp.example.com
ACCOUNTING_AUTH_EMAIL_SMTP_PORT=587
ACCOUNTING_AUTH_EMAIL_SMTP_USERNAME=accounting@example.com
ACCOUNTING_AUTH_EMAIL_SMTP_PASSWORD=<smtp-password>
ACCOUNTING_AUTH_EMAIL_SMTP_FROM=accounting@example.com
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
```

## Password Recovery

Password recovery uses the email verification-code infrastructure. Keep SMTP configured and `ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL` short enough to limit replay risk.

Public recovery endpoints return generic responses and must not reveal whether an email exists.

## TOTP

TOTP is optional and enabled globally by default. Users enroll TOTP after they are signed in. Password login for a TOTP-enabled user returns a pending challenge until the user submits a valid authenticator code.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_TOTP_ENABLED` | `true` | Enables TOTP setup and password-login challenges. |
| `ACCOUNTING_AUTH_TOTP_ISSUER` | `Accounting` | Issuer name shown by authenticator apps. |
| `ACCOUNTING_AUTH_TOTP_REPLAY_CACHE_DURATION` | `30s` | Short replay cache window for recently used codes. |

Disable TOTP globally:

```bash
ACCOUNTING_AUTH_TOTP_ENABLED=false
```

## Passkeys

Passkey login uses WebAuthn discoverable credentials. Users can register passkeys after login and then sign in without typing a password.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_PASSKEY_ENABLED` | `true` | Enables passkey registration, management, and login. |
| `ACCOUNTING_AUTH_PASSKEY_RP_DISPLAY_NAME` | `Accounting` | Display name shown by passkey authenticators. |
| `ACCOUNTING_AUTH_PASSKEY_RP_ID` | `localhost` | WebAuthn relying-party ID. Use the registrable production domain, such as `accounting.example.com` or `example.com`. |
| `ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN` | `http://localhost:5173` | Exact browser origin allowed for WebAuthn ceremonies. |

Production example:

```bash
ACCOUNTING_AUTH_PASSKEY_ENABLED=true
ACCOUNTING_AUTH_PASSKEY_RP_DISPLAY_NAME=Accounting
ACCOUNTING_AUTH_PASSKEY_RP_ID=accounting.example.com
ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN=https://accounting.example.com
```

Disable passkeys globally:

```bash
ACCOUNTING_AUTH_PASSKEY_ENABLED=false
```

## Turnstile

Cloudflare Turnstile can protect registration and login routes.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_TURNSTILE_ENABLED` | `false` | Enables Turnstile verification. |
| `ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE` | `always` | `always` requires Turnstile for login; `after_failure` requires it after a recent failed login for the same email. |
| `ACCOUNTING_AUTH_TURNSTILE_SITE_KEY` | empty | Public site key exposed to the frontend. |
| `ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY` | empty | Secret key used by the backend to verify tokens. |
| `ACCOUNTING_AUTH_TURNSTILE_VERIFY_URL` | Cloudflare siteverify URL | Verification endpoint. |

Production example:

```bash
ACCOUNTING_AUTH_TURNSTILE_ENABLED=true
ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE=after_failure
ACCOUNTING_AUTH_TURNSTILE_SITE_KEY=<public-site-key>
ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY=<secret-key>
```

## External SSO

External SSO delegates identity proof to another login page and still creates a local Accounting session after callback validation.

Accounting's browser entry point is always:

```text
/api/auth/sso/start
```

The frontend should not build `sso.laisky.com` URLs itself. The backend creates a short-lived anti-CSRF `state`, stores only its hash in an HTTP-only cookie, and redirects to the configured SSO login URL with `redirect_to=<callback-url>`.

### SSO Flow

1. The user clicks the SSO button in Accounting.
2. The browser requests `GET /api/auth/sso/start`.
3. Accounting redirects to `ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL` with `redirect_to` set to `ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL` plus a generated `state`.
4. The SSO provider authenticates the user.
5. The SSO provider redirects to Accounting's callback with `sso_token=<JWT>` and the original `state`.
6. Accounting validates the `state`, verifies the JWT locally, creates its own session cookie, and redirects to `ACCOUNTING_AUTH_EXTERNAL_SSO_SUCCESS_REDIRECT_URL`.

### `sso.laisky.com` Token Contract

Current `sso.laisky.com` tokens are EdDSA JWTs signed with an Ed25519 key. Accounting validates:

- JWT header algorithm is `EdDSA`.
- JWT header type is `JWT`.
- Signature verifies with the configured Ed25519 public key.
- `iss` is `laisky-sso`.
- `exp` and `iat` are valid.
- `jti` is present.
- `sub` and `uid` are present and equal.
- `username` is present.

Accounting maps `username` to the local user email. When auto-provisioning is enabled, the local user id is the UUIDv7 carried in `sub` and `uid`.

### SSO Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED` | `false` | Enables external SSO login. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL` | `https://sso.laisky.com/` | SSO login entry page. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL` | empty | Backend callback URL. Required for non-loopback hosts. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM` | empty | Static Ed25519 public key PEM for local JWT verification. Preferred for production. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_METADATA_URL` | empty | Optional runtime metadata URL for public-key discovery. Use only when static key configuration is not practical. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED` | `true` | Creates a local active user when the SSO identity is trusted and no local user exists. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_COOKIE_NAME` | `accounting_sso_state` | HTTP-only anti-CSRF state cookie name. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_STATE_TTL` | `5m` | State lifetime. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_SUCCESS_REDIRECT_URL` | `/` | Clean URL after successful SSO login. |

Recommended `sso.laisky.com` production configuration:

```bash
ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL=https://sso.laisky.com/
ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL=https://accounting.example.com/api/auth/sso/callback
ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM="-----BEGIN PUBLIC KEY-----\n<base64>\n-----END PUBLIC KEY-----"
ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_SUCCESS_REDIRECT_URL=/
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
```

SSO-only deployment:

```bash
ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED=false
ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED=false
ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL=https://sso.laisky.com/
ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL=https://accounting.example.com/api/auth/sso/callback
ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM="-----BEGIN PUBLIC KEY-----\n<base64>\n-----END PUBLIC KEY-----"
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
```

Metadata fallback deployment:

```bash
ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL=https://sso.laisky.com/
ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL=https://accounting.example.com/api/auth/sso/callback
ACCOUNTING_AUTH_EXTERNAL_SSO_METADATA_URL=https://sso.laisky.com/runtime-config.json
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
```

The static public-key path is preferred because login does not depend on fetching provider metadata at runtime. Rotate the configured key whenever `sso.laisky.com` rotates its signing key.

### SSO Callback Host Rules

For production, always set `ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL`. Without it, Accounting derives a callback URL from the request host only for loopback development hosts. This avoids trusting a client-controlled `Host` header for a callback URL that receives `sso_token`.

The SSO provider must also allow the callback host. `sso.laisky.com` accepts `laisky.com`, `*.laisky.com`, loopback, and documented internal IP ranges. Use a permitted host or update the provider allow-list before release.

## Deployment Profiles

### Local Development

```bash
ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED=false
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=false
ACCOUNTING_AUTH_PASSKEY_RP_ID=localhost
ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN=http://localhost:5173
```

### Private SSO-Only Production

```bash
ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED=false
ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED=false
ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL=https://sso.laisky.com/
ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL=https://accounting.laisky.com/api/auth/sso/callback
ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM="-----BEGIN PUBLIC KEY-----\n<base64>\n-----END PUBLIC KEY-----"
ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED=true
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
ACCOUNTING_AUTH_PASSKEY_ENABLED=true
ACCOUNTING_AUTH_PASSKEY_RP_ID=accounting.laisky.com
ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN=https://accounting.laisky.com
ACCOUNTING_AUTH_TOTP_ENABLED=true
```

### Public Email Login With SSO Alternative

```bash
ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED=true
ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED=true
ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED=true
ACCOUNTING_AUTH_EXTERNAL_SSO_LOGIN_URL=https://sso.laisky.com/
ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL=https://accounting.example.com/api/auth/sso/callback
ACCOUNTING_AUTH_EXTERNAL_SSO_PUBLIC_KEY_PEM="-----BEGIN PUBLIC KEY-----\n<base64>\n-----END PUBLIC KEY-----"
ACCOUNTING_AUTH_TURNSTILE_ENABLED=true
ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE=after_failure
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true
```

## Operational Checks

Before release:

- Confirm `GET /api/runtime-config` shows the intended login methods.
- Confirm public runtime config does not expose SMTP passwords, session tokens, SSO tokens, or the SSO public key.
- Confirm `/api/auth/sso/start` redirects to `sso.laisky.com` with a `redirect_to` callback under the Accounting domain.
- Confirm the SSO callback creates an `ACCOUNTING_AUTH_SESSION_COOKIE_NAME` cookie and redirects to a clean URL without `sso_token`.
- Confirm `ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=true` in HTTPS deployments.
- Confirm SMTP delivery works before enabling verified registration or password recovery.
- Run `make lint`, `make test`, and `make e2e` after auth configuration or auth flow changes.
