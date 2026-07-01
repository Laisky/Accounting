# Accounting Architecture

## Overview

Accounting is a single-repository web application for personal, family, and small-team bookkeeping. The backend is written in Go, the frontend is a React/Vite application, and production deployments run as one Go web server that serves both API routes and the built static frontend from `web/dist`.

The product target is a browser-first personal finance system with users, authentication, authorization, multi-book collaboration, account management, multi-currency support, a complete API surface, and migration paths from Wacai-style bookkeeping data.

## Goals

- Provide a browser-first bookkeeping application for entering, reviewing, reconciling, importing, exporting, and reporting financial transactions.
- Support a full user system with registration, login, session management, account recovery, authorization, and auditable identity boundaries.
- Give each user independent personal data while allowing each user to own multiple books and collaborate in books owned by other users.
- Support invite-based book membership with roles, including administrators who can manage the whole book and members who can edit only the entries they created.
- Support income and expense entries with top-level directions and configurable subcategories such as dining, groceries, household goods, salary, bonus, and reimbursements.
- Support personal financial accounts grouped by user-defined account groups such as cash, savings, credit cards, loans, investments, and payment platforms.
- Support multi-currency accounts and entries without assuming that all books use a single currency.
- Provide a stable, documented JSON API for the webapp first and for future CLI/import automation second.
- Support smooth migration from Wacai-style bookkeeping, including imports from Wacai exported spreadsheet data when available.
- Keep backend, CLI, frontend, documentation, and deployment files in one repository.
- Let `make dev` start a local Go API server and Vite frontend server for fast iteration.
- Let production builds copy `web/dist` beside the Go binary so the Go web server can serve the SPA.
- Mirror proven conventions from nearby projects: clear `docs/arch` ownership, Go domain packages under `internal`, React under `web`, structured logs, context propagation, and practical Make targets.

## Non-Goals

- This is not an enterprise accounting suite with payroll, invoicing, tax filing, procurement, or general-ledger close workflows.
- This is not a bank-connected aggregation product in the initial architecture; manual entry, file import, and user-reviewed normalization come first.
- This is not a blind clone of Wacai. The compatibility target is user migration and familiar bookkeeping workflows, not reproducing private implementation details or proprietary UI.
- This is not a mobile-native application.
- This is not an offline-first architecture.

## Repository Layout

```text
.
├── backend/                 # Go web server module.
│   ├── cmd/accounting-server
│   └── internal/
│       ├── config           # Environment-backed runtime settings.
│       ├── httpserver       # Gin router, middleware, API handlers, SPA serving.
│       ├── ledger           # Accounting domain use cases.
│       └── logger           # Global logger foundation and context logger fallback.
├── cli/                     # Go CLI module for operator and automation workflows.
│   ├── cmd/accounting
│   └── internal/app
├── docs/
│   └── arch/                # Long-lived architecture notes.
├── web/                     # React, TypeScript, Vite frontend.
│   └── src
└── Makefile                 # Root development, build, lint, and test commands.
```

## Runtime Shape

```text
Development

Browser ──> Vite dev server :5173 ──/api proxy──> Go backend :8080

Production

Browser ───────────────> Go backend
                         ├── /api/*        JSON APIs
                         ├── /assets/*     built frontend assets
                         └── /*            SPA index fallback
```

Development keeps frontend reloads fast. Production keeps deployment simple by making the Go server the only public process.

## Product Requirements

The core product model is user-centered bookkeeping:

