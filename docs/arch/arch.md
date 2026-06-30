# Accounting Architecture

## Overview

Accounting is a single-repository web application for personal and small-team bookkeeping. The backend is written in Go, the frontend is a React/Vite application, and production deployments run as one Go web server that serves both API routes and the built static frontend from `web/dist`.

The initial scaffold is intentionally small: it establishes the process layout, request-scoped logging, API boundaries, static serving, and developer commands before adding persistence-heavy accounting behavior.

## Goals

- Provide a browser-first bookkeeping application for entering, reviewing, reconciling, and reporting financial transactions.
- Keep backend, CLI, frontend, documentation, and deployment files in one repository.
- Let `make dev` start a local Go API server and Vite frontend server for fast iteration.
- Let production builds copy `web/dist` beside the Go binary so the Go web server can serve the SPA.
- Mirror proven conventions from nearby projects: clear `docs/arch` ownership, Go domain packages under `internal`, React under `web`, structured logs, context propagation, and practical Make targets.

## Non-Goals

- This is not a multi-tenant SaaS architecture yet.
- This is not a double-entry accounting engine yet; the scaffold only includes a placeholder ledger summary service.
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

## Backend Architecture

The backend uses Gin because the reference projects already use Gin, `github.com/Laisky/gin-middlewares/v7`, and request-scoped structured logging. The server is assembled in `backend/internal/httpserver.NewServer`.

Key responsibilities:

- `cmd/accounting-server`: process startup, signal handling, graceful shutdown.
- `internal/config`: environment parsing with explicit defaults.
- `internal/logger`: global structured logger foundation plus `FromContext` for non-Gin business code.
- `internal/httpserver`: middleware, route registration, SPA fallback, and development proxy hooks.
- `internal/ledger`: domain use cases. Persistence and double-entry rules should land here before controllers grow complex.

Request handlers must retrieve the request logger with `gmw.GetLogger(c)` once per handler and pass `c.Request.Context()` into business services. Business logic should use context-aware loggers, not package globals.

## Frontend Architecture

The frontend is a Vite React app under `web/`. The first screen is an operational bookkeeping workspace, not a marketing landing page.

Expected direction:

- `src/main.tsx` owns bootstrapping.
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

Future accounting APIs should be grouped by domain:

- `/api/accounts`
- `/api/entries`
- `/api/categories`
- `/api/reconciliation`
- `/api/reports`

Date-range filters must use UTC and include the full final day by ending just before `00:00` of the next day.

## Data Model Direction

The durable model should evolve toward double-entry accounting:

- Account: asset, liability, equity, income, and expense ledgers.
- Journal entry: immutable business event with occurred-at time in UTC.
- Posting: one debit or credit line within a journal entry.
- Category: user-facing classification for reports and budgets.
- Reconciliation: statement-period matching and lock records.

The first persistent store should be hidden behind repository interfaces owned by the relevant domain package. Controllers should not build SQL directly.

## Configuration

Current backend environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `ACCOUNTING_ADDR` | `:8080` | Go HTTP listen address. |
| `ACCOUNTING_DEBUG` | `false` | Enables debug logging and Gin debug mode. |
| `ACCOUNTING_SERVER_NAME` | `accounting` | Public server label returned in runtime config. |
| `ACCOUNTING_WEB_DIST_DIR` | `../web/dist` | Built SPA directory served by the Go process. |
| `ACCOUNTING_WEB_DEV_URL` | empty | Optional reverse proxy target for non-API frontend routes. |

Secrets must not be logged or returned by runtime config endpoints.

## Developer Commands

```sh
make dev
make build
make lint
make test
```

Required Go verification gates remain:

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
- Specify the double-entry posting invariants.
- Add import/export formats for bank statements.
- Add a deployment document once Docker and CI exist.
