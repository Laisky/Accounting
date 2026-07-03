# Accounting

Accounting is a browser-first bookkeeping system for people who want the speed of a personal finance app without giving up ownership, auditability, or migration control.

It combines a fast mobile bookkeeping workflow with a Go API, secure authentication, multi-book collaboration, Wacai import previews, multi-currency reporting, and a CLI-friendly backend. The goal is simple: make daily expense tracking feel lightweight while keeping the data model strong enough for families, shared projects, and small teams.

## Why It Exists

Most personal finance tools are either polished but closed, powerful but accountant-first, or easy to start and hard to leave. Accounting is built around a different set of defaults:

- Your books, accounts, categories, imports, and audit history stay in one transparent system.
- Shared books have explicit membership and role boundaries.
- Reports and summaries understand base currency, exchange rates, and source transaction currency.
- Imports are treated as reviewed migrations, not blind CSV dumps.
- The same repo contains the web app, API server, CLI, tests, and architecture notes.

## Highlights

- Mobile-first workspace for quick entry, accounts, reports, and profile flows.
- Email/password auth, sessions, password recovery, optional TOTP, passkeys, Turnstile, and external SSO.
- Book ownership, member roles, creator-only member edits, and auditable identity boundaries.
- Personal accounts, account groups, book categories, entries, merchants, tags, and notes.
- Base-currency reporting for USD, EUR, CNY, and CAD with a daily USD-relative exchange-rate table.
- Wacai CSV import preview with source hashing, row diagnostics, and staged review.
- Structured Go backend with context-aware logging, race-tested services, and a documented JSON API.
- React 19, TypeScript 6, Vite 8, i18n checks, Vitest, and Playwright-ready E2E setup.

## Tech Stack

| Layer | Choice |
| --- | --- |
| Backend | Go 1.26.4, Gin, Zap-style structured logging, OpenTelemetry hooks |
| Frontend | React 19, TypeScript 6, Vite 8, i18next, lucide-react |
| Auth | HttpOnly sessions, PBKDF2-SHA256, TOTP, WebAuthn passkeys, optional SSO |
| Storage | In-memory and JSON-file stores for local development; direct SQLite/PostgreSQL stores under `backend/internal/persistence` |
| Testing | Go race tests with coverage, Vitest, Testing Library, Playwright E2E |

## Quick Start

Prerequisites:

- Go 1.26.4
- Node.js and pnpm compatible with the `web/package.json` toolchain
- `make`

Install frontend dependencies once:

```bash
pnpm --dir web install
```

Run the local app:

```bash
ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED=false \
ACCOUNTING_AUTH_SESSION_COOKIE_SECURE=false \
make dev
```

Open `http://localhost:5173`.

Those two environment variables are local-development conveniences: email verification is disabled because no SMTP delivery is configured by default, and secure cookies are disabled because the dev server runs on plain HTTP. Do not use those settings for production.

## Common Commands

```bash
make dev              # Run Go API and Vite dev server.
make build            # Build frontend, backend, and CLI.
make lint             # gofmt, go mod tidy, go vet, ESLint, and i18n checks.
make test             # Backend race tests, CLI race tests, and frontend unit tests.
make e2e              # Playwright E2E tests.
```

Useful narrower commands:

```bash
cd backend && go test -race -cover ./...
cd cli && go test -race -cover ./...
pnpm --dir web run build
pnpm --dir web run test
```

## Repository Layout

```text
.
├── backend/     # Go API server, domain services, auth, import, audit, telemetry.
├── cli/         # Go CLI module for operator and automation workflows.
├── docs/arch/   # Architecture notes and product/API decisions.
├── web/         # React/Vite frontend.
└── Makefile     # Root development, lint, test, and build targets.
```

## Runtime Shape

Development keeps iteration fast:

```text
Browser -> Vite :5173 -> /api proxy -> Go backend :8080
```

Production is intentionally simple:

```text
Browser -> Go backend
           ├── /api/*    JSON API
           ├── /assets/* built frontend assets
           └── /*        SPA fallback
```

## Product Model

Accounting is organized around a few durable concepts:

- A user owns identity, auth settings, accounts, import jobs, and audit history.
- A book represents a personal, family, shared living, trip, project, or small-team ledger.
- Book members have explicit roles: owner, administrator, member, or viewer.
- Entries belong to one book and preserve creator, account, category, transaction currency, reporting currency, and exchange-rate context.
- Imports preserve raw source data so parser improvements can repair or enrich older migrations later.

For the full API and architecture notes, see [docs/arch/arch.md](docs/arch/arch.md).

## Configuration

The backend is environment-configured. Important local and deployment settings include:

| Variable | Purpose |
| --- | --- |
| `ACCOUNTING_ADDR` | Backend listen address. Defaults to `:8080`. |
| `ACCOUNTING_WEB_DIST_DIR` | Built frontend directory served by the Go server. |
| `ACCOUNTING_PERSISTENCE_DRIVER` | Storage driver: `memory`, `file`, `sqlite`, `postgres`, or `postgresql`. Defaults to `memory`. |
| `ACCOUNTING_PERSISTENCE_DIR` | JSON-file persistence directory and default SQLite database directory. |
| `ACCOUNTING_DATABASE_URL` | PostgreSQL URL or optional SQLite path/`sqlite://` URL. Falls back to `DATABASE_URL`. |
| `ACCOUNTING_AUTH_SESSION_COOKIE_SECURE` | Must be `true` behind HTTPS in production. |
| `ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED` | Require verified email before active use. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED` | Enable configured external SSO login. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_SHARED_SECRET` | HS256 secret shared with the SSO provider to verify `sso_token` JWTs locally. Required when SSO is enabled; must match the provider secret and be at least 32 bytes. |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_CALLBACK_URL` | Backend SSO callback URL. Required for non-loopback hosts. |

## Quality Bar

The repo is designed to keep changes shippable:

- Backend and CLI tests run with `-race -cover`.
- Go code is formatted and vetted through `make lint`.
- Frontend linting, TypeScript build, unit tests, and i18n checks are part of the normal workflow.
- Request paths use context-aware structured logging.
- Secrets, password hashes, tokens, and verification codes are never returned by APIs or logged.

## Roadmap

- Expand the SQL schema from JSON-backed records into relational accounting tables as reporting and double-entry invariants mature.
- Add invite flows and richer member management.
- Complete Wacai spreadsheet import mapping and import commit workflows.
- Add budget, recurring entry, refund, reimbursement, borrowing, and transfer workflows.
- Improve report drilldowns with exportable summaries and saved report views.
- Package production deployment examples.

## License

Accounting is released under the [MIT License](LICENSE).