- A user owns identity, authentication credentials, personal preferences, personal accounts, import jobs, and audit history.
- A user can create multiple books.
- A book can invite other users and can represent a personal, family, shared living, trip, project, or small-business scenario.
- Book membership must be explicit. The minimum roles are owner, administrator, member, and viewer.
- Book owners and administrators can manage book settings, categories, members, and all entries in the book.
- Book members can create entries and can edit or delete only entries they created.
- Book viewers can read book data but cannot create or mutate entries.
- Each entry belongs to exactly one book and has a creator.
- Entries must support income and expense as first-class directions. The model must also leave room for transfer, refund, reimbursement, borrow, lend, and repayment flows because Wacai-style data contains those concepts.
- Income and expense categories are separate trees. Top-level categories group subcategories such as dining, groceries, household goods, salary, bonus, interest, and reimbursements.
- Categories must be book-configurable and importable. Imports must preserve raw source category names when a normalized category mapping is uncertain.
- Accounts are personal to a user by default. A book entry can reference an account owned by the entry creator without making that account visible to every book member unless the user explicitly shares or exposes it through an aggregate view.
- Accounts can be grouped by user-defined groups such as cash, savings, credit cards, loans, investments, payment platforms, stored-value cards, and receivables.
- Accounts must carry currency. Entries must carry transaction currency, account currency, book reporting currency, and exchange-rate metadata when they differ.
- The webapp is the primary interface. CLI workflows should support administration, diagnostics, import automation, and future batch operations.

## Wacai Migration Compatibility

Wacai compatibility is a migration requirement, not a dependency. Public Wacai-facing information and third-party migration notes show that a smooth migration needs more than a flat income/expense CSV importer.

The import system should support:

- Spreadsheet imports, especially `.xlsx` and `.csv` files exported from Wacai or manually converted from Wacai exports.
- A staging preview that shows parsed rows, detected books, accounts, categories, currencies, members, merchants, tags, and unmapped fields before writing durable records.
- Idempotent imports keyed by source file hash, source row identity when present, normalized timestamp, amount, account, book, and source-specific identifiers.
- Per-row validation errors and warnings. Invalid rows must not block valid rows from being reviewed, but committed imports must be explicit.
- Mapping tables for source book, account, category, member, merchant, tag, and currency values.
- Raw-source retention for imported rows so future parser improvements can repair or enrich historical imports.
- Rollback or compensating delete for an import batch before the user starts editing imported entries individually.

The importer should expect these Wacai-style concepts:

- Books scoped by scenario and time range.
- Bills or transactions with type, datetime, amount, category, account, book, note, and creator/member.
- Transfers with source and destination accounts.
- Refund and reimbursement states.
- Borrowing, lending, repayment, counterparty, due date, and interest metadata.
- Merchants, tags, attachments, recurring rules, quick-entry templates, budgets, and reminders.
- Account types including cash, savings/debit cards, credit cards, investment accounts, online wallets, stored-value cards, loans, receivables, and payables.

Import caveats:

- Public sources confirm Wacai feature concepts and Excel-style export availability, but they do not guarantee a stable export schema.
- Third-party notes about Wacai local databases and exported tables are useful for field discovery but must be treated as version-specific evidence.
- The first implementation should use schema detection and explicit user mapping rather than assuming one canonical Wacai export layout.
- Automated migration promises must be verified against current Wacai export behavior before release because export access and file shape may change.

Public research anchors:

