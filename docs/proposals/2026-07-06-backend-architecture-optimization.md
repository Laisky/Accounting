# Backend Architecture Optimization — Delivery Handbook

- Status: **Delivery-ready execution version** (expanded from the earlier skeleton draft; every diagnostic anchor re-verified against the current tree, with per-item closure status and additional findings folded in).
- Date: 2026-07-06
- Scope: the `backend/` Go/Gin API server — persistence, domain layering, API contract, security baseline, observability, toolchain, and CI. `cli/` is affected indirectly by the API/persistence changes and owns the data-migration subcommand. `web/` only needs to coordinate on the `/api/v1` prefix switch and `problem+json` parsing (in concert with the frontend handbook `docs/proposals/2026-07-06-architecture-overhaul.md`).
- Non-goals: no frontend architecture change; no Go/Gin replacement; no microservice split; no third-party SaaS; no rework of the already-correct money/FX core or the authorization strategy.
- Product stage: not yet launched, no production data, breaking refactors are allowed. This is the lowest-cost window to land a relational schema and a double-entry core.
- Related commit: the previous commit `8dfbf10 refactor: overhaul frontend/backend architecture with OpenAPI contract` already landed the OpenAPI 3.1.2 contract, the `{code,message,requestId}` error envelope, the four CI workflows, and the `kin-openapi` contract test. **This handbook builds on that work; §1.5 lists what it already closed so the team does not re-implement it.**

## 0. Executive Summary

The correctness assets of the backend core (int64 minor-unit amounts, `big.Rat` half-even rounding, parameterized SQL, opaque hashed sessions, constant-time comparisons, per-handler ownership checks, and the four `http.Server` timeouts) already meet the 2026 baseline and **must be preserved as-is** (§1.1). However, the persistence layer is a single generic JSON table with no versioned migrations, in-memory pagination, and three duplicated store implementations per domain; import apply is non-transactional; there are no metrics; configuration silently falls back on bad values; and ten Medium-severity security gaps remain open. These directly conflict with the product goals of correctness, integrity, and auditability.

Remediation proceeds in five phases, each independently reviewable and reversible:

- **P0 — Security and engineering floor**: quick wins that do not depend on the refactor — session revocation, account lockout, TOTP encryption at rest, argon2id, member-management surface, HSTS / body-size limits, fail-fast config validation, exchange-rate goroutine cleanup, and golangci-lint / gosec / govulncheck in CI.
- **P1 — Persistence refactor (the core)**: goose embedded migrations + single-point driver selection + a UnitOfWork, replacing the single JSON table with a relational schema carrying FK / CHECK / unique constraints and keyset indexes, pushing pagination down to SQL, and removing the file driver. **P3 and P4 build their atomicity and double-entry core on P1.**
- **P2 — API contract and error model**: a single clean cutover to the `/api/v1` prefix (pre-launch, so no compatibility alias), 100% OpenAPI coverage, unified RFC 9457 `application/problem+json` errors (a governed `code` enum plus `request_id`), and Error-level logging for 5xx.
- **P3 — Import domain refactor**: sink the apply orchestration from the HTTP layer into `internal/imports`, wrap the whole flow in a single transaction with a database-level idempotency claim, and eliminate duplicate postings and concurrent double-writes.
- **P4 — Observability and double-entry core**: OTel Metrics (RED / DB pool / import / auth / ledger) plus `/healthz` and `/readyz`, and a double-entry core landed on P1's `journal_entries` / `postings` tables with balance reconciliation.

Every phase carries quantified, individually machine-checkable exit criteria (§6.2); the acceptance gate matrix is in §7.

## 1. Background

### 1.1 Current-state assessment — designs that already meet the bar and must be preserved

The following capabilities already match 2026 best practice and must be preserved unchanged; do not rebuild them (anchors verified against the current tree on 2026-07-06):

| Item | Evidence |
| --- | --- |
| Amounts stored as `int64` minor units; FX rates as decimal strings + `math/big.Rat` half-even rounding (`roundRatToInt64`); no float anywhere | `internal/ledger/model.go`, `internal/ledger/money.go:32-122` |
| Fully parameterized SQL (`?`→`$n` rebind); no string-concatenation injection surface | `internal/persistence/sql_store.go` |
| Sessions are opaque random tokens; only the SHA-256 hash is stored; HttpOnly / SameSite=Lax / Secure on by default | `internal/auth/session.go` |
| Constant-time comparison for password / TOTP / SSO state; email-enumeration protection; one-time-code attempt cap (`maxEmailCodeAttempts=5`); TOTP replay protection and failure lockout (`totpMaxFailures=5`) | `internal/auth/password.go`, `email_codes.go:190-198`, `totp.go:171-215` |
| Strict JSON decoding (`DisallowUnknownFields`), query-parameter allowlist, page-size cap of 100, `SetTrustedProxies` wired | `internal/httpserver/routes.go:439-452`, `server.go:41` |
| Baseline security response headers present (`X-Content-Type-Options` / `X-Frame-Options` / `Referrer-Policy` / `CSP`) | `internal/httpserver/server.go:285-291` |
| Secrets come only from environment variables; no on-disk default secrets; audit metadata sanitized (keys containing `password/token/secret/code` dropped) | `internal/config/config.go`, `internal/audit/service.go:143-150` |
| Complete authorization model: every authenticated CRUD path re-checks book-member role and resource ownership in the service layer; no privilege-escalation gap found | `internal/ledger/books.go:145-182`, `service.go` |
| Acyclic dependencies with a real service layer; all four `http.Server` timeouts set; `main.go` already establishes a `signal.NotifyContext` cancellable context and a graceful `Shutdown` | `internal/httpserver/server.go`, `cmd/accounting-server/main.go:23,54-71` |

### 1.2 Core architecture problems (B1–B12, all verified)

Each diagnostic anchor was verified against the current tree (`verdict` column). `DRIFTED` means the line numbers drifted and are corrected here; `PARTIAL` means the prior commit already closed part of it, with work remaining.

| ID | Verdict | Problem | Current evidence | Risk |
| --- | --- | --- | --- | --- |
| B1 | CONFIRMED | SQL persistence is one generic JSON table `accounting_records` (`namespace` + `jsonb`/`TEXT` blob); 19 domain namespaces share it, only `parent_key/owner_key/secondary_key` are queryable; no foreign keys, no CHECK, no SQL-side aggregation or filtering | `persistence/sql_store.go:342-389` (`ensureRecordSchema`), `Record` struct `:30-45` | Bookkeeping data has no database-level integrity guarantee; reporting/reconciliation cannot be pushed to the database |
| B2 | CONFIRMED | No versioned migrations: on startup `pingAndMigrate`→`ensureRecordSchema`→`execAll` runs bare `CREATE TABLE IF NOT EXISTS`; no migration table, no ordered files, no rollback | `persistence/sql_store.go:325-398` | Schema evolution is uncontrolled and irreversible |
| B3 | CONFIRMED | Three store implementations per domain (Memory / Snapshot(file) / SQL) each re-implement CRUD/sort/uniqueness; the driver-selection switch is copied in **exactly 5 places**: `openPersistenceDB`, `newLedgerStore`, `newAuditStore`, `newAuthStore` (`server.go:114-219`), `newDefaultImportService` (`import_service.go:18-44`) | see evidence column | Adding one field means editing 3 implementations; httpserver has become the persistence-assembly hub |
| B6 | CONFIRMED | Lists load the full set then paginate in memory; the SQL layer has no `LIMIT/OFFSET/ORDER BY`; pagination/normalization is duplicated **four times** (`ledger.paginate` and `auth.paginate` are byte-identical, plus inline copies in `ledger.ListEntries` and `audit.Service.List`) | `ledger/pagination.go:17-37`, `entries.go:38-60`, `persistence/sql_store.go:208-247` | Every list request becomes a full scan as data grows |
| B7 | **PARTIAL** | No API version prefix (`/api` only, `servers.url:/api`); the error `code` is **derived by slugifying the English message** in `apiErrorCode()` (change the wording and the code changes — not a governed enum); `respondLedgerError`'s default branch logs real 5xx at Debug level | `routes.go:557-572`, `api_error.go:34-55` | No buffer for future breaking changes; unstable error codes; a Debug black hole for production failures |
| B8 | CONFIRMED | No metrics: `telemetry.Init` only `SetTracerProvider`, no `SetMeterProvider`, no `/metrics`; `otel/metric` is an indirect dependency only; OTel tracing is off by default | `telemetry/telemetry.go:45-53`, `go.mod` (`otel/metric` indirect) | No RED / latency / throughput signal; no SLO alerting possible |
| B9 | CONFIRMED | Config parsing silently falls back: `readBool/readInt/readDuration` take the default on a malformed non-empty value; there is no startup `Validate()`, and cross-field rules are scattered in lazy checks such as `telemetry.Init` (OTLP endpoint) | `config/config.go`, `telemetry/telemetry.go:31-33` | Production misconfiguration goes unnoticed (e.g., a mistyped TTL silently becomes the default) |
| B10 | CONFIRMED | The exchange-rate updater is started with `context.Background()` and is not reclaimed on shutdown (**the goroutine loop already does `select ctx.Done()`; only the context fed to it is wrong**) | `ledger/rates.go:107-121`, `server.go:69` | Goroutine leak; unclean exit |
| B11 | CONFIRMED | Not double-entry: a transfer is a single entry with an optional destination account (**the movement across two accounts stores only one `AmountCents/TransactionCurrency`**); balances are a service-layer type-switch sum with no structural "debits equal credits" invariant | `ledger/model.go:251-271`, `service.go:99-123` | Account balances cannot be reconciled independently; cross-currency transfers are inherently unreconcilable |
| B12 | CONFIRMED | Weak toolchain: `make backend-lint` is only `gofmt` + `go mod tidy` + `go vet`, with no golangci-lint/gosec/govulncheck; `go.yml` has no `go build`, no govulncheck, and no Postgres service, so the Postgres integration test is permanently `t.Skip`ped for lack of `DATABASE_URL` | `Makefile:35-38`, `.github/workflows/go.yml`, `persistence/sql_store_test.go:41-45` | Zero coverage of static defect classes and Postgres-specific behavior (JSONB conversion, `$n` rebind, partial unique indexes) |

### 1.3 Security audit findings (S1–S10, Medium severity, all to be closed in P0)

| ID | Verdict | Problem | Current evidence |
| --- | --- | --- | --- |
| S1 | CONFIRMED | Old sessions are not revoked after password reset / TOTP disable; the store has no bulk delete-by-user | `auth/email_codes.go:128-136`, `totp.go:161-166`, `store.go` session interface |
| S2 | CONFIRMED | No account lockout: the failure count is per-email and only triggers Turnstile; brute-force defense is limited to an in-process IP+email fixed-window limiter (not shared across replicas, a new IP starts a new bucket, single-account guessing across IPs is unbounded) | `auth/service.go`, `auth_rate_limiter.go:16-42` |
| S3 | CONFIRMED | No HSTS response header (the other baseline headers are present) | `httpserver/server.go:285-291` |
| S4 | CONFIRMED | No request-body size limit on JSON routes (`http.MaxBytesReader` is used **only** on the 6 MiB import upload), leaving a memory-exhaustion DoS surface | `routes.go:439-452`, `import_routes.go:24,39` |
| S5 | CONFIRMED | The TOTP secret is serialized in plaintext (base32) with the user record to disk/DB; a store leak instantly defeats 2FA | `auth/model.go`, `totp.go` storage path |
| S6 | CONFIRMED | Audit is append-only by convention only: the underlying store can Update/Delete, there is no hash chain, **failed-login events have an empty ActorID** (even when the email resolves to a real user) so they are unfindable, and there is no admin global audit view | `audit/sql_store.go`, `auth_routes.go:71-75`, `audit_routes.go:17-44` |
| S7 | CONFIRMED | No member-removal / role-change / unshare API — sharing is irrevocable once it happens; `AddBookMember` exists but is reachable only via the import path | `ledger/books.go:145-182`, `import_members.go` |
| S8 | CONFIRMED | The SSO one-time token travels in the URL query (which enters access logs / Referer), auto-provisioning is on by default, and matching an existing account by email does not bind the SSO subject (subject/email confusion) | `auth_sso_routes.go:72`, `config.go:176`, `service.go:255-282` |
| S9 | CONFIRMED | Password hashing is PBKDF2-SHA256 (600k iterations); OWASP 2025/2026 prefers argon2id | `auth/password.go` |
| S10 | CONFIRMED | `FORCE_SMTP_TLS_VERIFY` is dead code (`tlsConfig` is never referenced); the actual send path is `smtp.SendMail` with opportunistic STARTTLS | `auth/email_sender.go:75-89` |

