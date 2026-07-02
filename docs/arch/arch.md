# Accounting Architecture

## Overview

Accounting is a single-repository web application for personal, family, and small-team bookkeeping. The backend is written in Go, the frontend is a React/Vite application, and production deployments run as one Go web server that serves both API routes and the built static frontend from `web/dist`.

The product target is a browser-first personal finance system with users, authentication, authorization, multi-book collaboration, account management, multi-currency support, a complete API surface, and an import path for data exported from Wacai.

## Goals

- Provide a browser-first bookkeeping application for entering, reviewing, reconciling, importing, exporting, and reporting financial transactions.
- Support a full user system with registration, login, session management, account recovery, authorization, and auditable identity boundaries.
- Give each user independent personal data while allowing each user to own multiple books and collaborate in books owned by other users.
- Support invite-based book membership with roles, including administrators who can manage the whole book and members who can edit only the entries they created.
- Support income and expense entries with top-level directions and configurable subcategories such as dining, groceries, household goods, salary, bonus, and reimbursements.
- Support personal financial accounts grouped by user-defined account groups such as cash, savings, credit cards, loans, investments, and payment platforms.
- Support multi-currency accounts and entries without assuming that all books use a single currency.
- Provide a stable, documented JSON API for the webapp first and for future CLI/import automation second.
- Support importing data exported from Wacai, including spreadsheet exports when available.
- Keep backend, CLI, frontend, documentation, and deployment files in one repository.
- Let `make dev` start a local Go API server and Vite frontend server for fast iteration.
- Let production builds copy `web/dist` beside the Go binary so the Go web server can serve the SPA.
- Mirror proven conventions from nearby projects: clear `docs/arch` ownership, Go domain packages under `internal`, React under `web`, structured logs, context propagation, and practical Make targets.

## Non-Goals

- This is not an enterprise accounting suite with payroll, invoicing, tax filing, procurement, or general-ledger close workflows.
- This is not a bank-connected aggregation product in the initial architecture; manual entry, file import, and user-reviewed normalization come first.
- This is not a clone of any existing bookkeeping product. The focus is familiar, general bookkeeping workflows, not reproducing another product's implementation details or proprietary UI.
- This is not a mobile-native application.
- This is not an offline-first architecture.

## Repository Layout

