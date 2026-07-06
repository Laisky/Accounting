# Frontend Implementation Notes

These notes capture durable frontend decisions for the React/Vite web app. They supplement
`docs/arch/arch.md` and `docs/arch/i18n.md`; product behavior and user-facing copy still
belong in those source-of-truth documents and locale bundles.

## 2026 UI and CSS Baseline

- Keep the app on React 19's modern root and JSX transform. New components should use typed
  props, semantic HTML first, and no legacy React APIs such as `ReactDOM.render`, string refs,
  `findDOMNode`, or `defaultProps` on function components.
  Sources: [React 19 upgrade guide](https://react.dev/blog/2024/04/25/react-19-upgrade-guide),
  [React 19 release](https://react.dev/blog/2024/12/05/react-19),
  [TypeScript JSX docs](https://www.typescriptlang.org/docs/handbook/jsx.html).
- Treat Vite as a modern-browser build tool, not a polyfill layer. Use progressive enhancement
  and `@supports` for newer platform features that are not yet part of the chosen browser
  baseline.
  Sources: [Vite build guide](https://vite.dev/guide/build),
  [Vite features](https://vite.dev/guide/features).
- Prefer container queries for reusable panels and cards. Viewport media queries should be
  reserved for page-level shell changes, viewport units, and input-mode conditions.
  Source: [MDN container queries](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Containment/Container_queries).
- Migrate CSS toward explicit cascade layers in a deliberate pass. Unlayered author CSS outranks
  normal layered CSS, so do not mix partial layers casually in feature work.
  Source: [MDN `@layer`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/%40layer).
- Native CSS nesting is acceptable for shallow, component-local selectors. Avoid deep nesting
  because nested specificity follows `:is()`-like behavior in important cases.
  Source: [MDN CSS nesting](https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Nesting).
- Full-height app shells should not rely on raw `100vh`. Use `100svh` as the safe CSS fallback
  and upgrade app chrome to the `--app-viewport-height` variable set from `visualViewport.height`
  so browser address bars, bottom bars, and virtual keyboard changes are reflected in layout.
  Critical bottom or edge controls should account for `env(safe-area-inset-*)`.
  Source: [MDN viewport meta](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/meta/name/viewport),
  [MDN `env()`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/env),
  [MDN Visual Viewport API](https://developer.mozilla.org/en-US/docs/Web/API/Visual_Viewport_API),
  [web.dev viewport units](https://web.dev/blog/viewport-units).
- Continue using OKLCH for perceptual color tokens and `color-mix(in oklch, ...)` where derived
  states reduce duplication. Use `light-dark()` only with support checks or an explicit browser
  support decision.
  Sources: [MDN OKLCH](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/color_value/oklch),
  [MDN `color-mix()`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/color_value/color-mix),
  [MDN `light-dark()`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Values/color_value/light-dark),
  [MDN `color-scheme`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/color-scheme).
- Encode focus and touch rules as shared patterns over time. WCAG 2.2 AA requires target-size and
  focus visibility coverage; this bookkeeping UI should aim for 44-48px frequent touch targets,
  `:focus-visible` rings, and scroll padding where sticky chrome can obscure focused controls.
  Sources: [WCAG target size minimum](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html),
  [WCAG target size enhanced](https://www.w3.org/WAI/WCAG22/Understanding/target-size-enhanced.html),
  [WCAG focus appearance](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html),
  [WCAG focus not obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html),
  [MDN `:focus-visible`](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Selectors/:focus-visible),
  [web.dev tap targets](https://web.dev/articles/accessible-tap-targets).

## Practical Direction

- Source imports may use the `@/*` alias for `web/src/*`. Cross-slice imports should prefer
  `@/components`, `@/lib`, and other stable source roots over multi-level `../../` paths so
  feature moves do not create avoidable import churn.
- Keep global primitives in `web/src/styles/tokens.css`; feature CSS can define local semantic
  tokens when the values are not yet shared.
- Preserve mobile-first behavior and add larger layouts with `min-width` media queries or
  container queries.
- Do not hide critical bookkeeping controls on phones. Adapt density, scrolling, or disclosure
  while keeping the workflow complete.
- Keep all visible UI copy in i18n bundles and verify locale shape with `pnpm --dir web run check:i18n`.

## Engineering Gates

- The frontend TypeScript build runs through `pnpm --dir web run build`, which starts with
  `tsc -b`. The frontend keeps `strict`, `noUncheckedIndexedAccess`, `noUnusedLocals`,
  `noUnusedParameters`, `noImplicitReturns`, and `noFallthroughCasesInSwitch` enabled.
- Frontend linting is type-aware through `typescript-eslint` and includes import hygiene through
  `eslint-plugin-import-x`. Current lint accepts the existing React effect warnings while Phase 3
  decomposes shell state, but errors must stay at zero.
- Prettier is the deterministic formatter for `web/`; `pnpm --dir web run format:check` must pass
  before merging. Stylelint parses every source CSS file through the standard config. Existing
  camelCase class names, selector ordering, and OKLCH hue notation are documented temporary debt in
  `stylelint.config.mjs` until the Phase 4 token/layer/class-prefix migration removes those waivers.
- Knip runs through `pnpm --dir web run lint:dead` to detect unused source files, exports, and
  dependencies.
- OpenAPI generated types run through `pnpm --dir web run gen:api` from `docs/api/openapi.yaml`
  and `docs/api/schemas.yaml`. `pnpm --dir web run check:api` must pass before merging so
  `web/src/lib/api/generated/schema.d.ts` cannot drift from the contract.
- `pnpm --dir web run lint`, `pnpm --dir web run check:i18n`, `pnpm --dir web run test`,
  `pnpm --dir web run format:check`, `pnpm --dir web run lint:css`,
  `pnpm --dir web run lint:dead`, `pnpm --dir web run check:api`, `pnpm --dir web run build`, and
  `pnpm --dir web run test:e2e` are the frontend gates that local development and CI should use.

## API Client

- `web/src/lib/apiClient.ts` owns application `fetch` calls, the browser same-origin credentials
  default, JSON request encoding, 204 handling, API error parsing, and `X-Request-ID` extraction.
- Domain modules under `web/src/lib/api/` stay as thin URL/operation wrappers. Their exported DTO
  aliases should come from `components["schemas"]` in `web/src/lib/api/generated/schema.d.ts`, not
  hand-written response shapes.
- User-facing API failure copy goes through `web/src/lib/apiErrorMessage.ts`, which maps stable
  `ApiError.code` values to i18n keys and falls back to feature-specific localized messages.
- The only allowed direct frontend `fetch(` call for application APIs is in `apiClient.ts`.
  Future telemetry work may add a separate `sendBeacon`/telemetry path with its own sanitized
  payload rules.

## Server State

- TanStack Query is installed as the server-state owner and is provided at the app root through
  `QueryClientProvider` in `web/src/main.tsx`.
- `web/src/lib/queryClient.ts` defines shared defaults: queries stay fresh for 30 seconds, window
  focus does not refetch bookkeeping screens, transient query failures retry at most twice, 4xx
  `ApiError` responses do not retry, and mutations do not retry automatically.
- Query keys live in `web/src/hooks/queryKeys.ts`. Keys must include the data owner and relevant
  scope: user/session keys for account-wide data, book IDs for book-owned data, and explicit page,
  size, filter, account, or search parameters when they affect the response.
- Mutations must invalidate through the same `queryKeys` factories instead of duplicating key arrays
  inline. Entry mutations should invalidate ledger summary, affected book entries, accounts, and
  report inputs until Phase 3 introduces narrower cache updates.
- The Me profile activity feed is loaded on demand by `useAuditEventsQuery` with
  `queryKeys.audit.list({ page: 1, pageSize: 20 })`; `MobileWorkspace` must not own audit event
  arrays or activity loading state.
- Passkey metadata is loaded by `usePasskeysQuery` with `queryKeys.auth.passkeys({ page: 1,
  pageSize: 20 })`. Passkey registration, rename, and delete flows update that Query cache through
  the shared key instead of keeping a parallel component-owned passkey list.
- TOTP status is loaded by `useTotpStatusQuery` with `queryKeys.auth.totpStatus()`. Setup remains
  local pending UI state, while confirm and disable mutations write the returned status into the
  same Query cache.
- Authenticated shell client state should move into narrow contexts as it leaves `MobileWorkspace`.
  `web/src/contexts/ThemeContext.tsx` owns the persisted `system`/`light`/`dark` theme preference and
  is composed by `MobileShellLayout`; feature components should consume it through `useThemeContext`
  instead of reading local storage directly.

## Routing

- Public, authentication, and authenticated top-level routes are declared in `web/src/App.tsx` with
  React Router `<Routes>`/`<Route>`. Authenticated users visiting `/` redirect to `/home`; signed-out
  users opening protected routes redirect to `/login` with the intended return path in route state.
- The canonical authenticated route set is `/home`, `/accounts`, `/accounts/:accountId/transactions`,
  `/record`, `/reports/:dimension`, `/imports`, `/me`, `/me/profile`, `/me/security`, and
  `/entries/:entryId`. `/reports` redirects to `/reports/category`; `/search?query=...` opens the
  shareable transaction search route.
- `web/src/features/shell/RequireAuth.tsx` owns protected-route redirects. `MobileWorkspace` receives
  route state from `App.tsx`; production routing must use React Router params and search params, not
  `mobileTabFromPath`-style pathname parsers.
- Search query state lives in `URLSearchParams` on `/search`; typing replaces the current search URL
  so the result can be refreshed or shared without replaying local component state. Search entry
  loading is owned by `useMobileSearchEntries`, which uses TanStack Query and
  `queryKeys.entries.list` instead of shell-owned `useState`.
- Account-detail transaction loading is owned by `useAccountDetailEntries`, keyed through
  `queryKeys.entries.list(bookId, { accountId, page: 1, pageSize: 100, revision })`. The mobile
  shell must not keep account-detail entry arrays or loading state.
- Direct-open entry detail fallback loading is owned by `useEntryDetail`, keyed through
  `queryKeys.entries.detail(bookId, entryId, revision)`. The shell may keep a short-lived edited
  entry draft for mutation feedback, but it must not fetch all entries to hydrate detail routes.
- Route panels are lazy-loaded behind `Suspense` in `MobileWorkspaceContent`: home, accounts, account
  transactions, entry detail, record, reports, imports, me/settings, and transaction search. The
  measured local Vite build after this split produced a main JS asset of 374.08 kB, down from the
  pre-split 461.55 kB measurement, or about 18.9%. The original 25% target is deferred until Phase 3
  moves server state and mutation code out of the monolithic mobile shell; route-panel splitting alone
  cannot remove shared shell/API/state code from the initial bundle.