### 1.4 Additional findings surfaced by this verification (diagnostic gaps, folded into the relevant phase)

The code review found the following real defects beyond the original diagnosis; each is folded into a phase deliverable:

| ID | Severity | Finding | Evidence | Owner |
| --- | --- | --- | --- | --- |
| N1 | **High** | Transfers are excluded from all monetary totals: `BookSummary` only does `TransferCount++` for transfers; a two-account movement stores a single `AmountCents/TransactionCurrency`, so cross-currency transfers can never reconcile | `ledger/service.go:120-122`, `model.go:257-263` | P4 (root-caused by double-entry) |
| N2 | Medium | A book's owner is permanently fixed to its creator: `isSupportedMemberRole` excludes `owner`, there is no `UpdateBookMember`, and no ownership transfer | `ledger/books.go:64-71,221-228` | P0-6 (member role change) |
| N3 | Medium | `Account` has no Update/Delete, so `SharedBookIDs/currency/group` are immutable after creation (the S7 unshare gap is one symptom) | `ledger/store.go:32-33` | P0-6 (`UpdateAccount`) |
| N4 | Medium | Import member resolution has write side-effects: `AddBookMember` runs before entry creation and outside any transaction, so a later row failure leaves members added | `import_members.go:76`, `import_routes.go:216` | P3 (single transaction) |
| N5 | Medium | `ensureWacaiImportReferences` eagerly creates accounts/categories before committing rows; a later hard row failure leaves those references persisted with no rollback | `import_routes.go:342-383,413-419` | P3 (single transaction) |
| N6 | Medium | The unauthenticated demo endpoint `GET /api/ledger/summary` still exists and leaks in-memory demo ledger totals without a session | `routes.go:120-125` | P2-2 (delete) |
| N7 | Low | `requestId` is usually empty: `api_error.go` reads it from the `X-Request-ID` response header, but no middleware sets that header | `api_error.go:21`, `server.go:44-57` | P2-3 (`RequestID()` middleware) |
| N8 | Low | CI never runs `go build ./...` and has no govulncheck: a compile break in a file with no tests, or a known-vulnerable dependency, would pass | `go.yml`; `Makefile:9-10` is not invoked by any workflow | P0-14 (add build + vuln) |
| N9 | Low | Import replay reads entries after deletion, yielding `ImportedCount > len(Entries)` — an internally inconsistent replay response | `import_routes.go:248-266` | P3 (keep behavior + assert, backlog) |

### 1.5 Already closed by the prior commit — **do not redo**

`8dfbf10` already landed the following; the corresponding items in this handbook are *incremental*, not from-scratch:

- **The OpenAPI contract already exists**: `docs/api/openapi.yaml` (3.1.2, `info.version 0.1.0`, `servers.url:/api`) + `docs/api/schemas.yaml` + `docs/api/paths/{ledger,system}.yaml`; the `ErrorResponse` schema (`required [code,message]`) matches the runtime shape. → **P2 only needs** to change the code from "slug of the message" to a governed enum, evolve the envelope to problem+json, add `X-Request-ID`, and add `/api/v1`.
- **The structured error envelope already exists**: `api_error.go`'s `apiErrorResponse{code,message,requestId}` + `respondAPIError/respondAPIMessage/abortAPIMessage`, used by all 96 error sites, with no raw `gin.H{"error"}`. → see above.
- **The backend contract test already exists**: `contract_routes_test.go` (`TestRegisterRoutesOpenAPIContract`, using `getkin/kin-openapi`). → **P2 only needs** to switch to the fully 3.1-capable `pb33f/libopenapi-validator` and upgrade `contract.yml` to a three-stage check.
- **CI already has four workflows**: `go.yml`/`web.yml`/`e2e.yml`/`contract.yml`; `make backend-lint`/`backend-test` are already invoked by `go.yml`; the `check:api` frontend-type freshness check is already in CI. → **B12 only needs** golangci-lint/gosec/govulncheck, a Postgres service, and `go build`.
- **The import-preview idempotency race is already closed**: `SaveBatchIfAbsent` (`store.go:74-87`, `sql_store.go:46-65`) removed the preview-side check-then-save race. → **B4 targets the apply side only** (that fix does not address apply double-writes).
- **SSO CSRF state is already protected**: the callback already validates a hashed HttpOnly state cookie (`auth_sso_routes.go:211-256`) and refuses to derive the callback URL from a client-controlled Host for non-loopback. → **S8 only needs** to change the token transport, flip the auto-provision default, and bind the subject.
- **The exchange-rate goroutine loop is already shutdown-aware**, and `main.go` already has a cancellable signal context. → **B10 only needs** to feed the correct context in and add `RegisterOnShutdown`.
- **The SQLite SQL path is already covered by CI** (`TestRecordStoreSQLiteRoundTrip`). → **B12 only needs** the Postgres path.

## 2. Overall Design Decisions (2026 baseline)

### 2.1 2026 practice baseline (verified 2026-07-06 against primary/official sources)