- [Wacai iOS app listing](https://apps.apple.com/cn/app/%E6%8C%96%E8%B4%A2%E8%AE%B0%E8%B4%A6-ai%E8%87%AA%E5%8A%A8%E8%AE%B0%E8%B4%A6/id1544045905): current public feature positioning for bill types, shared books, reports, budgets, assets, multi-currency, and import features.
- [Wacai personal information list](https://8.wacai.com/ikebana/contract/1DA41D9573A243EB373E1F9D9D3DD765): field-level evidence for bills, accounts, books, budgets, recurring rules, plans, import/export, and AI-assisted entry.
- [Wacai web app page](https://www.wacai.com/index/app.action): public evidence for web access, account sync, time-range export, and Excel-format export.
- [Third-party Wacai SQLite export notes](https://editst.com/2020/export-wacai-data/): version-specific evidence for observed local tables and CSV migration fields.

## Backend Architecture

The backend uses Gin because the reference projects already use Gin, `github.com/Laisky/gin-middlewares/v7`, and request-scoped structured logging. The server is assembled in `backend/internal/httpserver.NewServer`.

Key responsibilities:

- `cmd/accounting-server`: process startup, signal handling, graceful shutdown.
- `internal/config`: environment parsing with explicit defaults.
- `internal/logger`: global structured logger foundation plus `FromContext` for non-Gin business code.
- `internal/httpserver`: middleware, route registration, SPA fallback, and development proxy hooks.
- `internal/ledger`: domain use cases for books, entries, accounts, categories, imports, reporting, and future double-entry rules.

Request handlers must retrieve the request logger with `gmw.GetLogger(c)` once per handler and pass `c.Request.Context()` into business services. Business logic should use context-aware loggers, not package globals.

Authentication and authorization must be enforced in middleware and service-level policy checks. Handlers should authenticate the user, load book membership when the route is book-scoped, and call domain services with an actor object that contains the user id, membership role, and request context.

Sensitive values such as passwords, reset tokens, invite tokens, API keys, and session secrets must never be logged. Token and signature comparisons must use constant-time comparison. Password hashing must follow OWASP guidance and use at least 10,000 iterations in this project context.

## Authentication Architecture

Accounting should follow the proven `one-api` authentication shape while keeping the product focused on email-first bookkeeping:

- Email/password registration and login are first-class and independently configurable.
- Registration can be disabled globally without disabling existing-user login.
- Email verification should be required by default before a newly registered user can use the application.
- Password reset should use one-time email verification codes with expiry and single-use semantics.
- Cloudflare Turnstile must be supported on registration and login. The default product posture should bind Turnstile to both routes when enabled, while allowing an `after_failure` login mode that requires Turnstile after a recent failed login for the same email.
- Sessions should be HTTP-only, SameSite=Lax cookies. Production deployments must use Secure cookies.
- Session values should contain only stable user identity and role/status data, not secrets.
- Password hashes must use OWASP-compliant parameters and must never be returned by any API.
- Auth routes must not reveal whether an email exists when credentials, reset requests, or verification requests fail.
- TOTP is an optional per-user MFA method. Setup stores the generated secret only in the session until the user confirms a valid code.
- TOTP verification must be rate-limited and must reject recently reused codes through a short replay cache, using PostgreSQL or Redis-compatible storage once available. The fallback in-memory cache is acceptable only for local development.
- Passkey login is an optional WebAuthn flow. Users can register multiple passkeys after login, and passkey login should support discoverable credentials.
- Passkey credentials must store raw credential id, public key, sign count, backup flags, transports, AAGUID, human label, and timestamps. Private key material is never stored by the server.
- Admins may disable another user's TOTP or passkeys for account recovery, but these actions must be audited.
- The webapp must show available login methods based on public runtime config, but public config must expose only non-secret values such as Turnstile site key and passkey relying-party metadata.

Initial auth API shape:

- `POST /api/auth/register`: email/password registration with Turnstile and optional email verification code.
- `POST /api/auth/login`: email/password login with Turnstile and optional `totp_code`.
- `POST /api/auth/logout`: clears the active session.
- `GET /api/auth/email/verification`: sends a verification code for registration or email binding.
- `POST /api/auth/password-reset/request`: sends a password reset code.
- `POST /api/auth/password-reset/confirm`: verifies a reset code and updates the password.
- `GET /api/auth/totp/status`: returns whether the authenticated user has TOTP enabled.
- `POST /api/auth/totp/setup`: creates a temporary TOTP secret and returns an otpauth URI.
- `POST /api/auth/totp/confirm`: verifies the temporary TOTP code and stores the secret.
- `POST /api/auth/totp/disable`: verifies a TOTP code and disables TOTP.
- `POST /api/auth/passkeys/login/begin`: starts a discoverable WebAuthn login ceremony.
- `POST /api/auth/passkeys/login/finish`: completes a discoverable WebAuthn login ceremony and creates a session.
- `GET /api/auth/passkeys`: lists the authenticated user's passkeys.
- `POST /api/auth/passkeys/register/begin`: starts WebAuthn registration for the authenticated user.
- `POST /api/auth/passkeys/register/finish`: stores the confirmed passkey credential.
- `PUT /api/auth/passkeys/{id}`: renames a passkey owned by the authenticated user.
- `DELETE /api/auth/passkeys/{id}`: deletes a passkey owned by the authenticated user.

## Frontend Architecture

The frontend is a Vite React app under `web/`. The first screen is an operational bookkeeping workspace, not a marketing landing page.

Expected direction:

- `src/main.tsx` owns bootstrapping.
- Authentication screens should be real application screens: login, registration, account recovery, and invite acceptance.
- The primary authenticated layout should make book switching, account context, quick entry, transaction review, and import review immediately available.
- Shared UI should move into `src/components` once it is reused.
- Feature slices should live under `src/features/<feature>`.
- API clients should live under `src/lib/api`.
- Build output stays in `web/dist` and is never hand-edited.

Vite proxies `/api` to the Go server during local development. The Go server serves `web/dist` in production or whenever the built directory exists locally.

## API Boundary

Initial endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Process health check. |
| `GET` | `/api/runtime-config` | Public runtime settings for the frontend. |
| `GET` | `/api/ledger/summary` | Placeholder ledger aggregate for the first UI. |

Future APIs should be grouped by domain and versioned once the first persistent contract is ready:

- `/api/auth`: email registration, login, logout, session refresh, email verification, account recovery, TOTP, passkeys, and invite acceptance.
- `/api/users`: profile, preferences, and identity-safe user lookup for invitations.
- `/api/books`: book creation, settings, base currency, membership, invitations, and role management.
- `/api/books/{bookID}/entries`: income, expense, transfer, refund, reimbursement, borrow, lend, and repayment entries.
- `/api/books/{bookID}/categories`: income and expense category trees, category mapping, and display metadata.
- `/api/accounts`: personal accounts, account groups, balances, currency, credit-card settings, and visibility settings.
- `/api/imports`: upload, parse, preview, mapping, commit, rollback, and import-batch status.
- `/api/reconciliation`: statement-period matching and lock records.
- `/api/reports`: book-level and personal reports by category, account, member, merchant, tag, and time range.
- `/api/audit`: user-visible audit events for sensitive account, book, membership, and import changes.

API design rules:

- Every mutating route must authenticate the actor and enforce role-based authorization.
- Book-scoped routes must reject access when the actor is not a member of the book.
- Entry update and delete operations must enforce creator ownership unless the actor is a book owner or administrator.
- Public auth routes must apply critical rate limits and Turnstile checks when configured.
- Request and response bodies must use explicit JSON schemas. Unknown client fields should fail closed on mutating requests after the API contract is stable.
- List routes must paginate and bound page sizes to prevent accidental memory pressure.
- Filters must be validated and normalized before they reach repository code.
- Complex reads and joins should use explicit SQL or query-builder code owned by repositories, not controller-built query fragments.

Date-range filters must use UTC and include the full final day by ending just before `00:00` of the next day.

## Data Model Direction

The durable model should support Wacai-style cashflow bookkeeping first while leaving a path toward double-entry internals for reconciliation and reporting.

Core entities:

- User: identity, authentication settings, profile, preferences, and audit actor.
- Session: authenticated browser session with expiry, rotation, and revocation.
- Book: bookkeeping workspace with owner, settings, base reporting currency, category policy, and membership.
- Book member: user-to-book relationship with role, display name, invite status, and timestamps.
- Book invitation: invite token metadata, role, inviter, expiry, accepted user, and revocation state.
- Entry: user-facing bill or transaction with book, creator, type, occurred-at time in UTC, amount, currency, category, account references, note, merchant, tags, and source metadata.
- Category: book-owned income or expense tree node with parent, display metadata, sort order, and archived state.
- Account group: user-owned grouping for personal accounts.
- Account: user-owned financial account with type, group, currency, name, provider metadata, balance settings, and optional credit or statement settings.
- Exchange rate: rate source, base currency, quote currency, effective time, and precision metadata.
- Import batch: uploaded source file, parser version, source hash, detected schema, mapping decisions, row results, commit status, and rollback state.
- Audit event: security-relevant and data-changing events with actor, target, action, timestamp, and sanitized metadata.

Entry types should include at least:

- `expense`: money leaves the selected account and contributes to expense reporting.
- `income`: money enters the selected account and contributes to income reporting.
- `transfer`: money moves between two accounts and does not count as income or expense.
- `refund`: reverses or offsets an expense while preserving the original spending context.
- `reimbursement`: tracks money expected from or received from another party.
- `borrow`: records money received from another party with repayment expectations.
- `lend`: records money paid to another party with collection expectations.
- `repayment`: closes or reduces borrow/lend balances.

The future double-entry layer can model:

- Account ledger: asset, liability, equity, income, and expense ledgers derived from user accounts and categories.
- Journal entry: immutable business event with occurred-at time in UTC.
- Posting: one debit or credit line within a journal entry.
- Reconciliation: statement-period matching, balance checks, and lock records.

The first persistent store should be hidden behind repository interfaces owned by the relevant domain package. Controllers should not build SQL directly. Repositories must validate untrusted inputs, bind query parameters, and avoid building queries from raw user-provided strings.

## Configuration

Current backend environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_ADDR` | `:8080` | Go HTTP listen address. |
| `ACCOUNTING_DEBUG` | `false` | Enables debug logging and Gin debug mode. |
| `ACCOUNTING_SERVER_NAME` | `accounting` | Public server label returned in runtime config. |
| `ACCOUNTING_WEB_DIST_DIR` | `../web/dist` | Built SPA directory served by the Go process. |
| `ACCOUNTING_WEB_DEV_URL` | empty | Optional reverse proxy target for non-API frontend routes. |
| `ACCOUNTING_AUTH_EMAIL_REGISTER_ENABLED` | `true` | Enables self-service email/password registration. |
| `ACCOUNTING_AUTH_EMAIL_LOGIN_ENABLED` | `true` | Enables email/password login for existing users. |
| `ACCOUNTING_AUTH_EMAIL_VERIFICATION_REQUIRED` | `true` | Requires email verification during registration. |
| `ACCOUNTING_AUTH_EMAIL_VERIFICATION_TTL` | `10m` | Verification and reset code lifetime. |
| `ACCOUNTING_AUTH_EMAIL_ALLOWED_DOMAINS` | empty | Optional comma-separated registration email domain allowlist. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_HOST` | empty | SMTP host for verification and reset emails. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_PORT` | `587` | SMTP port. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_USERNAME` | empty | SMTP username. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_PASSWORD` | empty | SMTP password; never log or return this value. |
| `ACCOUNTING_AUTH_EMAIL_SMTP_FROM` | empty | Sender address for auth emails. |
| `ACCOUNTING_AUTH_EMAIL_FORCE_SMTP_TLS_VERIFY` | `true` | Requires SMTP TLS certificate verification. |
| `ACCOUNTING_AUTH_TURNSTILE_ENABLED` | `false` | Enables Cloudflare Turnstile checks for auth flows. |
| `ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE` | `always` | Login Turnstile mode: `always` or `after_failure`. |
| `ACCOUNTING_AUTH_TURNSTILE_SITE_KEY` | empty | Public Turnstile site key for the webapp. |
| `ACCOUNTING_AUTH_TURNSTILE_SECRET_KEY` | empty | Turnstile secret key; never log or return this value. |
| `ACCOUNTING_AUTH_TURNSTILE_VERIFY_URL` | Cloudflare siteverify URL | Turnstile verification endpoint. |
| `ACCOUNTING_AUTH_TOTP_ENABLED` | `true` | Enables per-user TOTP MFA setup and login challenge. |
| `ACCOUNTING_AUTH_TOTP_ISSUER` | `Accounting` | Issuer label in otpauth URIs. |
| `ACCOUNTING_AUTH_TOTP_REPLAY_CACHE_DURATION` | `30s` | Time window used to reject reused TOTP codes. |
| `ACCOUNTING_AUTH_PASSKEY_ENABLED` | `true` | Enables WebAuthn passkey registration and login. |
| `ACCOUNTING_AUTH_PASSKEY_RP_DISPLAY_NAME` | `Accounting` | WebAuthn relying-party display name. |
| `ACCOUNTING_AUTH_PASSKEY_RP_ID` | `localhost` | WebAuthn relying-party id; must match the serving domain. |
| `ACCOUNTING_AUTH_PASSKEY_RP_ORIGIN` | `http://localhost:5173` | WebAuthn origin for browser ceremonies. |
| `ACCOUNTING_ENABLE_PPROF` | `false` | Enables a dedicated `net/http/pprof` listener. |
| `ACCOUNTING_PPROF_LISTEN` | `localhost:6060` | pprof bind address; keep loopback unless protected by a firewall or auth proxy. |
| `ACCOUNTING_OTEL_ENABLED` | `false` | Enables OpenTelemetry tracing with Gin instrumentation. |
| `ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT` | empty | OTLP HTTP collector endpoint, without a required URL scheme. |
| `ACCOUNTING_OTEL_EXPORTER_OTLP_INSECURE` | `true` | Uses insecure OTLP transport for internal collectors. |
| `ACCOUNTING_OTEL_SERVICE_NAME` | `accounting` | Service name emitted in OpenTelemetry traces. |
| `ACCOUNTING_OTEL_ENVIRONMENT` | `debug` | Deployment environment label emitted in OpenTelemetry resources. |
| `ACCOUNTING_LOG_PUSH_API` | empty | Optional alertpusher API endpoint for error-level log notifications. |
| `ACCOUNTING_LOG_PUSH_TYPE` | empty | Optional alertpusher backend type. |
| `ACCOUNTING_LOG_PUSH_TOKEN` | empty | Optional alertpusher token; never log or return this value. |
| `ACCOUNTING_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown deadline for HTTP, pprof, and telemetry providers. |

Secrets must not be logged or returned by runtime config endpoints.

## Observability

Accounting follows the `one-api` operational pattern for backend observability:

- The global Zap logger is initialized during process startup and then enhanced with a host field and optional alertpusher hook.
- Request handlers use `gmw.GetLogger(c)` once per handler, while business logic uses context-aware logger lookup.
- Alertpusher is opt-in through environment variables and only receives error-level logs through a rate-limited hook.
- OpenTelemetry tracing is opt-in. When enabled, the backend initializes an OTLP HTTP trace exporter and installs Gin tracing middleware before request logging.
- pprof is opt-in and served from a dedicated HTTP listener, not from the public API router.
- The pprof listener defaults to `localhost:6060`. Binding it to a non-loopback address emits a warning because pprof has no built-in authentication.
- Shutdown drains the API server, pprof listener, and telemetry provider within the configured shutdown timeout.

## Engineering Standards

- Go code targets Go 1.26.4.
- Manually written code files must stay under 800 lines. Go files should stay under 600 lines when feasible and should split by responsibility before they become hard to review.
- Every function and interface must have a comment that starts with its name and describes purpose, parameters, and return values in complete sentences.
- Tests should use `github.com/stretchr/testify/require` for assertions.
- Errors must be wrapped with `github.com/Laisky/errors/v2`; do not return bare errors from application code.
- Each error must be processed exactly once: either return it or log it, but do not log and return the same error.
- Debug logs should be targeted and useful for diagnosis, and must never include secrets.
- Business logic must accept and propagate `context.Context` wherever lifecycle or cancellation matters.
- Server, database, and API time handling must use UTC.

## Developer Commands

```sh
make dev
make build
make lint
make test
```

Required final gates after code changes:

```sh
make lint
cd backend && go test -race -cover ./...
cd ../cli && go test -race -cover ./...
```

Equivalent separate invocations are also acceptable:

```sh
cd backend && go test -race -cover ./...
cd cli && go test -race -cover ./...
```

## Operational Notes

- The Go process handles graceful shutdown on `SIGINT` and `SIGTERM`.
- Static asset misses return `404`; browser navigations fall back to `index.html`.
- API misses under `/api/` return JSON `404` responses instead of the SPA.
- Logs are structured Zap-style logs through the shared logger and request middleware.
- Server, database, and API times must be handled in UTC.

## Next Architecture Decisions

- Choose the persistence layer and migration tool.
- Define authentication and session strategy.
- Define the first durable API versioning strategy and request/response schema format.
- Specify the double-entry posting invariants.
- Specify the Wacai import parser strategy, sample-file collection process, mapping UX, and rollback behavior.
- Add import/export formats for bank statements after the Wacai migration path is defined.
- Add a deployment document once Docker and CI exist.