```text
.
├── backend/                 # Go web server module.
│   ├── cmd/accounting-server
│   └── internal/
│       ├── audit            # User-visible audit events for security and data changes.
│       ├── config           # Environment-backed runtime settings.
│       ├── httpserver       # Gin router, middleware, API handlers, SPA serving.
│       ├── imports          # Import preview parsing and import-batch staging.
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
- Entries must support income and expense as first-class directions. The model must also leave room for transfer, refund, reimbursement, borrow, lend, and repayment flows because imported bookkeeping data commonly contains those concepts.
- Income and expense categories are separate trees. Top-level categories group subcategories such as dining, groceries, household goods, salary, bonus, interest, and reimbursements.
- Categories must be book-configurable and importable. Imports must preserve raw source category names when a normalized category mapping is uncertain.
- Accounts are personal to a user by default. A book entry can reference an account owned by the entry creator without making that account visible to every book member unless the user explicitly shares or exposes it through an aggregate view.
- Accounts can be grouped by user-defined groups such as cash, savings, credit cards, loans, investments, payment platforms, stored-value cards, and receivables.
- Accounts must carry currency. Entries must carry transaction currency, account currency, book reporting currency, and exchange-rate metadata when they differ.
- The webapp is the primary interface. CLI workflows should support administration, diagnostics, import automation, and future batch operations.

## Import Compatibility

Importing existing bookkeeping data is a migration convenience, not a runtime dependency. A smooth migration needs more than a flat income/expense CSV importer.

The import system should support:

- Initial authenticated CSV preview for Wacai exports through `POST /api/imports/wacai/preview`, using a multipart upload field named `file`.
- Later spreadsheet imports, especially `.xlsx` files exported from Wacai or manually converted from Wacai exports.
- A staging preview that shows parsed rows, detected books, accounts, categories, currencies, members, merchants, tags, and unmapped fields before writing committed ledger records.
- Idempotent preview batches keyed by user, source, and source file hash, with later committed imports also considering source row identity when present, normalized timestamp, amount, account, book, and source-specific identifiers.
- Per-row validation errors and warnings. Invalid rows must not block valid rows from being reviewed, but committed imports must remain explicit.
- Mapping tables for source book, account, category, member, merchant, tag, and currency values.
- Raw-source retention for imported rows so future parser improvements can repair or enrich historical imports.
- Rollback or compensating delete for an import batch before the user starts editing imported entries individually.

The importer should expect these bookkeeping concepts:

- Books scoped by scenario and time range.
- Bills or transactions with type, datetime, amount, category, account, book, note, and creator/member.
- Transfers with source and destination accounts.
- Refund and reimbursement states.
- Borrowing, lending, repayment, counterparty, due date, and interest metadata.
- Merchants, tags, attachments, recurring rules, quick-entry templates, budgets, and reminders.
- Account types including cash, savings/debit cards, credit cards, investment accounts, online wallets, stored-value cards, loans, receivables, and payables.

Import caveats:

- Source exports may offer CSV and Excel-style formats, but no external tool guarantees a stable export schema.
- Observed export tables and fields are useful for field discovery but must be treated as version-specific evidence.
- The first implementation should use schema detection and explicit user mapping rather than assuming one canonical export layout.
- Automated migration promises must be verified against current export behavior before release because export access and file shape may change.

## Backend Architecture

The backend uses Gin because the reference projects already use Gin, `github.com/Laisky/gin-middlewares/v7`, and request-scoped structured logging. The server is assembled in `backend/internal/httpserver.NewServer`.

Key responsibilities:

- `cmd/accounting-server`: process startup, signal handling, graceful shutdown.
- `internal/audit`: sanitized audit event recording and actor-scoped audit reads.
- `internal/config`: environment parsing with explicit defaults.
- `internal/logger`: global structured logger foundation plus `FromContext` for non-Gin business code.
- `internal/httpserver`: middleware, route registration, SPA fallback, and development proxy hooks.
- `internal/imports`: import preview parsing, source hashing, parser diagnostics, and import-batch staging.
- `internal/ledger`: domain use cases for books, entries, accounts, categories, reporting, and future double-entry rules.

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
- External SSO login is optional and configuration-driven. When enabled, the backend redirects users to the configured SSO login URL, validates the returned `sso_token` server-side through the configured SSO GraphQL `WhoAmI` endpoint, maps the SSO username to a local active user, and can provision that local user automatically when configured.
- Admins may disable another user's TOTP or passkeys for account recovery, but these actions must be audited.
- The webapp must show available login methods based on public runtime config, but public config must expose only non-secret values such as Turnstile site key, passkey relying-party metadata, and the local SSO start path.

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
- `GET /api/auth/sso/start`: creates a short-lived anti-CSRF state cookie and redirects to the configured external SSO login URL.
- `GET /api/auth/sso/callback`: validates state and `sso_token`, creates a local session cookie, and redirects to a clean application URL.
- `GET /api/auth/passkeys`: lists the authenticated user's passkeys with bounded pagination.
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
| `POST` | `/api/auth/register` | Email/password registration with optional pending email verification. |
| `POST` | `/api/auth/login` | Email/password login that creates an HttpOnly session cookie. |
| `POST` | `/api/auth/logout` | Session revocation and cookie clearing. |
| `GET` | `/api/auth/session` | Authenticated session and actor read. |
| `GET` | `/api/auth/email/verification` | Email verification code request with generic non-secret response. |
| `POST` | `/api/auth/email/verification` | Email verification code confirmation that activates pending users. |
| `POST` | `/api/auth/password-reset/request` | Password reset code request with generic non-secret response. |
| `POST` | `/api/auth/password-reset/confirm` | Password reset code confirmation that updates the password. |
| `GET` | `/api/auth/totp/status` | Authenticated TOTP enabled-state read. |
| `POST` | `/api/auth/totp/setup` | Authenticated TOTP setup start with pending session-scoped secret and otpauth URI. |
| `POST` | `/api/auth/totp/confirm` | Authenticated TOTP setup confirmation with one-time code validation. |
| `POST` | `/api/auth/totp/disable` | Authenticated TOTP disable with one-time code validation. |
| `POST` | `/api/auth/passkeys/login/begin` | Starts a discoverable WebAuthn passkey login ceremony. |
| `POST` | `/api/auth/passkeys/login/finish` | Completes a discoverable WebAuthn passkey login ceremony and creates an HttpOnly session cookie. |
| `GET` | `/api/auth/sso/start` | Starts configured external SSO login with a short-lived state cookie and provider redirect. |
| `GET` | `/api/auth/sso/callback` | Validates the external SSO token server-side, creates a local session cookie, and redirects to a clean URL. |
| `GET` | `/api/auth/passkeys` | Authenticated paginated list of public passkey metadata for the current user. |
| `POST` | `/api/auth/passkeys/register/begin` | Starts an authenticated WebAuthn passkey registration ceremony. |
| `POST` | `/api/auth/passkeys/register/finish` | Completes an authenticated WebAuthn passkey registration ceremony and stores the public credential. |
| `PUT` | `/api/auth/passkeys/{id}` | Authenticated passkey label update for an owned passkey. |
| `DELETE` | `/api/auth/passkeys/{id}` | Authenticated passkey deletion for an owned passkey. |
| `GET` | `/api/audit` | Authenticated user-visible audit event list for the current actor. |
| `GET` | `/api/books` | Authenticated paginated list of books where the current user has explicit membership, including the current user's role. |
| `POST` | `/api/books` | Authenticated book creation with server-controlled id, owner, owner membership, and timestamps. |
| `GET` | `/api/books/{bookID}` | Authenticated role-aware book settings read for explicit book members. |
| `PATCH` | `/api/books/{bookID}` | Authenticated book settings update for owners and administrators, currently name and reporting currency. |
| `GET` | `/api/books/{bookID}/members` | Authenticated paginated explicit member list for book members. |
| `GET` | `/api/books/{bookID}/ledger/summary` | Authenticated book-scoped summary using explicit membership policy and optional UTC `start_date` / `end_date` day filters. |
| `GET` | `/api/accounts` | Authenticated paginated list of personal accounts owned by the current user. |
| `POST` | `/api/accounts` | Authenticated personal account creation with server-controlled id, owner, and timestamps. |
| `GET` | `/api/accounts/groups` | Authenticated paginated list of personal account groups owned by the current user. |
| `POST` | `/api/accounts/groups` | Authenticated personal account group creation with server-controlled id, owner, and timestamps. |
| `PATCH` | `/api/accounts/groups/{groupID}` | Authenticated personal account group update for the owning user. |
| `GET` | `/api/books/{bookID}/categories` | Authenticated paginated category tree list for explicit book members. |
| `POST` | `/api/books/{bookID}/categories` | Authenticated category creation for book owners and administrators. |
| `PATCH` | `/api/books/{bookID}/categories/{categoryID}` | Authenticated category update and soft archive for book owners and administrators. |
| `GET` | `/api/books/{bookID}/entries` | Authenticated paginated entry list for explicit book members. |
| `POST` | `/api/books/{bookID}/entries` | Authenticated entry creation for owners, administrators, and members, with server-controlled creator and book fields. |
| `PATCH` | `/api/books/{bookID}/entries/{entryID}` | Authenticated partial entry update with owner/administrator override and member creator-only policy. |
| `DELETE` | `/api/books/{bookID}/entries/{entryID}` | Authenticated entry deletion with owner/administrator override and member creator-only policy. |
| `POST` | `/api/imports/wacai/preview` | Authenticated Wacai CSV import preview from multipart field `file`, returning `201` with a staged preview batch. |

Future APIs should be grouped by domain and versioned once the first persistent contract is ready:

- `/api/auth`: session refresh, recovery-code administration, passkey admin recovery, and invite acceptance beyond the initial email/password, verification, password-reset, TOTP, and user-owned passkey contract.
- `/api/users`: profile, preferences, and identity-safe user lookup for invitations.
- `/api/books`: invitations, invitation acceptance, role changes, and member removal beyond the initial book workspace/settings/member-read contract.
- `/api/books/{bookID}/entries`: income, expense, transfer, refund, reimbursement, borrow, lend, and repayment entries.
- `/api/books/{bookID}/categories`: category mapping and display metadata beyond the initial create/update/archive contract.
- `/api/accounts`: account balances, credit-card settings, and visibility settings beyond the initial account and group create/list/update contract.
- `/api/imports`: mapping, commit, rollback, durable raw-file storage, `.xlsx` parsing, and import-batch status beyond the initial Wacai CSV preview contract.
- `/api/reconciliation`: statement-period matching and lock records.
- `/api/reports`: book-level and personal reports by category, account, member, merchant, tag, and time range.
- `/api/audit`: membership, import commit/rollback, reconciliation, and admin recovery audit events beyond the initial auth, ledger mutation, and import-preview feed.

Initial auth, book, account, category, entry, and import request contracts:

- `POST /api/auth/register` accepts `email`, `password`, and optional `turnstile_token`. The response returns public user data only. Password hashes and plaintext passwords are never returned.
- `POST /api/auth/login` accepts `email`, `password`, and optional `turnstile_token`. The response returns public user and session metadata and sets the configured HttpOnly session cookie.
- `GET /api/auth/email/verification` accepts `email` as a query parameter and returns a generic `202 Accepted` response with expiry metadata. The response never includes the verification code.
- `POST /api/auth/email/verification` accepts `email` and `code`, consumes the one-time code, and returns public user data after activation.
- `POST /api/auth/password-reset/request` accepts `email` and returns a generic `202 Accepted` response with expiry metadata. The response never includes the reset code or reveals whether the email exists.
- `POST /api/auth/password-reset/confirm` accepts `email`, `code`, and `newPassword`, consumes the one-time code, and returns public user data. Plaintext passwords are never returned.
- `GET /api/auth/totp/status` returns whether the authenticated user has TOTP enabled.
- `POST /api/auth/totp/setup` stores a pending TOTP secret against the current session and returns an otpauth URI and expiry. The response does not expose a separate secret field.
- `POST /api/auth/totp/confirm` accepts `code`, validates the pending session-scoped secret, stores the confirmed secret on the user, and clears the pending setup.
- `POST /api/auth/totp/disable` accepts `code`, validates the current TOTP secret, and clears the stored secret. Login for TOTP-enabled users requires `totp_code`, rejects replayed codes, and applies per-user failed-code limits.
- Passkey begin endpoints return a server-side `flowId` plus WebAuthn public-key options. Finish endpoints accept `flowId` and a nested browser credential response. Registration requires an authenticated session and stores credential id, public key, sign count, backup flags, transports, AAGUID, label, and timestamps without private key material.
- External SSO start accepts no body, stores only a hashed state value in an HttpOnly cookie, and redirects to the configured SSO login URL with `redirect_to`. The callback accepts `state` and `sso_token` query values, validates state with a constant-time comparison, validates the token server-side, clears the state cookie, creates the local session cookie, and never returns the token in a response body.
- Paginated list endpoints accept optional `page` and `page_size`, reject unknown query filters, and bound `page_size` to at most `100`. New list endpoints return an envelope with `items`, `page`, `pageSize`, and `total`; the existing entry list keeps its domain-specific `entries` field with the same pagination metadata.
- `GET /api/audit` accepts optional `page` and `page_size`, returns only audit events for the authenticated actor, and omits secret metadata such as passwords, tokens, secrets, and one-time codes. The initial feed records registration, login, logout, login failures, verification and password-reset requests and confirmations, TOTP setup/enable/disable, passkey registration/login/rename/delete, book create/update, account group create/update, account create, category create/update, entry create/update/delete, and Wacai import-preview creation.
- `POST /api/books` accepts `name` and `reportingCurrency`. The response returns the created book as a role-aware book list item with owner role for the creator.
- `PATCH /api/books/{bookID}` accepts optional `name` and `reportingCurrency`. At least one field is required. The server controls book id, owner, role, and timestamps.
- `GET /api/books/{bookID}/members` returns paginated explicit members for the book. Membership mutation, invitations, and invite acceptance remain future work until user lookup and invite contracts are defined.
- `POST /api/accounts` accepts `groupId`, `name`, `type`, `currency`, `sharedBookIds`, and `openingBalanceCents`. The supported initial account types are `cash`, `savings`, `credit_card`, `loan`, `investment`, and `payment_platform`; stored-value, receivable, and payable variants remain import/modeling targets until explicit product contracts are added.
- `POST /api/accounts/groups` accepts `name` and `sortOrder`; `PATCH /api/accounts/groups/{groupID}` accepts optional `name` and `sortOrder`. Account groups are personal to the authenticated user, and ownership is server-controlled.
- `POST /api/books/{bookID}/categories` accepts `parentId`, `name`, `direction`, `sortOrder`, and `rawSourceName`. Category direction must be `income` or `expense`.
- `PATCH /api/books/{bookID}/categories/{categoryID}` accepts optional `parentId`, `name`, `direction`, `sortOrder`, `archived`, and `rawSourceName`. Category removal is archival by setting `archived=true`; hard delete is deferred to avoid breaking historical entry references.
- `PATCH /api/books/{bookID}/entries/{entryID}` accepts optional `type`, `accountId`, `destinationAccountId`, `categoryId`, `amountCents`, `transactionCurrency`, `exchangeRate`, `occurredAt`, `note`, `merchant`, and `tags`. At least one field is required. The server controls id, book id, creator, account currency, book reporting currency, raw source, and timestamps.
- `POST /api/imports/wacai/preview` accepts authenticated `multipart/form-data` with a required `file` field. Initial support is CSV-only and returns `201 Created` with a preview batch containing source hash, parser version, detected schema, preview rows, row-level warnings/errors, and detected accounts, categories, currencies, and tags. Re-uploading the same file for the same user and source returns the same idempotent preview batch.
- Mutating book, account, category, and entry requests reject unknown JSON fields, return `201` for creates and `200` for updates, map validation failures to `400`, policy failures to `403`, and missing referenced resources to `404`.

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

The durable model should support personal cashflow bookkeeping first while leaving a path toward double-entry internals for reconciliation and reporting.

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
- Import batch: source upload metadata, parser version, source hash, detected schema, preview rows, row diagnostics, and idempotent preview status, with mapping decisions, commit status, raw-file object storage, and rollback state added when committed imports become durable.
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

Persistent storage is hidden behind repository interfaces owned by the relevant domain package. The default `memory` driver is for local development. The `file` driver stores atomic JSON snapshots for auth, ledger, audit, and import-preview state under `ACCOUNTING_PERSISTENCE_DIR`; files are written with owner-only permissions because they contain password hashes, TOTP secrets, session hashes, and other server-side state. This single-node file driver is a durable bootstrap path.

The `sqlite` and `postgres`/`postgresql` drivers are direct SQL drivers. They do not load domain snapshots into process memory and later flush them; each create, update, delete, counter increment, and idempotent import-batch claim writes SQL rows synchronously before returning success. Reads query SQL rows through the domain store. The first SQL schema uses a shared `accounting_records` table keyed by namespace, primary key, parent key, owner key, and optional unique secondary key, with each domain object encoded as JSON. This keeps the current API behavior and store contracts stable while moving durability and concurrency control to the database. A later relational migration can split high-value accounting tables once reporting and double-entry invariants require database-native joins and constraints.

PostgreSQL uses `database/sql` through pgx stdlib, stores JSON as `jsonb`, and sets the session time zone to UTC on startup. SQLite uses the `mattn/go-sqlite3` `database/sql` driver with WAL, foreign keys, a busy timeout, `synchronous=NORMAL`, and immediate write transactions; the SQLite pool is serialized with one open connection until a broader concurrent-write design is tested. These choices follow the current Go `database/sql` contract, pgx stdlib driver documentation, PostgreSQL timestamp/transaction guidance, and SQLite WAL/foreign-key/transaction guidance.

Durable source references for future persistence work:

- Go `database/sql`: https://pkg.go.dev/database/sql
- pgx stdlib: https://pkg.go.dev/github.com/jackc/pgx/v5/stdlib
- PostgreSQL date/time types and transaction isolation: https://www.postgresql.org/docs/current/datatype-datetime.html and https://www.postgresql.org/docs/current/transaction-iso.html
- SQLite WAL, foreign keys, pragmas, and transactions: https://sqlite.org/wal.html, https://sqlite.org/foreignkeys.html, https://sqlite.org/pragma.html, and https://sqlite.org/lang_transaction.html
- mattn SQLite driver: https://pkg.go.dev/github.com/mattn/go-sqlite3

Controllers must not build SQL directly. SQL repositories must validate untrusted inputs, bind query parameters, and avoid building queries from raw user-provided strings.

## Configuration

Current backend environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_ADDR` | `:8080` | Go HTTP listen address. |
| `ACCOUNTING_DEBUG` | `false` | Enables debug logging and Gin debug mode. |
| `ACCOUNTING_SERVER_NAME` | `accounting` | Public server label returned in runtime config. |
| `ACCOUNTING_WEB_DIST_DIR` | `../web/dist` | Built SPA directory served by the Go process. |
| `ACCOUNTING_WEB_DEV_URL` | empty | Optional reverse proxy target for non-API frontend routes. |
| `ACCOUNTING_PERSISTENCE_DRIVER` | `memory` | Storage driver: `memory`, `file`, `sqlite`, `postgres`, or `postgresql`. |
| `ACCOUNTING_PERSISTENCE_DIR` | `./var/accounting` | Directory for file-backed snapshots and the default SQLite file. |
| `ACCOUNTING_DATABASE_URL` | empty | SQL database URL. PostgreSQL requires a pgx-compatible URL. SQLite may use an empty value for `ACCOUNTING_PERSISTENCE_DIR/accounting.sqlite3`, a plain path, or `sqlite://` URL. Falls back to `DATABASE_URL`. |
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
| `ACCOUNTING_AUTH_RATE_LIMIT_ENABLED` | `true` | Enables fixed-window limits on public authentication routes. |
| `ACCOUNTING_AUTH_RATE_LIMIT_LIMIT` | `20` | Maximum attempts per auth route, client IP, and email/flow subject within the window. |
| `ACCOUNTING_AUTH_RATE_LIMIT_WINDOW` | `1m` | Public auth route rate-limit window. |
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
| `ACCOUNTING_AUTH_SESSION_COOKIE_NAME` | `accounting_session` | Browser session cookie name. |
| `ACCOUNTING_AUTH_SESSION_COOKIE_SECURE` | `true` | Adds the Secure attribute to session cookies; keep enabled for production. |
| `ACCOUNTING_AUTH_SESSION_TTL` | `24h` | Browser session lifetime. |
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
make e2e
```

Required final gates after code changes:

```sh
make lint
cd backend && go test -race -cover ./...
cd ../cli && go test -race -cover ./...
pnpm --dir web run test:e2e
```

Equivalent separate invocations are also acceptable:

```sh
cd backend && go test -race -cover ./...
cd cli && go test -race -cover ./...
pnpm --dir web run test:e2e
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
- Extend the initial Wacai CSV preview parser with sample-file collection, mapping UX, commit behavior, rollback behavior, `.xlsx` support, and broader format coverage.
- Add import/export formats for bank statements after the Wacai migration path is defined.
- Add a deployment document once Docker and CI exist.