| Area | Conclusion | Source |
| --- | --- | --- |
| SQL migration tool | `pressly/goose` v3 (latest v3.27.x) supports compile-time embedding via `embed.FS` and covers both SQLite and Postgres; `golang-migrate` (`iofs` source driver) is the dual-database alternative | https://github.com/pressly/goose |
| Password hashing | The OWASP Password Storage Cheat Sheet prefers **argon2id** with a minimum tier of `m=19456 (19 MiB), t=2, p=1`; PBKDF2 is a FIPS-compliance fallback only (≥600k iterations, HMAC-SHA-256) | https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html |
| Error model | **RFC 9457** Problem Details (2023, obsoletes RFC 7807); media type `application/problem+json`; standard members `type/title/status/detail/instance` plus extension members | https://www.rfc-editor.org/rfc/rfc9457.html |
| OTel Go Metrics | The Metrics API/SDK has been **stable/GA since opentelemetry-go v1.19.0** (Nov 2023) and carries the project's backward-compatibility guarantees | https://opentelemetry.io/blog/2023/otel-go-metrics-sdk-stable/ |
| Contract validation library | Use **`pb33f/libopenapi` + `libopenapi-validator`** (`ValidateHttpResponse` accepts an `*http.Response`, full OpenAPI 3.1 support) instead of `getkin/kin-openapi` (incomplete 3.1 support, issue #230) | https://github.com/pb33f/libopenapi-validator |
| OpenAPI version | **3.1** is the broadly supported, safe baseline; 3.2.0 (Sep 2025) is backward-compatible but tooling for its new features is uneven, so it is not adopted yet | https://www.openapis.org/blog/2025/09/23/announcing-openapi-v3-2 |

### 2.2 Design decision table

| Area | Decision | Rationale / rejected alternative |
| --- | --- | --- |
| Database | A relational schema replaces the single JSON table: one table per core entity, with foreign keys + CHECK + unique constraints in the database | Bookkeeping integrity must be backed by the database; the single JSON table blocks all four of B1/B4/B6/B11. Rejected: "keep the JSON table + application-layer validation" |
| Driver convergence | Production PostgreSQL, single-node self-hosted SQLite, tests memory; **retire the file driver** | The file driver rewrites the whole snapshot (`ledger/file_store.go:277-283`), and SQLite already covers its single-node case. Three implementations collapse to "one SQL + one memory for tests" |
| Migration tool | Versioned SQL migrations, embedded in the binary (`embed.FS`), optional auto-migrate on startup + a CLI manual migrate. Adopt `pressly/goose` v3 | SQL-first, reviewable, reversible. Rejected: Atlas declarative and ORM auto-DDL |
| Transaction model | Introduce `internal/storage.DB.WithTx(ctx, fn)` as a cross-repository UnitOfWork; multi-write flows (import apply, double-entry writes) must be single transactions | Today's per-domain `RecordStore` makes cross-domain transactions impossible |
| Ledger core | Move to double-entry in two steps: P1 pre-provisions the `journal_entries` + `postings` tables; P4 makes transfers write paired postings in the same transaction; the user-visible Entry model is unchanged | Lowest cost to land the structure before launch; balances become independently reconcilable from postings. Rejected: change it after launch (requires data migration) |
| API | A single clean cutover to the `/api/v1` prefix with **no** `/api` compatibility alias; 100% OpenAPI 3.1 coverage; errors unified as RFC 9457 `application/problem+json` (a governed `code` enum + `request_id`) | Pre-launch with zero external clients: backward compatibility is explicitly a non-goal, and without an alias a missed caller fails loudly in CI instead of surviving silently. Rejected: a transitional alias with Deprecation/Sunset headers — post-launch machinery that only adds surface here. A generic or message-derived code cannot support stable client-side branching |
| Password hashing | argon2id (`x/crypto/argon2`, OWASP `m=19456,t=2,p=1`) with transparent rehash of old PBKDF2 hashes on successful login | Migrates without forcing a password reset |
| Sensitive data at rest | The TOTP secret is envelope-encrypted with AES-256-GCM (DEK-per-secret + KEK wrap + key-id rotation), keyed by `ACCOUNTING_SECRET_KEY` | A store/backup leak no longer directly defeats 2FA |
| Observability | OTel Metrics (stable) emit RED + Go/DB metrics via a Prometheus reader on `/metrics`; add `/healthz` (liveness) and `/readyz` (with DB ping) | Same pipeline as the frontend handbook's telemetry endpoints. Rejected: third-party APM |
| Toolchain | golangci-lint v2 (errcheck/staticcheck/gosec and ~20 linters) + `govulncheck` in `make backend-lint`/`backend-vuln` and CI; CI adds a Postgres service container for real integration tests | `vet`-only coverage is insufficient; the production driver must be continuously verified |
| Configuration | A startup `Config.Validate()` that fails fast on invalid values; no silent fallback | A bookkeeping service should refuse to start rather than run misconfigured |

## 3. Target Architecture

```text
cmd/accounting-server        # startup, signals, graceful shutdown (cancellable context for all background goroutines)
internal/
├── httpserver               # routing, auth middleware, DTO codec, problem+json mapping only — no store assembly, no domain orchestration
├── ledger | auth | audit    # domain services (invariants, policy), depend only on their own Repository interfaces
├── imports                  # parse + preview + [NEW] apply orchestration (sunk from import_routes.go), depends on ledger/auth service interfaces
├── storage                  # [NEW] unified assembly: single-point Open, goose migrations, WithTx (UnitOfWork), per-domain SQLRepository
├── crypto/keyring           # [NEW] AES-256-GCM envelope encryption (TOTP at rest, rotatable)
├── paging                   # [NEW] page→limit/offset normalization (consolidates the 4 duplicated paginators)
└── config / logger / telemetry / diagnostics
```

Relational schema highlights (full DDL in Appendix A):

- 14 core tables + the `account_shared_books` support table: `users`, `sessions`, `books`, `book_members`, `categories`, `account_groups`, `accounts`, `entries`, `journal_entries`, `postings`, `exchange_rates`, `import_batches`, `import_rows`, `audit_events`.
- Example enforced constraints: `entries.amount_cents BIGINT CHECK (> 0)`; `entries` transfer `CHECK(type<>'transfer' OR destination_account_id IS NOT NULL)`; `book_members` primary key `(book_id,user_id)`; `import_batches UNIQUE(user_id,source_hash)`; `users UNIQUE(lower(email))`; foreign keys on every cross-table reference; time columns are `timestamptz` (Postgres) / ISO8601 UTC `TEXT` (SQLite).
- List queries are pushed to the database: `ORDER BY … LIMIT ? OFFSET ?`; entry streams use the keyset composite index `entries(book_id, occurred_at DESC, id DESC)`.
- The 7 auth ephemeral namespaces (`email_codes/pending_totp/totp_replays/failed_totps/failed_logins/passkeys/passkey_ceremonies`) migrate into a constrained `auth_kv` residual table (zero-rewrite functional preservation); dedicated passkey tables are deferred to a later migration.
- The `postings` two-currency amounts (`amount_cents` in account currency + `reporting_cents` in reporting currency) make journal balance verifiable in pure SQL; per-journal debit=credit is enforced in the P4 write path by an application-layer assertion inside `WithTx` plus a Postgres `DEFERRABLE CONSTRAINT TRIGGER`.

## 4. Change Inventory

Delivered by phase; each phase is independently reviewable and reversible. **P0 goes first** (no refactor dependency); **P1 is the foundation for P3 and P4.** Each row carries "key steps" and "row-level acceptance" so it can be assigned directly.

---

### P0 — Security and Engineering Floor (quick wins, no refactor dependency)

> Goal: close the ten security gaps S1–S10 and the engineering gaps B9/B10/B12 without changing the external OpenAPI contract semantics (new endpoints require matching contract entries).

#### P0-A Security baseline (S1–S10)

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P0-1 | **[S1] Session revocation**: add `DeleteSessionsByUser(ctx,userID)` to `auth.Store`; add `DeleteByOwner(ns,ownerKey)` to `RecordStore` (`DELETE … WHERE namespace=? AND owner_key=?`); revoke all of a user's sessions after `ConfirmPasswordReset`/`DisableTOTP`; add `POST /api/auth/logout-all` (`RequireSession`) → `LogoutAll` + clear cookie + audit `auth.logout_all` | `auth/{store,sql_store,file_store,service,email_codes,totp}.go`, `persistence/sql_store.go`, `httpserver/auth_routes.go` | After password reset / TOTP disable / logout-all, the old token returns 401 on a protected route; `grep -rn DeleteSessionsByUser auth` hits 3 implementations |
| P0-2 | **[S2] Account lockout + limiter-key fix**: add `LoginThrottle{Email,FailedCount,LockedUntil,UpdatedAt}` to `model.go`; in `Login`, no lock for the first 5 failures, then exponential backoff `min(1h, 1min·2^(n-6))`, rejecting even a correct password while locked (429 + `Retry-After`, audit `auth.login_locked`); split `authRateLimiter.allow` into two dimensions `(route,subject)` and `(route,ip)` | `auth/{model,store,sql_store,file_store,service}.go`, `httpserver/{auth_rate_limiter,auth_routes}.go` | After 6 consecutive wrong passwords for one email, the 7th attempt is 429 even with the correct password (backoff 6→1m/7→2m/8→4m, capped at 1h); one subject exceeding the limit across many IPs is rejected |
| P0-3 | **[S3/S4] HSTS + body-size limit**: `securityHeaders` sends `Strict-Transport-Security: max-age=63072000; includeSubDomains` only over HTTPS (`c.Request.TLS!=nil` or a trusted `X-Forwarded-Proto=https`); `decodeStrictJSON` wraps the body in `http.MaxBytesReader(w,body,1<<20)` and maps `errors.As(*http.MaxBytesError)` to 413 | `httpserver/{server,routes,server_test}.go` | HTTPS responses carry HSTS, HTTP responses do not; any JSON route returns 413 for a >1 MiB body |
| P0-4 | **[S5] TOTP envelope encryption at rest**: add a `crypto/keyring` package (AES-256-GCM, DEK-per-secret + KEK wrap + `key-id` rotation, AAD bound to `"auth.totp:"+userID`); store `TOTPSecret`/`PendingTOTPSetup.Secret` as `enc:v1:<kid>:<b64>`; on startup, idempotently migrate historical plaintext in place; keys read from `ACCOUNTING_SECRET_KEY` (active) + `ACCOUNTING_SECRET_KEY_RETIRED` (decrypt-only) | `crypto/keyring/*.go`, `config/config.go`, `auth/{service,totp,store,sql_store}.go`, `httpserver/server.go` | After ConfirmTOTP the stored `TOTPSecret` has the `enc:v1:` prefix and does not match the plaintext-base32 regex, and TOTP login still succeeds; the migration is a no-op on second startup; keyring tests cover round-trip / retired-key decrypt / AAD tamper |
| P0-5 | **[S6] Queryable subject + hash chain + admin read**: add `subjectHash=sha256(lower(email))` to failed-auth event metadata; add `Seq/PrevHash/Hash` to `Event` and compute the chain in `Record` (via `Tail()`); add `GET /api/admin/audit` (`requireAdmin` allowlist) → `ListAll`. (The strict-monotonic `seq` UNIQUE constraint and a `/audit/verify` endpoint land with the P1 standalone table) | `audit/{model,service,store,sql_store,file_store}.go`, `httpserver/{auth_routes,auth_sso_routes,audit_routes}.go`, `config/config.go` | Failed-login events carry a stable `subjectHash`; a non-allowlisted user gets 403 on `/api/admin/audit`; adjacent events satisfy increasing `Seq` and a linking `PrevHash`, and a broken chain is detectable by the verify function |
| P0-6 | **[S7/N2/N3] Member-management surface + mutable accounts**: add `DeleteBookMember`/`UpdateBookMember`/`UpdateAccount` to `ledger.Store`; add `RemoveBookMember`/`UpdateBookMemberRole`/`UnshareAccount` to `Service` (Owner/Admin authorization, refuse to remove/demote the sole owner); routes `POST/PATCH/DELETE /api/books/:bookID/members[/:userID]` + `DELETE /api/accounts/:accountID/shares/:bookID`, all audited | `ledger/{store,sql_store,file_store,books,accounts_categories}.go`, `httpserver/{book_routes,account_category_routes}.go` | An Owner can add / change role / remove a member and unshare, producing audit events; Member/Viewer get 403; removing/demoting the sole owner is refused |
| P0-7 | **[S8] SSO token transport + auto-provision default**: change the callback to `POST /auth/sso/callback` (`c.PostForm("sso_token")`, state still via the hashed cookie); keep a GET compatibility path with `sso_token` log-redaction when the provider cannot do form_post; explicitly bind the SSO subject when matching an existing account by email; change `AUTO_PROVISION` default to `false` | `httpserver/auth_sso_routes.go`, `config/config.go`, `auth/service.go` | The callback completes login via POST and the token is absent from URLs/logs; a fresh deployment does not auto-provision by default |
| P0-8 | **[S9/S10] argon2id + enforced SMTP TLS**: `HashPassword` uses `argon2.IDKey` (`m=19456,t=2,p=1`, encoded `$argon2id$v=19$…`), `VerifyPassword` dispatches by scheme and remains compatible with legacy pbkdf2, and `NeedsRehash` triggers a transparent rehash after a successful login; send via `Dial→Extension("STARTTLS")→StartTLS(tlsConfig)→Auth→Data`, making `ForceTLSVerify` effective and removing `smtp.SendMail` | `auth/{password,password_test,service,email_sender}.go`, `go.mod` | New hashes match `^\$argon2id\$v=19\$m=19456,t=2,p=1\$`; legacy pbkdf2 still verifies and is transparently upgraded after one login; `grep -n smtp.SendMail auth`=0; with an invalid cert, `ForceTLSVerify=true` fails the send |

**P0-A key implementation (argon2id / envelope encryption / login backoff)** — full crypto artifacts in Appendix C; core parameters:

```go
// argon2id: m=19456(19MiB), t=2, p=1, keyLen=32, saltLen=16; encoded $argon2id$v=19$m=19456,t=2,p=1$<b64salt>$<b64hash>
// TOTP envelope: enc:v1:<kek_kid>:<base64url( dekNonce(12)||wrappedDEK(48)||payloadNonce(12)||ciphertext )>; aad="auth.totp:"+userID
// Login backoff: loginLockDuration(n) = n<=5 ? 0 : min(1h, 1min<<(n-6))
```

#### P0-B Engineering floor (B9/B10/B12)

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P0-9 | **[B9] Strict config parsing**: turn `readBool/readInt/readDuration` into `loader` methods that accumulate an error on a malformed non-empty value; change `LoadFromEnv()` to return `(Config,error)` (`errors.Join`); `main.go` `Fatal`s via a bootstrap logger on error | `config/config.go`, `config/config_test.go`, `cmd/*/main.go` | Injecting `ACCOUNTING_AUTH_RATE_LIMIT_LIMIT=abc` or similar makes the process exit non-zero with the variable name in the message; `grep -c 'func readBool\|func readInt\|func readDuration' config.go`=0 |
| P0-10 | **[B9] `Config.Validate()` fail-fast**: 11 cross-field/range rules (driver enum, pg requires `DATABASE_URL`, OTEL requires endpoint, Turnstile requires secret/site, SSO requires metadata or pubkey, TOTP+non-memory requires a 32-byte `ACCOUNTING_SECRET_KEY`, port/limit/6 durations positive); called in `main.go` after the real logger and before `telemetry.Init` | `config/config.go`, `config/config_test.go`, `cmd/*/main.go` | ≥8 cross-field negatives + 1 positive all pass; `driver=postgres` without `DATABASE_URL` etc. `Fatal`s on startup naming the variable |
| P0-11 | **[B10] Exchange-rate goroutine cleanup**: inside `NewServer`, `updaterCtx,cancelUpdater:=context.WithCancel(...)`, `StartDailyExchangeRateUpdater(updaterCtx,…)` (non-TestMode only), `server.RegisterOnShutdown(cancelUpdater)` (unconditional, satisfies vet lostcancel) | `httpserver/server.go`, `ledger/rates_test.go` | `grep 'StartDailyExchangeRateUpdater(context.Background()' server.go`=0; a leak test shows the goroutine count returning to baseline ≤1s after cancel |
| P0-12 | **[B12] golangci-lint v2 + govulncheck toolchain**: add `backend/.golangci.yml` (`default:none` + a curated set of ~20 linters: errcheck/govet/staticcheck/ineffassign/unused/gosec/bodyclose/rowserrcheck/sqlclosecheck/noctx/contextcheck/errorlint/nilerr/… + gofmt/goimports formatters); pin golangci-lint and govulncheck versions in the `go.mod` `tool` block | `backend/.golangci.yml`, `backend/go.mod`, `go.sum` | `go tool golangci-lint config verify` passes; ≥15 linters enabled; injecting an unchecked error makes `golangci-lint run` exit non-zero |
| P0-13 | **[B12] Makefile integration**: append `go tool golangci-lint run ./...` to `backend-lint`; add `backend-vuln` running `go tool govulncheck ./...`; update `.PHONY` | `Makefile` | `make backend-lint` runs gofmt→tidy→vet→golangci-lint; `make backend-vuln` exits 0 on a clean tree |
| P0-14 | **[B12/N8] CI completion**: add `services.postgres` (`postgres:16` + `pg_isready` health) and a job-level `DATABASE_URL` to the `go.yml` `backend-and-cli` job, and add `make backend-build` and `make backend-vuln` steps | `.github/workflows/go.yml` | CI logs show `TestRecordStorePostgresRoundTrip` running (not skipped); the job fails if golangci-lint/govulncheck/build fails |

**Phase exit criteria**: see the S1–S10, B9, B10, B12 items in §6.2.

---

### P1 — Persistence Refactor (the core of this plan; the foundation for P3/P4)

> Goal: use goose embedded migrations + single-point driver selection + `WithTx`, replacing the single JSON table (B1) with a relational schema carrying FK/CHECK/unique constraints and keyset indexes, pushing pagination down to SQL (B6), removing the file driver and all Snapshot stores (B3), and delivering the data-migration subcommand and a CI Postgres integration test (B12).

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P1-1 | **The `internal/storage` package**: `Open(driver,url,dir)` establishes the connection and selects the dialect at a single point (reusing `persistence.OpenSQL`'s connection logic minus the DDL); `//go:embed migrations/{postgres,sqlite}/*.sql` + `Migrate()` runs `goose.Up`; `WithTx(ctx,fn)` is the UnitOfWork; `DBTX` is the minimal execution surface (satisfied by both `*sql.DB` and `*sql.Tx`, so domain packages do not import storage); `Rebind(dialect,q)` moves here | `storage/{storage,migrate,uow,dbtx}.go`, `storage/migrations/{postgres,sqlite}/00001_core_schema.sql`, `go.mod` | After `Open+Migrate` on both sqlite and postgres, a `goose_db_version` table exists at the latest version; a `WithTx` whose fn returns an error rolls back with no residue; `grep -rn pingAndMigrate internal`=0 |
| P1-2 | **First relational schema migration** (Appendix A): 14 core tables + `account_shared_books`, with columns/types (BIGINT cents, timestamptz UTC), FK, CHECK (`entries.amount_cents>0`, transfer requires destination, `postings` column-level CHECK), unique constraints, and keyset indexes; both Postgres and SQLite dialects | `storage/migrations/{postgres,sqlite}/00001_core_schema.sql` | Violating writes are rejected by the DB: `entries.amount_cents=0`, a duplicate `book_members (book_id,user_id)`, and a transfer missing `destination_account_id` each raise a constraint error; `entries_book_keyset_idx` exists |
| P1-3 | **ledger SQL Repository**: `SQLRepository` implements every `ledger.Store` method reading/writing the core tables directly; list methods gain the pagination-pushdown signature `(…,limit,offset)([]T,int,error)`, entries use the keyset index + `COUNT(*)`, and the Go-side full sort is removed; `ReplaceExchangeRates` uses `WithTx` DELETE+INSERT | `ledger/{sql_repository,store,entries,books,accounts_categories,sql_store,sql_repository_test}.go` | `grep -rn accounting_records ledger`=0; list SQL contains `LIMIT/OFFSET/ORDER BY`; `EXPLAIN QUERY PLAN` hits the keyset index with no SORT |
| P1-4 | **auth/audit/imports SQL Repository**: auth writes `users`/`sessions` (`sessions_user_idx` supports S1), audit writes `audit_events` (`seq` reserved for S6), imports write `import_batches`/`import_rows` (parent/child, `SaveBatchIfAbsent` becomes `ON CONFLICT(user_id,source_hash) DO NOTHING`); the 7 auth ephemeral namespaces migrate into the `auth_kv` residual table | `{auth,audit,imports}/sql_repository.go`, `storage/migrations/*/00002_auth_kv.sql`, `{auth,audit,imports}/sql_store.go` | `grep -rn accounting_records {auth,audit,imports}`=0 (only `auth_kv`); email is matched case-insensitively, a duplicate email raises a unique conflict; audit list SQL carries `LIMIT/OFFSET` |
| P1-5 | **Remove the file driver + single-point driver selection**: delete 7 source files (`persistence/{snapshot,jsonfile}.go`, each domain's `file_store.go`, `auth/store_snapshot.go`) + 4 `*_file_store_test.go` + the dead `Snapshot()/NewMemoryStoreFromSnapshot`; converge `server.go` to a single `openStore(cfg)` (`memory` vs SQL), removing all 5 switches and `case "file"` | `httpserver/{server,import_service}.go` + the files above | `grep -rn 'SnapshotStore\|FileStore\|NewFileStore' backend`=0; `grep -c 'strings.TrimSpace(cfg.Persistence.Driver)' httpserver`=1; `go build ./...` passes |
| P1-6 | **[B6] Consolidate the 4 duplicated paginators**: add `internal/paging.Normalize(page,pageSize,def,max)(limit,offset)` (with the audit overflow guard); delete `ledger.paginate`/`auth.paginate`/the two inline slices, calling `Normalize` and passing the result to the repo | `paging/paging.go`, `ledger/{pagination,entries}.go`, `auth/pagination.go`, `audit/service.go` | `grep -rn 'func paginate' internal`=0; the audit overflow case returns an empty page without panicking; list counts/totals match the pre-refactor values |
| P1-7 | **cli `migrate-data` subcommand**: read the 18 namespaces from the old file snapshots or the old `accounting_records`, and idempotently upsert them into the new schema in FK-dependency order (users→books→…→audit_events) inside `WithTx`; `--dry-run` counts only, no writes | `cli/internal/app/{migrate_data,app,migrate_data_test}.go`, `cli/go.mod` | Migrating a sample old DB exits 0 with per-table row counts equal to the source namespace counts, a second run is idempotent, and `--dry-run` writes nothing; covers file→sqlite and sqlite→sqlite |
| P1-8 | **[B12] CI Postgres integration**: add `services.postgres:17` + `DATABASE_URL` to `go.yml`; add `storage_integration_test.go` (Open→Migrate→per-Repository write/read round-trip + CHECK/UNIQUE violations rejected), with `t.Skip` when `DATABASE_URL` is empty | `.github/workflows/go.yml`, `storage/storage_integration_test.go`, `persistence/sql_store_test.go` | CI logs show Postgres healthy and the integration test running (not skipped); both dialects run the same constraint cases |

**Phase exit criteria**: see the B1/B2/B3/B6/B12 items in §6.2.

---

### P2 — API Contract and Error Model

> Goal: move all routes to `/api/v1` in a single clean cutover (no `/api` alias — the product is pre-launch with no external clients, so backward compatibility is explicitly a non-goal), reach 100% OpenAPI coverage, upgrade `contract.yml` to a real three-stage check, and unify errors as RFC 9457 problem+json (a governed code enum + `request_id`, Error-level logging for 5xx). **Reconcile with §1.5 already-closed items: do increments only, not from scratch.**

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P2-1 | **[B7] Clean cutover to `/api/v1`**: change `registerRoutes`' `router.Group("/api")` to `router.Group("/api/v1")` — no double registration, no alias middleware; change `runtime-config.apiBase` and `openapi.yaml servers.url` to `/api/v1`; migrate frontend `/api/` literals to `/api/v1/` in the same change set (old un-versioned `/api/*` paths fall through to the existing JSON-404 handling for the `/api/` prefix, so any missed caller fails loudly in tests instead of surviving silently) | `httpserver/routes.go`, `docs/api/openapi.yaml`, `web/src/lib/api/*.ts` | `/api/v1/health` returns 200 and `/api/health` returns a JSON 404; `grep 'router.Group("/api")' backend`=0 (only `/api/v1` remains); the frontend's `/api/` literal count (excluding `/api/v1/`) reaches zero; `GET /api/v1/runtime-config` reports `apiBase:"/api/v1"`; `make e2e` green in the same change set |
| P2-2 | **[N6] Remove the unauthenticated demo endpoint**: delete `routes.go:120-125`'s `api.GET("/ledger/summary",…)` (keep the `authLimiter` declaration) and the matching `openapi.yaml`/`paths/ledger.yaml` entries; if `ledgerService.Summary` has no other reference, remove it too (book-level totals use the retained `/books/:bookID/ledger/summary`) | `httpserver/routes.go`, `docs/api/{openapi,paths/ledger}.yaml`, `ledger/service.go` | Both `/api/v1/ledger/summary` and `/api/ledger/summary` return 404; `grep 'ledger/summary' docs/api` leaves only `books/{bookID}/ledger/summary` |
| P2-3 | **[B7/N7] problem+json + stable code + tiered logging**: add `problem_codes.go` (a 16-value `ProblemCode` enum + `problemRegistry{status,title}` + `defaultCodeForStatus` + `messageCodeIndex`); rewrite `api_error.go` around `ProblemDetail` (`c.Data(status,"application/problem+json…")`, single emitter `respondProblem`), keeping `respondAPIMessage/respondAPIError` as thin adapters (avoids touching all 96 sites) and deleting `apiErrorCode`; inside `respondProblem`, 5xx→Error / 429→Warn / 4xx→Debug; `respondLedgerError`'s default routes to `CodeInternalError`; add a `RequestID()` middleware (after Recovery, before the logger, in `server.go`) | `httpserver/{api_error,problem_codes,requestid,server,routes,api_error_test}.go` | Every error response is `Content-Type: application/problem+json` with a body carrying `type/title/status/detail/code/requestId`; `grep apiErrorCode backend`=0; every response has a non-empty `X-Request-ID` matching `body.requestId`; a 500 logs at `level=error` (with code+request_id) |
| P2-4 | **OpenAPI to 100% coverage + problem+json**: replace `schemas.yaml`'s `ErrorResponse` with `ProblemDetail` outright (with the 16-value `code.enum`; no `ErrorResponse` alias is kept — the regenerated types and the frontend `apiClient`/`apiErrorMessage` switch in the same change set); change `components/responses/Error` content to `application/problem+json`; reconcile every route's default:Error response; regenerate frontend types with `pnpm --dir web run gen:api` | `docs/api/{schemas,openapi,paths/*}.yaml`, `web/src/lib/api/generated/schema.d.ts`, `web/src/lib/{apiClient,apiErrorMessage}.ts` | The backend-routes ↔ openapi-paths diff is empty (coverage script); redocly lint 0 errors; `ProblemDetail.code.enum` matches the backend `problemRegistry` key set; `grep -rn ErrorResponse docs/api`=0; `check:api` shows no diff |
| P2-5 | **[reconcile §1.5] `contract.yml` three-stage real validation**: split into spec-lint (`redocly lint --max-problems 0`) + backend-contract (`go test -run Contract`, switching to the fully 3.1-capable `pb33f/libopenapi(-validator)` in place of `kin-openapi`, with `BasePath` locating `docs/api` to resolve external `$ref`s) + frontend-types (keep `check:api`); add a problem+json error contract case | `.github/workflows/contract.yml`, `httpserver/contract_routes_test.go`, `backend/go.mod`, `go.sum`, `Makefile` | All three jobs green; an invalid schema fails spec-lint; a handler returning an out-of-contract field fails backend-contract; `grep kin-openapi backend`=0, `go.mod` has `pb33f/libopenapi(-validator)` |

**Phase exit criteria**: see the API-contract and error-model items in §6.2.

---

### P3 — Import Domain Refactor (depends on P1's transaction capability)

> Goal: sink the whole apply orchestration from `import_routes.go` (730 lines) + `import_members.go` (153 lines) into `internal/imports`, leaving the HTTP layer with only DTO/authz/audit (B5, `import_routes.go` < 300 lines); wrap the entire apply in a single transaction with a database-level idempotency claim, eliminating duplicate postings and concurrent double-writes (B4/N4/N5).

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P3-1 | **Orchestration-port skeleton**: add `LedgerPort` (9 methods, reusing ledger's exact signatures), `UserResolver`, and `TxManager.WithinApply(ctx,fn)` (`ApplyTx` exposes `Ledger()`/`Batches()`) to `imports`; the `ApplyRequest`/`ApplyResult` types; add `applying` to `BatchStatus` | `imports/{ports,apply_types,model}.go` | `go build imports` passes; `LedgerPort` signatures match `ledger.Service` exactly; the `applying` constant exists |
| P3-2 | **[B4] DB-level claim/finalize/revert**: add `ClaimForApply(userID,batchID)(Batch,claimed,err)`/`FinalizeApplied`/`RevertToPreview` to `imports.Store`; Memory uses a mutex CAS, SQL a conditional `UPDATE … WHERE status='preview'` (Postgres may use `SELECT … FOR UPDATE`), with `applied`→replay and `applying`→`ErrConflict` | `imports/{store,sql_store}.go` | Two goroutines concurrently `ClaimForApply` the same preview batch and exactly one gets `claimed=true`; the SQLite/Postgres conditional-update unit tests pass; `-race` shows no data race |
| P3-3 | **[B4/N4/N5] Single-transaction `Service.Apply`**: within `WithinApply`, claim→members→references→per-row entries→`FinalizeApplied` are atomic; soft failures (unmapped/unsupported type) accumulate into `SkippedRows` without stopping, while hard failures (amount/date parse error, `CreateEntry` err) `return err` and roll back the whole batch (members/references/entries all reverted); `ImportedCount==0` also rolls back; idempotent replay returns `Replayed=true` | `imports/{service,apply}.go` | Two serial Applies: the first is `Replayed=false` and creates N entries, the second is `Replayed=true` returning the same Entries, and the batch entry count is always == ImportedCount with no duplicates; injecting an error at row K rolls back to preview with 0 entries and 0 residual members/accounts/categories |
| P3-4 | **[B5] Orchestration sink + HTTP slimming**: move all ledger-facing helpers in `import_routes.go:214-715` (references/mapping/commit/replay) and all 7 functions of `import_members.go` into `imports` (package-private, using `tx.Ledger()`), and delete `import_members.go`; converge the apply/preview handlers to `actor→decodeStrictJSON→importService.Apply→audit→c.JSON` | `imports/{apply,references,member_resolve,mapping}.go`, `httpserver/{import_routes,import_members}.go` | `wc -l import_routes.go`<300 (target ≈120); `import_members.go` deleted; `grep 'ledgerService\.\|authService\.' import_routes.go`=0; regression tests green |
| P3-5 | **DI assembly convergence**: add `import_adapters.go` (`ledgerPortAdapter`/`authResolverAdapter`/`sqlTxManager` (via `records.WithTx`)/`memoryTxManager` (serialize, for tests)); make `newDefaultImportService(cfg,db,dialect,ledgerService,authService)` a single-point selector and delete `case "file"` | `httpserver/{import_service,import_adapters,routes,server}.go` | `grep -c 'strings.TrimSpace(cfg.Persistence.Driver)' import_service.go`=1 with no `case "file"`; memory-driver preview+apply e2e passes; the SQL driver uses a real transaction |
| P3-6 | **Concurrency/rollback/dual-dialect tests**: `fakeLedgerPort` (programmable error at row K) + concurrent double-apply (exactly one first-commit, `CreateEntry` total calls == single-batch rows) + SQLite/Postgres real-transaction integration | `imports/{apply_test,apply_concurrent_test,apply_sql_test}.go` | `go test imports -race -run 'Apply\|Concurrent'` is green with no flake over ≥50 iterations; `grep httpserver imports/*_test.go`=0 (imports does not depend back on the HTTP layer) |

**Phase exit criteria**: see the import-transaction/idempotency and layering items in §6.2.

---

### P4 — Observability and Double-Entry Core

> Goal: on top of the trace-only baseline, land OTel Metrics (RED / DB pool / import / auth / ledger) plus `/healthz` and `/readyz` (B8); on P1's `journal_entries` / `postings` tables, land a double-entry core (transfer = two account legs, balanced writes in the same transaction) with balance-reconciliation queries and a periodic task (B11/N1), keeping the user-visible Entry/Summary contract unchanged; sync `docs/arch/arch.md`.

| ID | Change & key steps | Key files | Row-level acceptance |
| --- | --- | --- | --- |
| P4-1 | **[B8] MeterProvider + Prometheus exporter**: `telemetry.Init` adds `sdkmetric.NewMeterProvider(WithReader(otelprom.New()))` + `otel.SetMeterProvider`; add `ACCOUNTING_OTEL_METRICS_ENABLED` (default true, independent of trace); `Shutdown` flushes the meter; promote `otel/metric` to a direct dependency | `telemetry/telemetry.go`, `config/config.go`, `go.mod` | `go build ./...` passes; `grep 'otel/metric ' go.mod` no longer shows `// indirect`; with `MetricsEnabled=true`, `bundle.meterProvider!=nil` and `otel.GetMeterProvider()` is non-noop |
| P4-2 | **Instrument registry + RED middleware + DB-pool callback**: `telemetry/metrics.go` defines all 19 instruments (Appendix D); `MetricsMiddleware` records requests/duration/active/errors (labels use the `c.FullPath()` template, no high-cardinality); the DB pool uses `RegisterCallback` + `sql.DB.Stats()` (skipped when `db==nil`) | `telemetry/metrics.go`, `httpserver/metrics_middleware.go`, `httpserver/server.go` | After a request, `/metrics` contains `http_server_requests_total{http_route="/api/health"}` etc.; the memory driver has no `db_*` and does not panic; the SQL driver has `db_client_connections_usage{state="in_use"}` |
| P4-3 | **Wire counters into handlers + `/metrics` route**: `router.GET("/metrics", gin.WrapH(promhttp.Handler()))` (top-level, bypassing AttachSession); auth-failure rate (`auth.Login`), import batches/rows (apply/preview handlers), and entry/posting writes (ledger) are recorded via injected metrics (with `if m!=nil` guards) | `httpserver/{server,import_routes,auth_routes}.go`, `auth/service.go`, `ledger/entries.go` | After a failed login, `auth_login_failures_total{reason="bad_password"} 1`; after one apply, `import_batches_applied_total{result="applied"}` and `ledger_entries_written_total`; with `MetricsEnabled=false`, no nil panic |
| P4-4 | **`/healthz` + `/readyz`**: `/healthz` always returns 200 without touching the DB (liveness); `/readyz` does `db.PingContext`(2s), 200 on success / 503 on failure (sanitized), returning `database:skipped` when `db==nil`; registered at the top level (no session, no limiter), keeping `/api/health` | `httpserver/{health,server}.go` | `/healthz` stays 200 when the DB is down; after closing the DB pool the SQL driver's `/readyz` is 503 with `checks.database≠ok`; memory returns 200 `skipped` |
| P4-5 | **[B11] Posting model + storage**: `ledger.Posting` (two-currency `AmountCents` in account currency + `ReportingCents` in reporting currency) + `PostingDirection`; add `CreateEntryWithPostings`/`ReplaceEntryPostings`/`DeleteEntryPostings`/`PostingsByBook`/`PostingsByAccount` to `Store`; align with the P1 `postings` table (fall back to the `ledger.postings` namespace if P1 is not merged) | `ledger/{model,postings,store,store_memory,sql_store}.go`, `storage/migrations/*/0002_postings.sql` | `go build` passes; after a write, `PostingsBy*` reads back with clone isolation; SQLite + Postgres round-trip pass |
| P4-6 | **[B11/N1] Balanced write path**: `buildPostings(entry,src,dst,rates)` generates two legs per the direction table (non-transfer = account leg + nominal leg; transfer = source credit + destination debit, two account legs); `assertJournalBalanced` (reporting-currency `\|Σdebit-Σcredit\|≤len(postings)` tolerance); `CreateEntry`/`UpdateEntry`/`DeleteEntry` switch to `*WithPostings` in the same transaction, with the external Entry JSON/validation unchanged | `ledger/{postings,entries,money}.go` | An expense generates exactly 2 postings with imbalance=0; a same-currency transfer's two account legs net to 0 in reporting currency; a cross-currency transfer nets within tolerance; a deliberately unbalanced input triggers `ErrInvalidInput`; a posting write failure rolls back the entry |
| P4-7 | **[N1] Balance reconciliation + Summary derived from postings**: `AccountBalance` = opening + Σdebit − Σcredit (account currency); `reconcileBook` runs the journal-imbalance query; `BookSummary.BalanceCents` becomes the sum of per-account balances converted to reporting currency, Income/Expense aggregate from nominal legs, and **non-transfer cases are byte-identical to the old switch** (locked by a golden test) | `ledger/{reconcile,service}.go` | For a transfer-free fixture the new/old Summary are field-identical; with a transfer, source/destination balances both move and a same-currency transfer contributes 0 to Balance; the journal-imbalance query returns 0 rows for every seed book |
| P4-8 | **Periodic reconciliation task**: `StartPeriodicReconciliation(ctx,interval)` (a shutdown-aware select like the rate updater, reusing the P0-11 lifecycle context) emits `ledger_reconciliation_mismatches_total` + an Error log on any imbalance (book_id in the log, not a label); `ACCOUNTING_LEDGER_RECONCILE_INTERVAL` defaults to 1h | `ledger/reconcile.go`, `httpserver/server.go`, `config/config.go` | Injecting one unbalanced posting bumps the mismatch counter by 1 and logs an Error after one cycle; cancelling the lifecycle context exits both background goroutines; TestMode does not start them |
| P4-9 | **Sync `docs/arch/arch.md`**: Observability (the instrument table + `/healthz`/`/readyz` contract), Data Model Direction (Journal/Posting landed, two-currency balance, reconciliation), Configuration (the two new env vars), and a security note (`/metrics`/`/healthz`/`/readyz` are top-level session-free, sanitized, no high-cardinality labels) | `docs/arch/arch.md` | The three sections contain the above; the instrument table matches `NewMetrics` registrations; no secrets or real connection strings appear |

**Phase exit criteria**: see the metrics, health, and double-entry items in §6.2.

## 5. Test Matrix

| Level | Coverage | Tools / location | CI gate |
| --- | --- | --- | --- |
| Static | lint, vulnerable dependencies, security patterns | golangci-lint v2 (incl. gosec), govulncheck, `go build ./...` (P0-12/13/14) | `go.yml`, must be 0 errors |
| Unit | domain invariants: money/FX rounding, entry validation, role policy, argon2id, keyring round-trip, posting balance | `go test -race -cover`, memory repositories | Must pass |
| Migration | every migration up/down reversible, replayable from zero to latest, both dialects (PG/SQLite) | goose + `storage_integration_test` (P1-8) | Must pass |
| Repository integration | full SQL Repository behavior, FK/CHECK/unique constraints enforced, pagination pushdown | SQLite (local) + Postgres service container (CI, P1-8) | Must pass, no skips allowed |
| Transaction/concurrency | import apply interrupt-and-replay no duplicates; concurrent apply commits once; session bulk-revocation races | integration (`apply_concurrent_test`/`apply_sql_test`, fault injection + concurrent goroutines) | Must pass |
| Contract | OpenAPI matches implementation (status, schema, problem+json shape); 100% route coverage; code enum matches registry | redocly lint + `pb33f/libopenapi-validator` httptest (P2-5) | `contract.yml` all three jobs must pass |
| Security regression | old session 401 after reset; lockout effective; oversized body 413; HSTS header present; TOTP ciphertext at rest; SSO token absent from logs; malformed config fail-fast | `httptest` + integration (`security_regression_test`/`config_test`/`server_test`) | Must pass |
| Observability | `/metrics` carries RED metrics; `/readyz` 503 on DB disconnect; reconciliation mismatch count | `server_test` + `telemetry_test` + a manual curl drill | Report-only, key assertions must pass |
| E2E | register→record→import→report end to end with no regression; frontend has no regression after the `/api/v1` switch | existing Playwright (`make e2e`) | `e2e.yml` must pass |
| Performance smoke | list/summary p95 and memory bound at ~100k entries (keyset pagination hit) | `go test -bench` + Postgres container (thresholds set by the team) | Report-only, non-blocking |

## 6. Acceptance Criteria

### 6.1 Functional regression

- `make lint` (including golangci-lint/gosec/govulncheck), `make test`, and `make e2e` are all green; the frontend passes without changes (the P2-1 `/api/v1` cutover, which lands together with the frontend literal migration in one change set, is the exception).
- No regression in existing API behavior (status codes, authorization semantics, pagination semantics); any difference must appear in the OpenAPI diff and be reviewed. The external `Entry`/`BookSummary` JSON schema remains unchanged after the P4 double-entry core lands (the contract test does not need changes).

### 6.2 Quantified exit criteria (each machine-checkable)

**P0 security/engineering**

1. After password reset / TOTP disable / `logout-all`, a session token issued before the change returns 401 on a protected route; `grep -rn DeleteSessionsByUser backend/internal/auth` hits 3 implementations. (S1)
2. An account enters exponential-backoff lockout after 5 failures, and a correct password is still 429 while locked (6→1m/7→2m/8→4m, capped at 1h); one subject exceeding the limit across many IPs is rejected. (S2)
3. A JSON body over 1 MiB returns 413; HTTPS responses carry `Strict-Transport-Security`, HTTP does not. (S3/S4)
4. Dumping the database/snapshot yields a `TOTPSecret` with the `enc:v1:` ciphertext prefix — no usable TOTP secret and no plaintext password material; the migration is a no-op on the second startup. (S5)
5. Failed-login events carry a stable `subjectHash`; a non-allowlisted user gets 403 on `/api/admin/audit`; adjacent audit events satisfy increasing `Seq` and a linking `PrevHash`, and a broken chain is detectable. (S6)
6. An Owner can add / change role / remove a member and unshare an account with audit events, Member/Viewer get 403, and removing/demoting the sole owner is refused; `Account` has `UpdateAccount`. (S7/N2/N3)
7. The SSO callback completes login via POST form and the URL/logs do not contain `sso_token`; `AUTO_PROVISION` defaults to false. (S8)
8. New password hashes match `^\$argon2id\$v=19\$m=19456,t=2,p=1\$`, legacy pbkdf2 still verifies and is transparently upgraded after one login; `grep -n smtp.SendMail backend/internal/auth`=0, and with an invalid cert `ForceTLSVerify=true` fails the send. (S9/S10)
9. Any malformed `ACCOUNTING_*` bool/duration/int value makes the process fail to start, naming the variable; `grep -c 'func readBool\|func readInt\|func readDuration' config.go`=0; `Config.Validate()` covers 11 cross-field rules. (B9)
10. `grep -c 'StartDailyExchangeRateUpdater(context.Background()' server.go`=0; the leak test shows the goroutine count returning to baseline ≤1s after cancel. (B10)
11. `go tool golangci-lint config verify` passes with ≥15 linters enabled; `make backend-vuln` exits 0 on a clean tree; CI shows `TestRecordStorePostgresRoundTrip` running (not skipped) and includes `go build`. (B12/N8)

**P1 persistence**

12. Business code no longer reads/writes `accounting_records`: `grep -rn accounting_records backend/internal/{ledger,auth,audit,imports}` leaves only `auth_kv`; each core entity has its own relational table and violating writes are rejected by the DB (the three cases: `entries.amount_cents=0`, duplicate `book_members`, transfer missing destination). (B1)
13. The schema is entirely managed by versioned migrations: `grep -rn 'pingAndMigrate\|ensureRecordSchema\|execAll' backend/internal`=0, and both sqlite and postgres have a `goose_db_version` table. (B2)
14. `grep -c 'strings.TrimSpace(cfg.Persistence.Driver)' backend/internal/httpserver`=1, `grep -rn 'SnapshotStore\|FileStore' backend`=0, and the 7 source + 4 test files are deleted. (B3)
15. List SQL contains `LIMIT ? OFFSET ?`, and `EXPLAIN QUERY PLAN` for entries hits `entries_book_keyset_idx` with no SORT; `grep -rn 'func paginate' backend/internal`=0. (B6)
16. CI runs the migration and integration tests on real Postgres, so the `DATABASE_URL`-gated `t.Skip` does not apply in CI (they actually run). (B12)

**P2 contract and error model**

17. All routes carry the `/api/v1` prefix and appear 100% in `docs/api/openapi.yaml` (the coverage script diff is empty); no `/api` alias exists — legacy un-versioned `/api/*` paths return a JSON 404, and `grep 'router.Group("/api")' backend`=0.
18. Error responses are all `application/problem+json` with a `code` from the 16-value governed enum and a `request_id`; `grep -rn apiErrorCode backend`=0; every response has a non-empty `X-Request-ID` matching `body.requestId`.
19. Both `GET /api/v1/ledger/summary` and `GET /api/ledger/summary` return 404 (the demo endpoint is deleted).
20. A request that triggers a 5xx logs at `level=error` (with code + request_id); `grep -rn kin-openapi backend`=0 and `contract.yml`'s three jobs are all green.

**P3 import domain**

21. Killing the process at any point during import apply and retrying yields a final entry count equal to the batch row count (no duplicates, no gaps); of two concurrent applies only one succeeds; after a hard failure the batch status stays preview with 0 residual members/accounts/categories. (B4/N4/N5)
22. `wc -l backend/internal/httpserver/import_routes.go`<300 with no `grep 'ledgerService\.\|authService\.\|MarkApplied'`; `import_members.go` is deleted; `grep -rn httpserver backend/internal/imports/`=0. (B5)

**P4 observability and double-entry core**

23. `/metrics` contains `http_server_requests_total`/`http_server_request_duration_seconds_bucket`/`http_server_active_requests`; the SQL driver also has `db_client_connections_usage`; labels contain no high-cardinality dimensions. (B8)
24. `/readyz` returns non-200 on DB disconnect and `database:skipped` on the memory driver; `/healthz` stays 200 when the DB is down. (B8)
25. Every non-transfer entry maps to a balanced posting set; a transfer produces paired postings across two accounts; the journal-imbalance query returns 0 rows for every seed book; the reconciliation query independently recomputes account balances consistent with the report. (B11/N1)
26. For a transfer-free existing fixture, the `BookSummary` derived from postings is byte-identical field-by-field to the pre-refactor value (golden). (N1)

### 6.3 Behavioral acceptance scenarios (must be automated tests)

| Scenario | Layer | Acceptance |
| --- | --- | --- |
| Register→login→create book→create accounts→record an entry→import→report | E2E | End to end with no regression and no untranslated strings |
| Old session keeps requesting after password reset | httptest + integration | The old token consistently returns 401 with no residual success state |
| Repeated login failures trigger lockout | integration | Beyond the threshold, even a correct password is 429; passkey/SSO channels are unaffected by password lockout |
| Import apply double-click / concurrent submit | integration (imports, -race) | Exactly one commit takes effect; `CreateEntry` total calls == single-batch rows |
| Hard failure mid-apply | integration | Whole-batch rollback: batch stays preview, 0 entries, 0 residual members/references |
| Client triggers a 5xx | httptest | The response is problem+json + `request_id`, the server logs at `level=error` |
| Cross-currency transfer recorded | unit + integration | Source/destination balances both move; the reporting-currency journal nets within tolerance |
| DB disconnect | integration | `/readyz` is 503, `/healthz` is 200, error text sanitized |
| Malformed / missing-required config | unit (config_test) | `Validate()` errors naming the variable and the process exits on startup |

### 6.4 Documentation acceptance

- The persistence, configuration, security, and observability sections of `docs/arch/arch.md` are updated in sync with the implementation; the `file` driver is removed from the docs and config table; the single-`accounting_records` description is updated to the relational schema.
- `docs/api/openapi.yaml` is the single source of truth, with `servers.url` = `/api/v1` and problem+json error responses; add a `docs/arch` page on migration and backup operations (including the SQLite→Postgres path and `cli migrate-data` usage).
- New endpoints (`/api/auth/logout-all`, `/api/admin/audit`, the member/share routes, `/healthz`, `/readyz`, `/metrics`) are registered in OpenAPI or the ops docs.

## 7. Acceptance Gate Matrix

Gate shorthand: **L** `make lint` (incl. golangci-lint/gosec/govulncheck); **G** backend/CLI `go test -race -cover ./...`; **Cn** contract (redocly + libopenapi httptest + `check:api`); **I** SQL integration (SQLite + CI Postgres); **X** transaction/concurrency integration; **E** Playwright E2E; **O** observability drill; **Doc** `docs/arch`/OpenAPI sync.

| Phase | Required automated gates | Key manual/targeted acceptance |
| --- | --- | --- |
| P0 security/engineering floor | L, G, E | Assertions for 401-after-reset, lockout, 413, HSTS, TOTP ciphertext at rest, fail-fast; CI runs Postgres integration (not skipped) |
| P1 persistence | L, G, I, Doc | No `accounting_records` (business domains), violating writes rejected by the DB, keyset hit, single driver point, `migrate-data` idempotent |
| P2 contract/error | L, G, Cn, E, Doc | `/api/v1` 100% coverage, legacy `/api/*` returns JSON 404 (no alias), problem+json + governed code, `request_id`, 5xx Error logging |
| P3 import domain | L, G, X, E | import_routes<300 lines, single-transaction atomic rollback, concurrency commits once, imports does not depend back on http |
| P4 observability/double-entry | L, G, I, X, O, Doc | RED/`/readyz`/`/healthz`, posting balance, 0 reconciliation imbalance, Summary golden-identical |

## 8. Dependency Changes

| Action | Package | Type | Rationale |
| --- | --- | --- | --- |
| Add | `github.com/pressly/goose/v3` | dependency | Embedded versioned SQL migrations (P1) |
| Add | `golang.org/x/crypto/argon2` | promote from indirect to direct | argon2id password hashing (P0-8; `x/crypto` already in go.mod) |
| Add | `go.opentelemetry.io/otel/sdk/metric` + `exporters/prometheus` | dependency | OTel Metrics + Prometheus reader (P4) |
| Add | `go.opentelemetry.io/otel/metric` | promote from indirect to direct | Meter API (P4) |
| Add | `github.com/prometheus/client_golang` (promhttp) | promote from indirect to direct | `/metrics` handler (P4) |
| Add | `github.com/pb33f/libopenapi` + `libopenapi-validator` | test dependency | Full OpenAPI 3.1 contract validation (P2-5) |
| Add | `golangci-lint` v2 + `govulncheck` (go.mod `tool` directives) | tool dependency | Static/vulnerability gates (P0-12) |
| Add | `@redocly/cli` (via `npx -y` in CI) | CI tool | OpenAPI spec lint (P2-5) |
| Remove | `github.com/getkin/kin-openapi` (test usage) | — | Incomplete 3.1 support, replaced by libopenapi (P2-5) |
| Do not add | Atlas / ORM auto-DDL | — | SQL-first explicit migrations are safer |
| Do not add | Third-party APM / telemetry SaaS | — | Data-ownership and privacy goals |
| Do not add | Redis (distributed sessions/limiting) | — | Not needed for the single-replica self-hosted case; noted as a later architecture decision |

## 9. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| The P1 repository rewrite introduces behavior regressions | Backfill behavior tests for the existing stores first, run old and new implementations against the same suite before switching, land per-domain PRs, and check list counts/totals match the pre-refactor values |
| Dual-dialect DDL drift (PG jsonb/timestamptz/IDENTITY vs SQLite TEXT/INTEGER) | The P1-8 storage integration test runs the same constraint cases (CHECK/UNIQUE violations must be rejected) on both dialects, with CI-provisioned Postgres forcing both to verify simultaneously; use the Appendix-A constraint reference table as a review checklist for both migration files |
| argon2id `m=19MiB` amplification DoS under high login concurrency | The login path is fronted by the P0-2 two-dimension limiter + account lockout; if needed, bound argon2 computation with a semaphore; parameters are centralized as constants so they can be tuned per machine class |
| Loss/misconfiguration of the TOTP KEK (`ACCOUNTING_SECRET_KEY`) permanently locks users out of 2FA | `Config.Validate` fails closed in production and checks 32 bytes; the `RETIRED` list supports "read-two, write-one" smooth rotation; the migration is idempotent and only encrypts; ops docs require key custody and backup |
| Account lockout becomes a lockout DoS via a victim's email | Backoff starts short and caps at 1h and clears immediately on success; the per-IP dimension is retained; passkey/SSO channels do not go through password lockout; the lockout response is a generic 429 that does not reveal account existence |
| Enforced SMTP STARTTLS breaks sending if the production SMTP does not support it | Probe the target SMTP's StartTLS before launch; retain a 465 implicit-TLS direct-connect config option; `ForceTLSVerify=false` is a local-only escape hatch and defaults to true |
| The SSO callback switch to POST form_post requires provider cooperation | Provide a GET compatibility fallback with `sso_token` log-redaction to remove the immediate token-in-log/Referer risk first, then push the form_post contract |
| A frontend `/api/` literal is missed in the `/api/v1` cutover | By design there is no alias, so a missed call site fails loudly as a JSON 404 instead of surviving silently: `sed`-replace all literals and `grep`-count to zero in the same change set, then gate the merge on `make e2e` and the contract jobs; backend and frontend land as one coordinated release (pre-launch, zero external clients) |
| The error-envelope switch (`{code,message,requestId}`→problem+json) breaks frontend error handling | Cut over in one coordinated change set: `code` stays the machine key so the `apiErrorMessage`/i18n mapping carries over as-is, `apiClient` parses the problem+json shape, and regenerated types + the contract jobs + `check:api` catch any drift in CI |
| P3 depends on P1's transaction capability | Gate P3-3 on P1's exit criteria; before P1 is ready, land P3-2's `ClaimForApply` (which already closes the concurrent double-write window) as an independently acceptable intermediate |
| The P4 Summary change regresses values for historical entries | Provide a one-time backfill migration (`buildPostings` over existing entries, idempotent by EntryID); the golden test locks non-transfer values; reconciliation must show 0 imbalance before and after |
| Double-entry scope creep | P4 does only "posting structure + balance invariant + reconciliation query + periodic task"; a full switch of reports to the posting basis is a separate proposal |
| The memory driver has no cross-store transaction atomicity | The memory driver is for tests only; atomicity/rollback acceptance runs against real SQLite/Postgres; `memoryTxManager` only carries orchestration-logic unit tests |

## 10. Implementation Sequence and PR Strategy

**Phase dependencies**: P0 is independent and parallelizable; **P1 must precede P3/P4**; P2 is independent but preferably after P1 (the contract stabilizes with the relational schema).

Recommended PR sequence (small PRs, clear ownership, mechanical changes separated from behavioral ones):

1. P0-9/10/11 (config fail-fast + rate-updater cleanup) — low-risk first, surfaces misconfiguration.
2. P0-12/13/14 (golangci-lint + govulncheck + CI Postgres/build) — establish gates first so later PRs are bound by them; fix or precisely suppress existing warnings with `//nolint:linter // reason`.
3. P0-1/3 (session revocation + HSTS/body limit), P0-8 (argon2id + SMTP), P0-2 (lockout) — security quick wins, each independent.
4. P0-4 (TOTP envelope encryption, with the one-time migration), P0-5 (audit chain + admin read), P0-6 (member surface), P0-7 (SSO) — each with matching contract entries.
5. P1-1/2 (storage package + first migration) — the foundation, review the DDL.
6. P1-3, P1-4 (per-domain SQL Repository, one domain per PR) — backfill existing store behavior tests before switching.
7. P1-5/6 (remove file driver + consolidate pagination), P1-7 (cli migrate-data), P1-8 (CI Postgres integration).
8. P2-1 (`/api/v1` clean cutover — backend + frontend literals in one coordinated change set), P2-2 (delete demo), P2-3/4 (problem+json + code enum + OpenAPI), P2-5 (contract three-stage check).
9. P3-1/2 (ports + claim) — can land the claim intermediate before P1 completes; P3-3/4/5 (sink + single transaction + assembly), P3-6 (concurrency/rollback tests).
10. P4-1/2/3/4 (Metrics + health), P4-5/6/7 (double-entry core + reconciliation + Summary), P4-8 (periodic task), P4-9 (arch docs).

Concurrent-work rules: one PR owns a directory or a narrow cross-cutting file set; announce mechanical PRs (e.g., the `/api/v1` sed, bulk gofmt) before other branches rebase; do not revert unrelated edits from another session; generated artifacts (`schema.d.ts`, go.mod tool) are updated only by their generator command.

## 11. Handoff Checklist

Before development starts:

- [ ] Assign phase owners; create issue labels `phase-0`…`phase-4`, `persistence`, `contract`, `security`, `observability`, `double-entry`.
- [ ] Confirm CI runner support for the Postgres service container, `go tool` tool dependencies, and race tests.
- [ ] Inventory each deployment environment's env list and complete it per the Appendix-B validation table (strict parsing will break configs that relied on the old silent fallback).
- [ ] Generate/custody `ACCOUNTING_SECRET_KEY` (32 bytes) and establish key backup and rotation procedures.
- [ ] Agree with the frontend handbook owner that the `/api/v1` cutover and the problem+json parsing change land in the same release as one coordinated change set (no compatibility window — the product is pre-launch).

Before each phase is accepted:

- [ ] Deliverables complete; the matching column in the §7 gate matrix is green; the matching §6.2 quantified items pass their machine checks.
- [ ] The relevant `docs/arch`/OpenAPI files are updated in the same PR; new endpoints are in the contract.
- [ ] Sensitive-data review: no secrets/plaintext/high-cardinality leakage in API errors, audit, logs, metric labels, or test fixtures.
- [ ] No hand-written file exceeds 800 lines (Go files preferably under 600); split by responsibility when over.

Before first release:

- [ ] All gates pass: L, G, Cn, I, X, E, O.
- [ ] New-user onboarding, record entry, import preview/apply, reports, account/member management, security settings, and logout are verified on mobile and desktop.
- [ ] The production Go binary serves all routes via the SPA fallback; `/metrics`/`/readyz` are visible in the operational environment and wired to alerting; `/metrics` external reachability is restricted at the ingress gateway.
- [ ] This handbook is archived under `docs/proposals/archives/` with an implementation-result summary and accepted deviations.

---

## Appendix A: Relational Schema DDL (Postgres primary)

> SQLite dialect deltas are in the table at the end; apart from placeholders and `jsonb` casting routed through `storage.Rebind` + `dataPlaceholder`, column/constraint/index names stay identical. goose files use `-- +goose Up/Down` and `StatementBegin/End` annotations.

```sql
-- migrations/postgres/00001_core_schema.sql  (+goose Up)

CREATE TABLE users (
    id text PRIMARY KEY, email text NOT NULL, status text NOT NULL,
    email_verified boolean NOT NULL DEFAULT false, totp_enabled boolean NOT NULL DEFAULT false,
    base_currency text NOT NULL DEFAULT '', password_hash text NOT NULL,
    totp_secret text NOT NULL DEFAULT '',                 -- P1 plaintext; enc:v1: after S5
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_status_chk CHECK (status IN ('pending_verification','active')));
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

CREATE TABLE sessions (
    token_hash text PRIMARY KEY, id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_email text NOT NULL, status text NOT NULL,
    expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX sessions_user_idx ON sessions (user_id);        -- supports S1 bulk revocation
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE books (
    id text PRIMARY KEY, owner_user_id text NOT NULL REFERENCES users(id),
    name text NOT NULL, reporting_currency text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX books_owner_idx ON books (owner_user_id);

CREATE TABLE book_members (
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL, display_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (book_id, user_id),
    CONSTRAINT book_members_role_chk CHECK (role IN ('owner','administrator','member','viewer')));
CREATE INDEX book_members_user_idx ON book_members (user_id);

CREATE TABLE account_groups (
    id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL, sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX account_groups_user_idx ON account_groups (user_id);

CREATE TABLE accounts (
    id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id text NOT NULL REFERENCES account_groups(id),
    name text NOT NULL, type text NOT NULL, currency text NOT NULL,
    opening_balance_cents bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounts_type_chk CHECK (type IN
        ('cash','savings','credit_card','loan','investment','payment_platform')));
CREATE INDEX accounts_user_idx ON accounts (user_id);
CREATE INDEX accounts_group_idx ON accounts (group_id);

CREATE TABLE account_shared_books (           -- backs Account.SharedBookIDs (many-to-many)
    account_id text NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, book_id));
CREATE INDEX account_shared_books_book_idx ON account_shared_books (book_id);

CREATE TABLE categories (
    id text PRIMARY KEY, book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    parent_id text REFERENCES categories(id) ON DELETE SET NULL,
    name text NOT NULL, direction text NOT NULL, sort_order integer NOT NULL DEFAULT 0,
    archived boolean NOT NULL DEFAULT false, raw_source_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_direction_chk CHECK (direction IN ('income','expense')));
CREATE INDEX categories_book_idx ON categories (book_id);
CREATE INDEX categories_parent_idx ON categories (parent_id);

CREATE TABLE entries (
    id text PRIMARY KEY, book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    creator_user_id text NOT NULL REFERENCES users(id), type text NOT NULL,
    account_id text REFERENCES accounts(id), destination_account_id text REFERENCES accounts(id),
    category_id text REFERENCES categories(id),
    amount_cents bigint NOT NULL, transaction_currency text NOT NULL,
    account_currency text NOT NULL, book_reporting_currency text NOT NULL,
    exchange_rate text NOT NULL DEFAULT '', occurred_at timestamptz NOT NULL,
    note text NOT NULL DEFAULT '', merchant text NOT NULL DEFAULT '',
    tags jsonb NOT NULL DEFAULT '[]'::jsonb, raw_source text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT entries_amount_positive_chk CHECK (amount_cents > 0),
    CONSTRAINT entries_type_chk CHECK (type IN
        ('expense','income','transfer','refund','reimbursement','borrow','lend','repayment')),
    CONSTRAINT entries_transfer_dest_chk CHECK
        (type <> 'transfer' OR destination_account_id IS NOT NULL));
CREATE INDEX entries_book_keyset_idx ON entries (book_id, occurred_at DESC, id DESC);  -- keyset pagination
CREATE INDEX entries_account_idx ON entries (account_id);

CREATE TABLE journal_entries (                -- B11 header (created in P1, written in P4)
    id text PRIMARY KEY, entry_id text NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    occurred_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT journal_entries_entry_key UNIQUE (entry_id));
CREATE INDEX journal_entries_book_idx ON journal_entries (book_id);

CREATE TABLE postings (                       -- debit/credit legs; per-journal balance strategy in §P4
    id text PRIMARY KEY, journal_id text NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    entry_id text NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    account_id text NOT NULL REFERENCES accounts(id),
    direction text NOT NULL, amount_cents bigint NOT NULL, currency text NOT NULL,
    occurred_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT postings_direction_chk CHECK (direction IN ('debit','credit')),
    CONSTRAINT postings_amount_positive_chk CHECK (amount_cents > 0));
CREATE INDEX postings_journal_idx ON postings (journal_id);
CREATE INDEX postings_account_idx ON postings (account_id, occurred_at);
CREATE INDEX postings_book_idx ON postings (book_id);

CREATE TABLE exchange_rates (                 -- global set, fully replaced by ReplaceExchangeRates
    currency text PRIMARY KEY, units_per_usd text NOT NULL,
    source text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now());

CREATE TABLE import_batches (
    id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source text NOT NULL, filename text NOT NULL DEFAULT '', content_type text NOT NULL DEFAULT '',
    source_hash text NOT NULL, parser_version text NOT NULL DEFAULT '', status text NOT NULL,
    detected_schema jsonb NOT NULL DEFAULT '{}'::jsonb, detected jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_count integer NOT NULL DEFAULT 0, warning_count integer NOT NULL DEFAULT 0,
    applied_book_id text REFERENCES books(id), applied_entry_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    applied_skipped_rows jsonb NOT NULL DEFAULT '[]'::jsonb, applied_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT import_batches_status_chk CHECK (status IN ('preview','applied','applying')),
    CONSTRAINT import_batches_hash_key UNIQUE (user_id, source_hash));  -- SaveBatchIfAbsent CAS
CREATE INDEX import_batches_user_idx ON import_batches (user_id);

CREATE TABLE import_rows (
    batch_id text NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    row_number integer NOT NULL, data jsonb NOT NULL,
    error_count integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (batch_id, row_number));

CREATE TABLE audit_events (                   -- monotonic seq, S6 hash chain reserved; actor_id nullable (failed login)
    id text PRIMARY KEY, seq bigint GENERATED ALWAYS AS IDENTITY,
    actor_id text, actor_email text NOT NULL DEFAULT '', action text NOT NULL,
    target_type text NOT NULL, target_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX audit_events_actor_idx ON audit_events (actor_id, created_at DESC);
CREATE UNIQUE INDEX audit_events_seq_key ON audit_events (seq);
```

**SQLite dialect deltas** (`migrations/sqlite/00001_core_schema.sql`): `timestamptz`→`TEXT` (ISO8601 UTC, default `strftime('%Y-%m-%dT%H:%M:%fZ','now')`); `bigint`→`INTEGER`; `boolean`→`INTEGER` + `CHECK(col IN (0,1))`; `jsonb`→`TEXT CHECK(json_valid(col))`; `GENERATED ALWAYS AS IDENTITY`→`INTEGER AUTOINCREMENT`; FK requires `PRAGMA foreign_keys=ON` (set by `storage.Open`); the `lower(email)` expression unique index and partial index/CHECK are all supported.

## Appendix B: Complete Environment-Variable Validation Table (B9)

`strict` = parsed by `loader.boolean/integer/duration`, so a malformed non-empty value makes `LoadFromEnv` return an error. `Validate()` = cross-field/range rules. (New variables: `ACCOUNTING_SECRET_KEY`, `ACCOUNTING_SECRET_KEY_RETIRED`, `ACCOUNTING_ADMIN_USER_IDS`, `ACCOUNTING_ADMIN_EMAILS`, `ACCOUNTING_OTEL_METRICS_ENABLED`, `ACCOUNTING_LEDGER_RECONCILE_INTERVAL`.)

| Variable | Type | strict | Validate() rule |
| --- | --- | :---: | --- |
| `ACCOUNTING_PERSISTENCE_DRIVER` | string | — | ∈ {memory,file,postgres,postgresql,sqlite} (trim+lower) |
| `ACCOUNTING_DATABASE_URL` (falls back to `DATABASE_URL`) | string | — | required when driver ∈ {postgres,postgresql} |
| `ACCOUNTING_OTEL_ENABLED` | bool | ✔ | when Enabled, `OTLP_ENDPOINT` non-empty |
| `ACCOUNTING_OTEL_METRICS_ENABLED` | bool | ✔ | — (default true) |
| `ACCOUNTING_AUTH_TURNSTILE_ENABLED` | bool | ✔ | when Enabled, SecretKey & SiteKey non-empty |
| `ACCOUNTING_AUTH_TURNSTILE_LOGIN_MODE` | string | — | ∈ {always, after_failure} |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_ENABLED` | bool | ✔ | when Enabled, MetadataURL or PublicKeyPEM non-empty |
| `ACCOUNTING_AUTH_EXTERNAL_SSO_AUTO_PROVISION_ENABLED` | bool | ✔ | default **false** (was true) |
| `ACCOUNTING_AUTH_TOTP_ENABLED` | bool | ✔ | when Enabled and driver≠memory, `ACCOUNTING_SECRET_KEY` required and decodes to 32 bytes |
| `ACCOUNTING_SECRET_KEY` | string | — | format `<kid>:<b64-32B>`; see TOTP rule |
| `ACCOUNTING_AUTH_EMAIL_SMTP_PORT` | int | ✔ | 1 ≤ port ≤ 65535 |
| `ACCOUNTING_AUTH_RATE_LIMIT_LIMIT` | int | ✔ | > 0 |
| `ACCOUNTING_AUTH_SESSION_TTL` / `_EMAIL_VERIFICATION_TTL` / `_EXTERNAL_SSO_STATE_TTL` / `_RATE_LIMIT_WINDOW` / `_TOTP_REPLAY_CACHE_DURATION` / `SHUTDOWN_TIMEOUT` | duration | ✔ | all > 0 |
| `ACCOUNTING_LEDGER_RECONCILE_INTERVAL` | duration | ✔ | > 0 (default 1h) |
| Other bools (`DEBUG`/`EMAIL_LOGIN_ENABLED`/`REGISTER_ENABLED`/`VERIFICATION_REQUIRED`/`FORCE_SMTP_TLS_VERIFY`/`PASSKEY_ENABLED`/`RATE_LIMIT_ENABLED`/`SESSION_COOKIE_SECURE`/`ENABLE_PPROF`/`OTEL_EXPORTER_OTLP_INSECURE`) | bool | ✔ | — |

Totals: 15 bool + 2 int + 6 duration go through strict parsing; 11 cross-field/range rules.

## Appendix C: Governed Error-Code Enum (P2)

> Governance principle: `code` is bound one-to-one to an HTTP status and a stable `title` and **never changes with message wording**; `detail` carries the dynamic text; a new error must be registered in `problemRegistry` first. `ProblemDetail` = `{type,title,status,detail,instance,code,requestId}`, media type `application/problem+json`. `title` is a stable English label; user-facing localized copy is derived from `code` on the frontend via i18n (per `docs/arch/arch.md`).

| ProblemCode | code string | status | title | Typical trigger |
| --- | --- | :---: | --- | --- |
| CodeInvalidRequestBody | `invalid_request_body` | 400 | Invalid request body | decodeStrictJSON |
| CodeValidationFailed | `validation_failed` | 400 | Validation failed | 400 default fallback |
| CodeInvalidLedgerInput | `invalid_ledger_input` | 400 | Invalid ledger input | `ErrInvalidInput` |
| CodeAuthenticationRequired | `authentication_required` | 401 | Authentication required | actor missing |
| CodeInvalidCredentials | `invalid_credentials` | 401 | Invalid credentials | login failure |
| CodeTOTPRequired | `totp_required` | 401 | Two-factor verification required | login needs TOTP |
| CodeAccessDenied | `access_denied` | 403 | Access denied | 403 default |
| CodeLedgerAccessDenied | `ledger_access_denied` | 403 | No access to this book | `ErrAccessDenied` |
| CodeNotFound | `not_found` | 404 | Resource not found | 404 default |
| CodeLedgerNotFound | `ledger_not_found` | 404 | Ledger resource not found | `ErrNotFound` |
| CodeConflict | `conflict` | 409 | Resource conflict | idempotency/unique/concurrent apply |
| CodePayloadTooLarge | `payload_too_large` | 413 | Request body too large | MaxBytesReader |
| CodeRateLimited | `rate_limited` | 429 | Too many requests | limiter/account lockout |
| CodeImportFailed | `import_failed` | 422 | Import processing failed | apply business failure |
| CodeExchangeRatesUnavailable | `exchange_rates_unavailable` | 503 | Exchange-rate service unavailable | ExchangeRates failure |
| CodeInternalError | `internal_error` | 500 | Internal server error | 5xx default fallback |

`respondProblem` is the single emitter: 5xx→`log.Error`, 429→`log.Warn`, other 4xx→`log.Debug` (carrying code/request_id/path); the legacy `respondAPIMessage/respondAPIError` remain as thin adapters (`codeForMessage`/`defaultCodeForStatus` normalization), and `apiErrorCode` is deleted.

## Appendix D: OTel Instrument List (P4, 19 instruments)

> The Prometheus exporter converts `.`→`_` and appends `_total`/`_bucket` by type. Labels are strictly low-cardinality; `http_route` uses the gin `c.FullPath()` template — **never** the raw path, book_id, or user_id.

| name (OTel) | /metrics rendering | type | labels | source |
| --- | --- | --- | --- | --- |
| http.server.requests | http_server_requests_total | Int64Counter | http_method, http_route, http_status_code | MetricsMiddleware |
| http.server.request.duration | http_server_request_duration_seconds | Float64Histogram | http_method, http_route, http_status_code | MetricsMiddleware |
| http.server.active_requests | http_server_active_requests | Int64UpDownCounter | http_method, http_route | MetricsMiddleware |
| http.server.errors | http_server_errors_total | Int64Counter | http_method, http_route, http_status_class | MetricsMiddleware |
| db.client.connections.usage | db_client_connections_usage | Int64ObservableGauge | state(in_use/idle) | `sql.DB.Stats()` |
| db.client.connections.max | db_client_connections_max | Int64ObservableGauge | — | `sql.DB.Stats()` |
| db.client.connections.wait_count | db_client_connections_wait_count_total | Int64ObservableCounter | — | `sql.DB.Stats()` |
| db.client.connections.wait_duration | db_client_connections_wait_duration_ms_total | Float64ObservableCounter | — | `sql.DB.Stats()` |
| import.batches.previewed | import_batches_previewed_total | Int64Counter | source | preview handler |
| import.batches.applied | import_batches_applied_total | Int64Counter | source, result(applied/replayed) | apply handler |
| import.rows.imported | import_rows_imported_total | Int64Counter | source | apply handler |
| import.rows.skipped | import_rows_skipped_total | Int64Counter | source | apply handler |
| auth.login.attempts | auth_login_attempts_total | Int64Counter | result(success/failure) | auth.Login |
| auth.login.failures | auth_login_failures_total | Int64Counter | reason(bad_password/unknown_user/locked) | auth.Login |
| ledger.entries.written | ledger_entries_written_total | Int64Counter | type, operation(create/update/delete) | ledger CUD |
| ledger.postings.written | ledger_postings_written_total | Int64Counter | direction(debit/credit) | buildPostings |
| ledger.reconciliation.runs | ledger_reconciliation_runs_total | Int64Counter | — | periodic task |
| ledger.reconciliation.mismatches | ledger_reconciliation_mismatches_total | Int64Counter | kind(journal_imbalance/account_drift) | reconcileBook |
| ledger.reconciliation.duration | ledger_reconciliation_duration_seconds | Float64Histogram | — | periodic task |

**Posting direction table** (P4-6):

| entry.Type | account-leg direction | counter leg | counter-leg subject | balance effect |
| --- | --- | --- | --- | --- |
| income / borrow / refund / reimbursement / repayment | debit | credit | Category (nominal) | + |
| expense / lend | credit | debit | Category (nominal) | − |
| transfer | credit (source) / debit (destination) | — | two account legs, no nominal | both accounts move |

Balance invariant (reporting currency): `|Σ debit.reporting_cents − Σ credit.reporting_cents| ≤ len(postings)` (absorbs ≤1-cent half-even rounding per leg). The balance-reconciliation queries (pure SQL) are in §P4-7 and `reconcile.go`.
