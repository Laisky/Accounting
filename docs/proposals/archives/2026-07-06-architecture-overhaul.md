# Architecture Overhaul — Implementation Result (archived)

- Status: ARCHIVED / implemented. Archived 2026-07-07.
- Outcome: Phases 0–4 and 6 complete; Phase 5 core delivered; Phase 7 partial. All hard
  quantitative exit criteria met. Full local acceptance green: `make test` (backend + CLI
  `-race`, 84 web unit tests), `make e2e` (all specs incl. axe a11y). `make lint` is green
  except `check:api`, whose `git diff --exit-code` flags the regenerated OpenAPI types as
  pending until they are committed alongside the spec (deterministic — regeneration adds only
  the `ClientTelemetry` schema); it passes in CI once committed.

## Quantitative exit criteria (section 10)

| Metric | Target | Result |
| --- | --- | --- |
| `MobileWorkspace.tsx` line count | < 300 | deleted; shell is `features/shell/MobileShell.tsx` (222 lines) |
| Widest props interface | ≤ 12 | 12 (`MobileWorkspaceHeader`, `MeView`) |
| Direct frontend `fetch(` calls | ≤ 2 | 2 (`lib/apiClient.ts`, `lib/telemetry.ts`) |
| Hand-written API response DTOs | 0 | 0 (generated from `docs/api/openapi.yaml`) |
| Manual routing path parsers | 0 | 0 (`useShellChrome` derives chrome from `useMatch`) |
| Server data in feature `useState` | 0 | 0 (Query hooks + contexts own server state) |
| Feature CSS `oklch(` literals | 0 outside tokens | 0 (312 primitive tokens in `styles/palette.css`) |
| ErrorBoundary layers | ≥ 2 | 2 (`main.tsx` app + `MobileShell` route) |
| GitHub Actions workflows | ≥ 3 | 4 (go, web, e2e, contract) |
| Frontend client telemetry | enabled + sanitized | error + Web Vitals, strict server allowlist |
| Dead-code gate | passes | `knip` green |
| Relative `../../` source imports | 0 | 0 |
| Authenticated shell bundle | ≥ 25% lower | 330 kB vs 461 kB pre-split (~28% lower) |
| Axe serious+ findings | 0 | 0 (5 views × light/dark in `e2e/a11y.spec.ts`) |
| Web unit coverage | toward 80/70 | 78.9% stmts / 70.3% branches (CI threshold 74/64) |

## Phase status

- Phase 0 (engineering floor / CI) — complete (pre-existing).
- Phase 1 (OpenAPI contract + generated types + apiClient + contract tests) — complete
  (pre-existing); extended with the telemetry schema.
- Phase 2 (declarative routing) — complete; router-derived chrome replaced the last manual
  path parsing.
- Phase 3 (state decomposition) — complete. Per-domain Query hooks (books/accounts/entries/
  categories/members/user) with invalidating mutations; Session/Book/Notice/Theme contexts;
  `MobileShell` layout route + `<Outlet/>`; every view fetches through hooks; monolith and its
  48-prop content dispatcher deleted; one shared `entries.all` query removed the duplicate
  full-ledger loads.
- Phase 4 (design system) — foundation complete. `styles/palette.css` (312 value-exact
  primitive tokens, generated + `check:tokens` gate), semantic + dark tokens in `tokens.css`,
  `data-theme` mechanism, `@layer` order, six tested UI primitives (`components/ui`), stylelint
  color-literal ban.
- Phase 5 (launch UX floor) — core delivered: destructive-action confirmation + 10s undo,
  keyboard bookkeeping (`n` / `/` / `?` / Esc) + shortcut sheet, PWA manifest + static-only
  service worker, Home empty-state CTA, WCAG axe scans green. Desktop density was already done.
- Phase 6 (observability) — complete: X-Request-ID middleware, `POST /api/telemetry/client`
  with a strict allowlist (unknown-field rejection proves no sensitive data is accepted) +
  OpenAPI entry + tests, two `AppErrorBoundary` layers, `lib/telemetry.ts` (window error +
  unhandledrejection + sampled Web Vitals), backend OTel `MeterProvider` with HTTP RED and
  domain counters, alert-drill test.
- Phase 7 (test architecture) — partial: `app.test.tsx` split retained (< 400 lines each),
  coverage thresholds enforced, and `e2e/a11y.spec.ts` (axe, light/dark) added with a shared
  `e2e/helpers.ts`.

## Accepted deviations (remaining work)

- Phase 4: full `@layer` migration of feature/base CSS, adoption of the primitives into every
  legacy form/dialog, and the `acct-`/`rec-`/… class-prefix rename are incremental. Legacy
  feature CSS still themes via the `.mobileShellThemeDark` shell class; `data-theme` is the
  canonical signal for the token layer. These overlap the separate
  `docs/proposals/2026-07-06-ui-ux-overhaul.md` (W1) and are best finished there.
- Phase 5: first-run onboarding flow (P5.2) and empty-state CTAs beyond Home (P5.1) are not yet
  built.
- Phase 7: MSW v2 handlers (P7.1/P7.2 — the hand-built fetch harness still backs the app tests),
  the full domain split of `e2e/accounting.spec.ts` (P7.4, only a11y extracted), and Playwright
  visual-regression baselines (P7.6, must be generated in CI's pinned browser image) remain.
- Lighthouse performance/PWA scoring is not measured locally (requires a running production
  build + CI harness).

---

# Architecture Overhaul Implementation Handbook

- Status: Delivery handbook for implementation planning
- Date: 2026-07-06
- Product stage: New product, not yet released. Breaking refactors are allowed when they reduce launch risk.
- Primary scope: `web/` architecture, API contract, observability, test structure, and CI gates.
- Explicitly out of scope: rewriting the bookkeeping domain model, changing double-entry invariants, changing persistence boundaries, or replacing React/Vite/Gin.
- Source-of-truth documents: `docs/arch/arch.md`, `docs/arch/frontend.md`, `docs/arch/i18n.md`.

## 1. Executive Decision

Accounting should complete a focused architecture reset before first release. The current implementation already contains a modern stack, but too much behavior is concentrated in one authenticated frontend shell, API contracts are implicit, CI is missing, and production observability is incomplete. Those issues directly conflict with the product goals in `docs/arch/arch.md`: correctness, data integrity, auditability, and user data ownership.

The reset is not a visual redesign for its own sake. It is a staged engineering program that makes the web client easier to reason about, gives backend and frontend teams a shared contract, adds enforceable quality gates, and creates a minimum production telemetry loop without sending bookkeeping data to third-party SaaS.

Implementation must proceed in phases. Each phase must be independently reviewable, testable, and reversible. The first merge must create CI gates; later phases must not depend on local manual discipline.

## 2. 2026 Practice Baseline

The recommendation below was checked against primary or official sources on 2026-07-06.

| Area | Current finding | Decision for this project |
| --- | --- | --- |
| API description | OpenAPI 3.2.0 is the latest published OpenAPI version, but `openapi-typescript` currently documents OpenAPI 3.0/3.1 support. | Use OpenAPI 3.1.2 for the first contract because it is current enough and supported by the chosen generator. Track OpenAPI 3.2 adoption as a later ADR after generator and validator support is confirmed. |
| API-generated TypeScript | `openapi-typescript` generates zero-runtime TypeScript types from OpenAPI 3.0/3.1 schemas. | Generate all frontend API response/request types from `docs/api/openapi.yaml`; hand-written response DTOs are prohibited after Phase 1. |
| Server state | TanStack Query v5 documents targeted invalidation, stale marking, background refetching, and mutation lifecycle support. | Use TanStack Query for server-owned data and mutations. Do not keep server data in feature-level `useState`. |
| Routing | React Router 8.1.0 documents declarative `<Routes>/<Route>` and `useSearchParams`; updating search params is navigation. | Stop manually parsing paths. Every authenticated view must have a canonical URL and use route params/search params. |
| Accessibility | WCAG 2.2 AA includes focus visibility, focus-not-obscured, target-size, error suggestion, and financial/data error-prevention criteria. | Treat WCAG 2.2 AA as launch baseline. Money-changing and data-deleting flows require reversible, checked, or confirmed actions. |
| Web performance | Core Web Vitals are LCP, INP, and CLS, with good thresholds of LCP <= 2.5s, INP <= 200ms, and CLS <= 0.1. | Collect sampled LCP/INP/CLS from the client and gate regressions with Lighthouse and Playwright where practical. |
| Observability | OpenTelemetry Go lists traces and metrics as stable, with logs still beta. | Add OTel Metrics alongside existing optional OTLP tracing; keep Zap as the application log source. |
| Network mocks | MSW intercepts outgoing requests in browser and Node.js without patching application fetch logic. | Replace hand-built fetch mocks with MSW handlers organized by OpenAPI operation. |
| Browser CI | Playwright documents GitHub Actions examples, browser dependency installation, and official container usage for Linux agents. | Run E2E, accessibility, and visual tests in CI using the Playwright-recommended setup or container. |

Reference links:

- OpenAPI latest specification: https://spec.openapis.org/oas/latest.html
- OpenAPI 3.1.2 specification: https://spec.openapis.org/oas/v3.1.2.html
- openapi-typescript documentation: https://openapi-ts.dev/
- TanStack Query v5 invalidation: https://tanstack.com/query/v5/docs/framework/react/guides/query-invalidation
- React Router Routes: https://reactrouter.com/api/components/Routes
- React Router useSearchParams: https://reactrouter.com/api/hooks/useSearchParams
- WCAG 2.2: https://www.w3.org/TR/WCAG22/
- Web Vitals: https://web.dev/articles/vitals
- OpenTelemetry Go: https://opentelemetry.io/docs/languages/go/
- MSW documentation: https://mswjs.io/docs/
- Playwright CI documentation: https://playwright.dev/docs/ci

## 3. Current-State Findings

Measurements below describe the current tree on 2026-07-06.

| ID | Finding | Evidence | Release risk |
| --- | --- | --- | --- |
| D1 | Authenticated workspace state is centralized in one giant component. | `web/src/features/mobile/MobileWorkspace.tsx` is 819 lines and contains 23 `useState` declarations. | State ordering and race behavior are hard to reason about; most frontend work conflicts in one file. |
| D2 | Props are too wide and cross-cutting. | `MobileWorkspaceContentProps` has 54 props; `MobileWorkspaceHeaderProps` has 21; `AccountsViewProps` has 16; `MeViewProps` has 15. | Adding a new state or mutation touches unrelated files and obscures ownership. |
| D3 | Routing is manual despite React Router being installed. | No real `<Routes>/<Route>` usage exists; `mobileTabFromPath` and related helpers parse `location.pathname`. | URLs, data loading, and view state are coupled; direct-open and back/forward behavior are fragile. |
| D4 | API fetch behavior is duplicated. | `web/src/lib/api/*.ts` contains 44 direct `fetch(` calls across 5 files. | Error parsing, credentials, abort behavior, and 401 handling can diverge silently. |
| D5 | API types are not contract-backed. | Frontend API modules define response types manually and use type assertions. | Backend field changes can compile while breaking runtime behavior. |
| D6 | Shared UI primitives are missing. | `web/src/components/` contains only three general components; forms and dialogs are reimplemented per feature. | Accessibility, keyboard handling, focus trapping, and destructive-action confirmation are inconsistent. |
| D7 | CSS uses global naming and un-tokenized color literals. | 21 CSS files, 6,093 CSS lines, 576 `oklch(` occurrences across 18 CSS files. | Dark mode, theming, contrast audit, and collision prevention cannot be handled systematically. |
| D8 | Runtime rendering failures are not contained. | No ErrorBoundary implementation is present. | A route-level render exception can produce a white screen with no server-visible signal. |
| D9 | Dead code and build artifacts are mixed into the source tree. | `ImportWorkspace.tsx`, `features/ledger/`, and old desktop CSS are no longer part of the active shell; build metadata files are present. | Contributors and agents can modify obsolete code or chase false dependencies. |
| D10 | Frontend engineering gates are weak. | No path alias, no Prettier, no type-aware ESLint/import rules, no stylelint, no dead-code gate. | Multi-agent edits create avoidable churn and classes of type/import bugs remain unchecked. |
| D11 | Tests are concentrated in oversized files. | `web/src/app.test.tsx` is 1,087 lines; `web/e2e/accounting.spec.ts` is 540 lines. | Failures are hard to localize; parallelization and ownership are poor. |
| D12 | Repository CI is absent. | `.github/workflows/` is missing. | `make lint`, `make test`, i18n checks, and E2E checks are advisory rather than enforced. |
| D13 | Observability is incomplete. | Backend has structured logs, optional OTLP tracing, and optional pprof, but no metrics; frontend has no error or Web Vitals telemetry. | Production failures and degraded UX cannot be diagnosed quickly. |
| D14 | Launch UX floor is not yet met. | Empty states, onboarding, keyboard bookkeeping, desktop density, dark mode, and destructive-operation safeguards are not systematic. | First users may hit avoidable friction in core bookkeeping flows. |

## 4. Guiding Principles

1. Correctness beats delivery speed. Any proposed shortcut that weakens data integrity, auditability, or user ownership is rejected.
2. Backend API behavior is the contract. Frontend types must be generated from the reviewed API description.
3. Server state belongs in Query. Client state belongs in small, named contexts or local component state.
4. URLs are product surface. Any authenticated page or detail view must be directly openable and refresh-safe.
5. Design-system work must improve operability, not create a generic component library. Build only the primitives needed by current flows.
6. Observability must be privacy-preserving. Client telemetry payloads use a whitelist and must not contain amounts, notes, account names, emails, tokens, secrets, or form contents.
7. CI is the merge authority. Manual local checks are useful, but a PR cannot merge unless CI passes.
8. Every phase must update the relevant `docs/arch` note when it changes durable architecture.

## 5. Target Architecture

### 5.1 Frontend Shape

```text
web/src/
|-- main.tsx
|-- App.tsx
|-- components/
|   |-- AppErrorBoundary.tsx
|   |-- LanguageSelector.tsx
|   |-- LoadingOverlay.tsx
|   |-- Spinner.tsx
|   `-- ui/
|       |-- Button.tsx
|       |-- Card.tsx
|       |-- EmptyState.tsx
|       |-- FormField.tsx
|       |-- Notice.tsx
|       `-- Sheet.tsx
|-- contexts/
|   |-- BookContext.tsx
|   |-- NoticeContext.tsx
|   |-- SessionContext.tsx
|   `-- ThemeContext.tsx
|-- hooks/
|   |-- useAccounts.ts
|   |-- useAudit.ts
|   |-- useBooks.ts
|   |-- useEntries.ts
|   |-- useImports.ts
|   |-- usePasskeys.ts
|   `-- useReports.ts
|-- lib/
|   |-- apiClient.ts
|   |-- apiErrorMessage.ts
|   |-- queryClient.ts
|   |-- telemetry.ts
|   `-- api/
|       |-- generated/
|       `-- *.ts
|-- features/
|   |-- accounts/
|   |-- auth/
|   |-- entry/
|   |-- home/
|   |-- imports/
|   |-- landing/
|   |-- me/
|   |-- record/
|   |-- reports/
|   |-- search/
|   `-- shell/
|-- i18n/
|-- styles/
|   |-- layers.css
|   `-- tokens.css
`-- test/
    |-- msw/
    `-- renderApp.tsx
```

### 5.2 State Ownership

| State class | Owner | Rules |
| --- | --- | --- |
| Authenticated user, actor, runtime config | `SessionContext` | Read once from session/runtime-config Query hooks; no prop drilling through feature trees. |
| Selected book, book currency, display currency, rate index | `BookContext` | URL/book selection updates invalidate affected Query keys. |
| Books, accounts, entries, categories, reports, imports, audit, passkeys | TanStack Query hooks | Components cannot call `fetch` directly or copy this data into durable local state. |
| Notifications and recoverable errors | `NoticeContext` | All notices are translatable and may include a request/error ID. |
| Theme | `ThemeContext` | Supports `system`, `light`, and `dark`; writes `data-theme` and persists user choice. |
| Form drafts, sheet open state, inline UI toggles | Local component state | Kept close to the component and reset on route/book changes as needed. |

### 5.3 API and Telemetry Flow

```text
docs/api/openapi.yaml
  |-- pnpm --dir web run gen:api
  |   `-- web/src/lib/api/generated/
  `-- backend httptest contract checks

React feature hook
  `-- domain API function
       `-- apiClient
            |-- credentials, JSON parsing, AbortSignal, ApiError
            |-- X-Request-ID propagation
            `-- 401 handling

Browser error or Web Vitals sample
  `-- sendBeacon /api/telemetry/client
       `-- backend whitelist validation
            |-- Zap structured log
            |-- OTel metric
            `-- alertpusher only for error-level events
```

## 6. Technical Decisions

| Decision | Adopt | Reject for this phase | Implementation note |
| --- | --- | --- | --- |
| API contract | OpenAPI 3.1.2 in `docs/api/openapi.yaml` with generated frontend types. | Hand-written DTOs; Go-comment-only spec generation; OpenAPI 3.2 until generator support is verified. | Add a future ADR to reassess OpenAPI 3.2 after tooling supports it end-to-end. |
| Fetch layer | One `apiClient.ts` wrapper. | Per-file `fetch` wrappers. | Only `apiClient.ts` and `telemetry.ts` may call `fetch` or `sendBeacon` directly. |
| Server state | TanStack Query v5. | Redux/Zustand/Jotai for server state; custom cache. | Query keys are a reviewed API, documented in `docs/arch/frontend.md`. |
| Client state | React Context plus `useReducer` where needed. | A broad global store. | Keep context values narrow and stable. |
| Routing | React Router 8 declarative routes, params, and search params. | Manual pathname parsing. | Use code splitting for reports, imports, search, and settings routes. |
| Runtime validation | Contract tests plus generated types. | Full frontend zod validation for all payloads. | Runtime validation is allowed only at trust boundaries where it catches real user input or telemetry risk. |
| CSS | Hand-written CSS with semantic tokens, `@layer`, OKLCH tokens, and feature prefixes. | Tailwind, CSS Modules, new component library. | Keep the existing no-framework direction in `docs/arch/frontend.md`. |
| UI primitives | Button, Sheet, FormField, Notice, EmptyState, Card. | A broad design system. | Sheet must handle focus trap, Escape, inert background, and safe-area spacing. |
| Testing mocks | MSW v2 handlers organized by OpenAPI operation. | Manual fetch mocks in app tests. | Same handlers serve Vitest and optional dev mock mode. |
| Observability | OTel Metrics, existing OTel tracing, Zap logs, client telemetry endpoint. | Third-party frontend telemetry SaaS. | Telemetry payloads are sanitized and rate-limited. |
| CI | GitHub Actions gates for Go, web, contract, and E2E. | Local-only gates. | Use `pnpm --dir web` and root `make` commands consistently. |

## 7. Implementation Phases

### Phase 0: Engineering Floor and CI

Goal: create enforcement before behavior-changing refactors.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P0.1 | Remove dead frontend code and build artifacts. | `web/src/features/imports/ImportWorkspace.tsx`, `web/src/features/ledger/`, obsolete CSS, `*.tsbuildinfo`, duplicate Vite config artifacts. | No live imports reference removed files; production build still serves active routes. |
| P0.2 | Add and apply `@/` path alias. | `web/tsconfig*.json`, `web/vite.config.ts`, `web/src/**/*.ts(x)`. | `rg '../../' web/src --glob '*.{ts,tsx}'` returns zero intentional source imports. |
| P0.3 | Add Prettier and run one mechanical formatting PR. | `web/package.json`, config files, formatted web source. | `pnpm --dir web run format:check` passes in CI. |
| P0.4 | Tighten TypeScript. | `web/tsconfig*.json`. | `noUncheckedIndexedAccess`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`, and `noFallthroughCasesInSwitch` are enabled with zero errors. |
| P0.5 | Upgrade frontend linting. | `web/eslint.config.js`, `web/package.json`. | Type-aware TypeScript ESLint and import rules pass. |
| P0.6 | Add stylelint and dead-code detection. | `web/package.json`, stylelint config, knip config. | Existing debt is either removed or explicitly ignored with comments; new un-tokenized colors are blocked after Phase 4. |
| P0.7 | Add GitHub Actions. | `.github/workflows/`. | PRs run web lint/i18n/test/build, Go vet/test, contract placeholders, and Playwright E2E. |
| P0.8 | Update architecture docs for gates. | `docs/arch/frontend.md`, `docs/arch/arch.md` as needed. | Docs list exact commands and CI expectations. |

Mandatory gates: `make lint`, `make test`, `pnpm --dir web run build`, `pnpm --dir web run test:e2e`.

### Phase 1: API Contract and Unified Network Layer

Goal: make backend/frontend integration explicit and type-safe.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P1.1 | Create `docs/api/openapi.yaml` with OpenAPI 3.1.2. | `docs/api/openapi.yaml`. | Covers every endpoint listed in `docs/arch/arch.md` API Boundary, including auth, books, accounts, categories, entries, imports, audit, health, and runtime config. |
| P1.2 | Define shared schemas. | `docs/api/openapi.yaml`. | Error schema includes `{ code, message, requestId? }`; paginated lists use one envelope shape unless a documented legacy shape is retained. |
| P1.3 | Generate frontend types. | `web/src/lib/api/generated/`, `web/package.json`. | `pnpm --dir web run gen:api` is deterministic; CI fails when generated output is stale. |
| P1.4 | Add backend contract tests. | `backend/internal/httpserver` tests. | Httptest responses validate against the OpenAPI document for success and representative error cases. |
| P1.5 | Implement `apiClient.ts`. | `web/src/lib/apiClient.ts`. | Handles credentials, JSON and 204 responses, error body parsing, `AbortSignal`, request ID extraction, and 401 behavior. |
| P1.6 | Convert domain API modules to thin functions. | `web/src/lib/api/*.ts`. | Domain modules contain URL/operation glue only and import generated types. |
| P1.7 | Centralize API error copy mapping. | `web/src/lib/apiErrorMessage.ts`, i18n locale bundles. | User-facing API error copy is mapped from `ApiError.code` and all locales pass `check:i18n`. |
| P1.8 | Normalize import apply signature. | `web/src/lib/api/imports.ts`, callers. | `applyWacaiImport` accepts one options object and no overload shim remains. |

Mandatory gates: lint, i18n, contract tests, Go tests, Vitest, build, E2E.

### Phase 2: Declarative Routing

Goal: make URLs canonical, direct-open safe, and independent from manual string parsing.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P2.1 | Replace manual authenticated routing with `<Routes>`. | `web/src/App.tsx`, `features/shell/`. | Public routes and authenticated routes are defined declaratively. |
| P2.2 | Add `RequireAuth` and app layout route. | `features/shell/RequireAuth.tsx`, `MobileShellLayout.tsx`. | Unauthenticated direct-open redirects to login and returns to the original URL after login. |
| P2.3 | Replace path helpers with params/search params. | Remove or reduce `mobile-workspace-utils.ts`. | No `mobileTabFromPath`-style parser is used for production routing. |
| P2.4 | Move search query into URL. | Search feature. | Search can be shared, refreshed, and navigated with back/forward. |
| P2.5 | Add route-level lazy loading. | Reports, imports, me/security, search route entries. | Initial authenticated shell bundle decreases by at least 25% from measured baseline or the variance is documented. |
| P2.6 | Verify SPA fallback. | Go static server tests, Playwright. | Direct-open works through Vite dev server and production Go binary fallback. |

Canonical route set:

- `/home`
- `/accounts`
- `/accounts/:accountId/transactions`
- `/record`
- `/entries/:entryId`
- `/reports/:dimension`
- `/imports`
- `/me`
- `/me/profile`
- `/me/security`
- `/search?query=...`

### Phase 3: State Decomposition

Goal: reduce `MobileWorkspace` to a shell and move data ownership into domain hooks.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P3.1 | Add `QueryClientProvider`. | `web/src/main.tsx`, `web/src/lib/queryClient.ts`. | Query defaults document retry, stale time, and error policy. |
| P3.2 | Define query keys. | `docs/arch/frontend.md`, hook files. | Keys include book/user scope and filter parameters; mutation invalidation references the same constants. |
| P3.3 | Add domain hooks. | `web/src/hooks/`. | Hooks cover books, accounts, entries, reports, imports, audit, and passkeys. |
| P3.4 | Add contexts. | `web/src/contexts/`. | Session, book, notice, and theme contexts are composed in shell layout. |
| P3.5 | Migrate views one at a time. | `features/home`, `accounts`, `record`, `me`, `search`, `entry`, `imports`, `reports`. | Each migrated view fetches through hooks and receives no server data through shell props. |
| P3.6 | Retire `MobileWorkspaceContentProps`. | `features/mobile/MobileWorkspaceContent.tsx` removed or replaced by `<Outlet />`. | Maximum view prop interface width is <= 12; shell line count is < 300. |
| P3.7 | Fix all-entry fetch behavior. | `useEntries`, API contract, search/detail flows. | Search and detail do not trigger duplicate full-ledger loads; pagination/filtering is shared through Query. |

Migration order:

1. Home
2. Accounts
3. Record
4. Me
5. Search
6. Entry detail
7. Imports
8. Reports

Each view PR must include tests for loading, success, empty, error, mutation, and route/book-change behavior.

### Phase 4: Design System Foundation

Goal: create reliable UI primitives and a maintainable CSS cascade without introducing a CSS framework.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P4.1 | Expand semantic tokens. | `web/src/styles/tokens.css`. | Tokens cover surfaces, ink, muted text, accent, positive, negative, warning, border, focus, spacing, radius, shadow, and type scale. |
| P4.2 | Add dark-mode token values. | `tokens.css`, theme context. | Light, dark, and system modes work; contrast is verified for common token pairs. |
| P4.3 | Add cascade layers. | `web/src/styles/layers.css`, all CSS imports. | Layer order is `tokens, base, components, features`; unlayered author CSS is eliminated intentionally. |
| P4.4 | Build UI primitives. | `web/src/components/ui/`. | Button, Card, EmptyState, FormField, Notice, and Sheet have tests and stories/examples where useful. |
| P4.5 | Replace bespoke forms/dialogs. | Record, accounts, me, import, entry views. | Focus trap, Escape handling, `aria-describedby`, and 44px touch target rules are not reimplemented per feature. |
| P4.6 | Tokenize colors. | All feature CSS. | `rg 'oklch\\(' web/src --glob '*.css'` returns only token files; stylelint blocks new feature literals. |
| P4.7 | Prefix feature classes. | Feature CSS. | New feature CSS uses clear prefixes such as `acct-`, `rec-`, `me-`, `entry-`, `imp-`, `rep-`, `shell-`. |

### Phase 5: Launch UX Floor

Goal: make first-use and daily-use workflows coherent across mobile and desktop.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P5.1 | Add zero-data empty states. | Home, accounts, record, reports, imports. | Every empty authenticated tab explains the state and offers one primary action. |
| P5.2 | Add first-run onboarding. | Shell, books/accounts/record flows. | A new user can create a book, select currency, create first accounts, and record a first entry in <= 3 minutes in E2E. |
| P5.3 | Improve desktop density. | Feature CSS. | At >=1024px, views use available width without showing a phone-card layout; tables/lists are scan-friendly. |
| P5.4 | Add keyboard bookkeeping. | Shell, search, record, sheet primitives. | `n` opens new entry, `/` focuses search, `?` opens shortcuts, Enter submits valid sheet forms, Escape closes modal sheets. |
| P5.5 | Add destructive-action protection. | Accounts, categories, entry detail. | Delete/archive actions require confirmation; entry delete has a 10-second undo window where feasible. |
| P5.6 | Meet WCAG 2.2 AA. | Whole web app. | Axe serious+ violations are zero; manual keyboard walkthrough passes; focus is never hidden by sticky chrome. |
| P5.7 | Add PWA install baseline. | `web/` manifest/assets/service worker. | App is installable; service worker caches static assets only and never caches API responses. |

### Phase 6: Observability Loop

Goal: make production failures diagnosable without leaking sensitive bookkeeping data.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P6.1 | Add request correlation ID. | Backend middleware, `apiClient.ts`. | Every API response includes `X-Request-ID`; frontend errors and notices can display/copy it. |
| P6.2 | Add client telemetry endpoint. | Backend route, OpenAPI spec. | `POST /api/telemetry/client` validates a strict payload whitelist and rate-limits callers. |
| P6.3 | Add frontend error capture. | `AppErrorBoundary`, `web/src/lib/telemetry.ts`. | Render errors, `window.onerror`, and unhandled rejections reach backend logs with a generated event ID. |
| P6.4 | Add Web Vitals sampling. | `telemetry.ts`, backend metrics. | Sampled LCP, INP, and CLS are logged/metriced without user-entered data. |
| P6.5 | Add backend OTel metrics. | `backend/internal/telemetry`. | HTTP request count, duration, error count, entry writes, import previews/applies, and login outcomes are emitted. |
| P6.6 | Add alert drill. | Tests or runbook. | A synthetic frontend error appears in backend logs and alertpusher without sensitive fields. |

Telemetry payload allowlist:

- `kind`: `error` or `vitals`
- `eventId`
- `requestId`
- `routePattern`
- `componentStackHash`
- `errorName`
- `errorMessageHash`
- `metricName`
- `metricValue`
- `rating`
- `navigationType`
- `userAgentFamily`
- `timestamp`

The payload must never include full URL query strings, form data, notes, amounts, account names, emails, tokens, one-time codes, password material, TOTP secrets, WebAuthn credential material, or external SSO tokens.

### Phase 7: Test Architecture Rebalance

Goal: replace oversized, fragile tests with feature-owned coverage and contract-aware mocks.

Deliverables:

| ID | Change | Files | Acceptance |
| --- | --- | --- | --- |
| P7.1 | Add MSW v2. | `web/src/test/msw/`. | Handlers are grouped by OpenAPI tag/operation and reuse generated types. |
| P7.2 | Replace hand-built fetch mocks. | Vitest setup and feature tests. | Tests exercise real app fetch calls through MSW. |
| P7.3 | Split `app.test.tsx`. | `features/*/__tests__/`, `test/renderApp.tsx`. | Existing 19 scenarios are mapped; assertion count does not decrease; no test file exceeds 400 lines. |
| P7.4 | Split E2E spec. | `web/e2e/`. | Specs cover auth, passkeys, bookkeeping, imports, landing, accessibility, and visuals. |
| P7.5 | Add accessibility automation. | `web/e2e/a11y.spec.ts`. | Five core authenticated views pass light/dark axe scans with zero serious+ findings. |
| P7.6 | Add visual regression. | `web/e2e/visual.spec.ts`, baselines. | Core views have baselines at 375, 768, 1024, and 1280 px in light and dark themes. |
| P7.7 | Add coverage thresholds. | `web/vite.config.ts`, CI. | Start at current coverage minus 5% to block regression; raise toward 80% statements and 70% branches after Phase 3. |

## 8. Acceptance Matrix

Gate abbreviations:

- L: `make lint` plus frontend ESLint, Prettier, stylelint, i18n, and dead-code checks once configured.
- T: `make test`.
- G: backend and CLI `go test -race -cover ./...`.
- W: frontend Vitest.
- B: `pnpm --dir web run build`.
- C: OpenAPI validation, generated type diff, and backend contract tests.
- E: Playwright E2E.
- A: automated accessibility scan.
- V: visual regression.
- M: observability drill.

| Phase | Required automated gates | Required manual/targeted acceptance |
| --- | --- | --- |
| P0 Engineering floor | L, T, B, E | CI workflows run on a PR; removed files have no live references; all manually written files remain < 800 lines. |
| P1 API contract | L, T, G, W, B, C, E | `fetch(` appears only in `apiClient.ts` and `telemetry.ts`; generated types replace hand-written response DTOs; API error copy is localized. |
| P2 Routing | L, T, W, B, E | Canonical URLs direct-open, refresh, and navigate back/forward in both Vite dev and production Go fallback. |
| P3 State decomposition | L, T, W, B, C, E | `MobileWorkspace` < 300 lines; `MobileWorkspaceContentProps` gone; mutation invalidation updates home, accounts, and reports without manual refresh. |
| P4 Design foundation | L, T, W, B, E, V | No feature CSS `oklch(` literals; dark and light themes meet contrast checks; UI primitives cover repeated dialogs/forms/notices. |
| P5 UX floor | L, T, W, B, E, A, V | First-run flow <= 3 minutes; keyboard-only entry creation passes; destructive operations are confirmed or reversible. |
| P6 Observability | L, T, G, W, B, C, E, M | Synthetic frontend error reaches backend log and alert path with request/event ID and no sensitive fields. |
| P7 Test rebalance | L, T, G, W, B, C, E, A, V | Scenario mapping proves existing tests were not dropped; MSW operation coverage matches OpenAPI operations. |

## 9. Behavioral Acceptance Scenarios

These scenarios must exist as automated tests unless marked manual.

| Scenario | Coverage owner | Acceptance |
| --- | --- | --- |
| Register -> login -> first book -> first accounts -> first entry. | E2E | Completes on mobile viewport and desktop viewport; no untranslated strings. |
| Session expiry during mutation. | Vitest + E2E | User sees a consistent localized sign-in prompt; no stale success state remains. |
| Network interruption during entry create. | Vitest | Request can be retried; no optimistic half-state remains after failure. |
| Rapid search typing. | Vitest | Only the latest query result is rendered; aborted/older responses do not overwrite current results. |
| Switch books while old requests are in flight. | Vitest | Old book data never appears in the new book view. |
| Import apply double-click. | Backend + Vitest + E2E | One committed import mutation occurs; second click is disabled or deduplicated. |
| Entry create mutation. | Vitest | Home summary, account balances, and reports update through Query invalidation. |
| Direct-open `/entries/:entryId`. | E2E | Works after login redirect and on production Go fallback. |
| Delete entry. | E2E | Confirmation appears; successful delete offers undo when feasible; audit event exists. |
| Client render exception. | E2E or integration | Route fallback appears; error/event ID can be copied; backend receives sanitized telemetry. |
| Dark mode. | Visual + a11y | Core views render without contrast failures or layout clipping. |
| Locale integrity. | CI | `check:i18n` passes for English, Simplified Chinese, French, Spanish, and Japanese. |

## 10. Quantitative Exit Criteria

| Metric | Current | Final acceptance |
| --- | ---: | ---: |
| `MobileWorkspace.tsx` line count | 819 | < 300 |
| Widest props interface | 54 | <= 12 |
| Direct frontend `fetch(` calls | 44 | <= 2 |
| Hand-written API response DTOs | Many | 0 for contracted API responses |
| Manual routing path parsers | Multiple | 0 in production routing |
| Server data stored in feature `useState` | Present | 0 durable server-data copies |
| Feature CSS `oklch(` literals | 576 occurrences across 18 files | 0 outside token files |
| ErrorBoundary layers | 0 | >= 2 |
| GitHub Actions workflows | 0 | >= 3 |
| Frontend client telemetry | None | Error and Web Vitals telemetry enabled and sanitized |
| Dead-code gate | None | knip or equivalent passes |
| Max manually written file length | 1,087+ lines in tests; 819 in source | < 800 repo rule, < 500 for new source/test files where feasible |
| Relative `../../` source imports | 67 | 0 intentional source imports |
| TypeScript safety switches | `strict` only | listed Phase 0 switches enabled |
| Authenticated shell bundle | unsplit baseline | >= 25% lower or documented exception |
| Axe serious+ findings | not measured | 0 |
| Lighthouse mobile authenticated shell | not measured | Performance >= 85, Accessibility >= 95, PWA installable |

## 11. Documentation Acceptance

Each phase must update documentation in the same PR when durable behavior changes.

Required doc updates:

- `docs/arch/frontend.md`: state ownership, Query key rules, routing rules, UI primitives, token/layer rules, class prefix rules, theme behavior, and test conventions.
- `docs/arch/arch.md`: API contract ownership, request ID behavior, telemetry endpoint, metrics, CI gate expectations, and any backend route changes.
- `docs/arch/i18n.md`: any i18n workflow changes caused by new checks or namespaces.
- `docs/api/openapi.yaml`: API fact source after Phase 1.
- This handbook: move to an archive path after completion and add an implementation summary with accepted deviations.

## 12. PR Strategy

Use small PRs with clear ownership. Do not mix mechanical churn with behavior changes.

Recommended sequence:

1. P0 dead-code removal and artifact cleanup.
2. P0 alias and lint/build config.
3. P0 formatting-only PR.
4. P0 CI workflows.
5. P1 OpenAPI skeleton and generator.
6. P1 apiClient and one migrated API module.
7. P1 remaining API modules and contract tests.
8. P2 routing shell.
9. P2 direct-open route tests.
10. P3 one PR per feature migration.
11. P4 primitives and token foundation.
12. P4/P5 feature-level UI replacement PRs.
13. P6 telemetry backend and frontend in two coordinated PRs.
14. P7 MSW and test split.

Rules for concurrent work:

- A PR owns a directory or narrow cross-cutting file set.
- Mechanical PRs must be announced before other feature work rebases.
- Do not revert unrelated edits from another session.
- Generated files must be updated only by the matching generator command.
- Any conflict between this handbook and `docs/arch/*` must be resolved explicitly in the PR.

## 13. Risk Register

| Risk | Level | Mitigation |
| --- | --- | --- |
| Phase 3 state migration changes behavior. | High | Migrate one view at a time; add race tests before deleting old paths; keep Query key review strict. |
| OpenAPI spec drifts from backend. | Medium | Backend contract tests and generated-type diff gate run in CI. |
| OpenAPI 3.2 pressure arrives before tooling is ready. | Medium | Use 3.1.2 now; write ADR criteria for 3.2 upgrade. |
| Prettier and alias rewrites create merge churn. | Medium | Keep formatting-only and alias-only PRs separate. |
| CSS layer migration changes specificity. | Medium | Migrate layers in one controlled pass and use visual regression immediately after. |
| Telemetry leaks sensitive bookkeeping data. | High | Whitelist payload, hash messages, rate-limit endpoint, add serialization tests, and manually review sample logs. |
| Visual regression baselines become noisy. | Medium | Pin Playwright browser/container versions and generate baselines in the same Linux environment used by CI. |
| UX phase expands beyond launch floor. | Medium | New product ideas go to a backlog unless they satisfy a listed Phase 5 acceptance item. |
| New dependencies increase supply-chain risk. | Low | Keep runtime additions minimal; lock versions; prefer devDependencies for tooling. |

## 14. Dependency Changes

| Action | Package | Type | Rationale |
| --- | --- | --- | --- |
| Add | `@tanstack/react-query` | dependency | Server-state cache, invalidation, mutation lifecycle. |
| Add | `web-vitals` | dependency | LCP, INP, CLS collection. |
| Add | `openapi-typescript` | devDependency | Generate frontend types from OpenAPI 3.1.2. |
| Add | `msw` | devDependency | Contract-aware network mocking. |
| Add | `prettier` | devDependency | Deterministic formatting. |
| Add | `knip` or equivalent | devDependency | Dead-code detection. |
| Add | `stylelint` and standard config | devDependency | CSS hygiene and token enforcement. |
| Add | `eslint-plugin-import-x` | devDependency | Import sorting, cycles, and unresolved import checks. |
| Add | `@axe-core/playwright` | devDependency | Automated accessibility checks. |
| Add | `@vitest/coverage-v8` | devDependency | Frontend coverage thresholds. |
| Add | `github.com/getkin/kin-openapi` | Go test dependency | Contract validation in backend tests. |
| Add | OTel Metrics SDK packages | Go dependency | Backend metrics through existing telemetry package. |
| Do not add | Redux, Zustand, Jotai | none | Server state and small client contexts cover current needs. |
| Do not add | Tailwind, CSS Modules, broad component library | none | Conflicts with existing frontend architecture direction. |
| Do not add | Sentry or other third-party telemetry SaaS | none | Data ownership and privacy goals favor first-party telemetry. |
| Do not add | zod for all API responses | none | Contract tests and generated types are the first-line control; runtime validation is reserved for selected trust boundaries. |

## 15. Handoff Checklist

Before development starts:

- [ ] Assign phase owners.
- [ ] Confirm CI runner capabilities for Go race tests, pnpm, and Playwright.
- [ ] Create issue labels for `phase-0` through `phase-7`, `contract`, `routing`, `state`, `design-system`, `observability`, and `test-architecture`.
- [ ] Create an ADR placeholder for OpenAPI 3.2 adoption criteria.
- [ ] Freeze unrelated broad frontend refactors until Phase 0 mechanical PRs land.

Before each phase is accepted:

- [ ] Phase deliverables are complete.
- [ ] Required gates in the acceptance matrix pass.
- [ ] Relevant `docs/arch` files are updated.
- [ ] New user-facing strings are present in every locale.
- [ ] No manually written file exceeds the 800-line repository rule.
- [ ] Sensitive data review is complete for API errors, telemetry, logs, tests, and docs.

Before first release:

- [ ] Full matrix passes: L, T, G, W, B, C, E, A, V, M.
- [ ] New-user onboarding, record entry, import preview/apply, reports, account management, security settings, and logout are verified on mobile and desktop.
- [ ] Production Go binary serves all SPA routes through fallback.
- [ ] Frontend telemetry and backend metrics are visible in the operational environment.
- [ ] This handbook is archived with an implementation result summary and deviations.
