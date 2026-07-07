# User-Facing UI/UX Overhaul — Development Handbook

- **Status:** Ready for implementation. This document rewrites the architect's direction-setting
  proposal into an executable, code-grounded manual with concrete change lists and an acceptance
  matrix. A developer can pick up any workstream (`W*`) or page (`P*`) below and ship it without
  further design negotiation; a reviewer can accept it against the stated gates.
- **Date:** 2026-07-06 (direction) · rewritten 2026-07-07 against the post-Phase-7 code tree.
- **Product stage:** Pre-launch. Breaking refactors are allowed. **No backward-compatibility
  shims** — do not keep legacy class names, dual theme mechanisms, or parallel palettes alive.
  Every migration is a clean cutover: when a page moves to the new system, its old CSS and
  hand-rolled components are deleted in the same PR.
- **Scope:** every user-visible surface under `web/` — the public pages (Landing, the full Auth
  flow) and every authenticated view (Home, Accounts, Account Transactions, Record, Reports,
  Imports, Me/Security, Search, Entry Detail) plus the application shell (navigation, header,
  notices).
- **Explicitly out of scope:** backend API and domain model; the i18n architecture (adding or
  editing copy keys is allowed); the React/Vite stack; introducing a third-party component library
  or charting library (**charts stay hand-written SVG**).
- **Related docs:**
  - `docs/proposals/archives/2026-07-06-architecture-overhaul.md` — the frontend architecture
    overhaul (Phases 0–7), **implemented and archived**. Its Phase 4 ("Design System Foundation")
    delivered the *scaffolding* this handbook builds on; see §1.1 for exactly what landed and what
    it deferred. This handbook owns the deferred visual/UX work.
  - `docs/arch/frontend.md` — the 2026 UI/CSS baseline and engineering gates. Item AC-11 requires
    this file to be updated to describe the end state.
  - `docs/arch/i18n.md` — five-language contract (`en` canonical, `zh`/`fr`/`es`/`ja`); the
    `check:i18n` gate enforces identical key sets and line counts across locales.
  - `docs/arch/landing.md` — landing-page product research (informs `P1`).

---

## 1. Background

### 1.1 What Phase 4 already delivered, and what it deferred

The architecture overhaul landed the *engineering scaffolding* for a design system. **Do not
rebuild these** — adopt them:

| Delivered (as-built) | Evidence |
| --- | --- |
| Cascade-layer order `@layer tokens, base, components, features` | `web/src/styles/layers.css` |
| Semantic token layer + light/dark under `:root[data-theme="dark"]` | `web/src/styles/tokens.css:14-119` |
| `ThemeContext` writes `data-theme` on `<html>`, persists `system`/`light`/`dark` | `web/src/contexts/ThemeContext.tsx:25-38` |
| Color-tokenization script + CI gates (`check:tokens`, `lint:css`, stylelint `function-disallowed-list` + `color-no-hex`) | `web/scripts/tokenize-colors.mjs`, `web/stylelint.config.mjs`, `.github/workflows/web.yml` |
| Seven tested UI primitives | `web/src/components/ui/{Button,Card,ConfirmDialog,EmptyState,FormField,Notice,Sheet}.tsx` + `ui.test.tsx` |
| Router-based shell decomposition (layout route + lazy route bodies + `useShellChrome`) | `web/src/features/shell/`, `web/src/App.tsx` |
| Responsive shell that reflows to a rail (≥768px) and a labelled sidebar (≥1024px) | `web/src/features/mobile/mobile-navigation.css:101-221` |

Phase 4 **explicitly deferred** the substance of the visual overhaul (archived proposal,
Phase-status notes): the full `@layer` migration of feature CSS, *adoption* of the primitives into
features, retiring the `.mobileShellThemeDark` theming track, repointing feature CSS onto the
semantic tokens, typography convergence, the desktop workspace, motion, chart interaction, the
Landing/Auth redesign, and tri-state coverage. **That deferred work is this handbook.**

### 1.2 Current-state diagnostic (as-built, with evidence)

The original proposal's `U1–U10` table described the *pre-overhaul* tree. The table below is the
accurate as-built state on the 2026-07-07 tree; each row is the real starting point for a
workstream. Metrics are reproducible with the commands in §5.

| ID | Finding | Evidence & metric | Owning workstream |
| --- | --- | --- | --- |
| **C0** | **Primitive palette is committed empty.** `styles/palette.css` ships an empty `:root {}` while **647** `var(--tone-*)` references (across **312** distinct names) exist in `src` CSS — including the semantic aliases in `tokens.css`. `check:tokens` does not catch this (it only greps for residual `oklch(` literals; see `tokenize-colors.mjs:58-66`). No build step regenerates the file (`build` = `tsc -b && vite build`). Every `var(--tone-*)` currently resolves to an invalid value, so colors fall back to inherited/initial. Both `frontend.md:176-180` and the archived proposal claim this file "holds every primitive"/"312 primitive tokens" — a doc-vs-reality regression. | `git show HEAD:web/src/styles/palette.css`; `grep -rho "var(--tone-" web/src --include='*.css' \| wc -l` → 647 | **S0 pre-flight**, then `W1` |
| **C1** | **No designed token scale.** The 312 `--tone-*` names are a *mechanical* 1:1 lift of every distinct old `oklch()` literal (`tokenize-colors.mjs:31-41`), not a curated ramp. Feature CSS references machine primitives, **not** the semantic layer: semantic-role usage across `src/features` + `src/components` totals ~50 refs (`--surface`×10, `--ink`×14, `--border`×6, `--accent`×3, …) versus 647 machine refs. | grep counts in §5 | `W1` |
| **C2** | **Theming is dual-track and incomplete.** `data-theme` exists but `MobileShell.tsx:94` still stamps `mobileShellTheme{Dark,Light}` on `<main>`; legacy `.mobileShellThemeDark` rules live in `mobile-shell.css`, `record-entry.css`, `record-category-sheet.css`; `mobile-navigation.css:223` themes via `@media (prefers-color-scheme: dark)`. `color-scheme` is never set on the document. Public routes (`/`, auth) are **not** wrapped in `ThemeProvider`, so `data-theme` is never written there. Views with **zero** dark rules of any kind: Accounts, Account Transactions, Reports, Imports, Me/Security, Search (bound to non-flipping `--tone-*`). | `MobileShell.tsx:94`; `app.workspace.test.tsx:57` asserts the legacy class | `W1` |
| **C3** | **Typography is unconverged and ultra-heavy.** `font-weight` declarations span 620–950 (peaks: 950 in `landing.css`, 930 in `reports.css`/`mobile-shell.css`, 920 in `account-transactions.css`/`entry-detail.css`). The `--font-weight-*` tokens (400/500/600/700) are used only by `ui.css`. | `grep -rhoE "font-weight:\s*[0-9]+" web/src --include='*.css' \| sort \| uniq -c` | `W2` |
| **C4** | **UI primitives exist but adoption is ~2%.** Only three feature import sites: `EntryDetailEditor` (`ConfirmDialog`), `MobileShell` (`Sheet`), `HomeView` (`EmptyState`). `Button`/`Card`/`FormField`/`Notice` have **zero** feature usage. ~73 raw `<button>` with bespoke classes; every form is hand-rolled `<label><input>`; ~13 hand-rolled empty-state blocks. Two hand-rolled overlays (`CategorySheet` `RecordEntryView.tsx:484`, `WorkspaceMenu` `MobileWorkspaceHeader.tsx:211`) reimplement `Sheet` without its focus trap/Escape/return-focus. | see `W3` change list | `W3` |
| **C5** | **Missing primitives.** No `Select`, `Input`, `Skeleton`, `Toast`, `SegmentedControl`, `Tag`, or `AmountText`. Money formatting is centralized in `lib/money.ts` (`formatMoney`) but never componentized (~35 inline call sites); `font-variant-numeric: tabular-nums` is re-declared in **8** stylesheets instead of one utility. | see `W2`/`W3` | `W2`, `W3` |
| **C6** | **Desktop is still the phone frame, restyled.** The shell reflows to rail/sidebar, but the element is literally `.phoneFrame` (`MobileShell.tsx:130`, `mobile-shell.css:70`); the rounded 430px mockup is still visible at 521–767px (`mobile-shell.css:79-84`). Desktop header carries no breadcrumb/search/quick-record; the sidebar carries no BookSwitcher/UserCard. Home and Account Transactions have **no** desktop layout at all; Accounts' "2-pane" is controls+list, not master-detail. | `mobile-navigation.css:101-221`; `mobile-home.css` has no `@media`/`@container` | `W4` |
| **C7** | **No motion system.** Zero motion/duration/easing tokens. ~14 hand-written `transition:` rules cluster inconsistently at 120/140/160ms/0.15s; one keyframe (`Spinner`); `prefers-reduced-motion` handled in 6 files with three different (partly vacuous) patterns. | `grep -rn "prefers-reduced-motion" web/src --include='*.css'` → 6 files | `W5` |
| **C8** | **Chart interaction is uneven and colors are hardcoded.** `TrendChart` (`ReportVisuals.tsx:327`) and the balance sparkline (`AccountTransactionsView.tsx:259`) nearly meet the standard; the **donut** and **ranked-bar** fail scrub/tooltip/keyboard/reduced-motion and paint from `reportColors.ts` hex applied inline (`ReportVisuals.tsx:208,242,296,305`). Scrub/tooltip/keyboard logic is duplicated across the two files with no shared module. | `reportColors.ts:1-10` (8 hex); duplicated `indexFromClientX` in both files | `W6` |
| **C9** | **Landing is a static faked mockup.** `LandingPage.tsx:63-90` hand-builds a `<div>` "ledger window" from copy; there is no real component render, no motion, no dark theme, no theme control; CSS uses raw `--tone-*` + local `--landing-*` aliases. | `LandingPage.tsx:63-90`; `landing.css` has no `[data-theme]`/`prefers-color-scheme` | `P1` |
| **C10** | **Tri-state is thin.** No `Skeleton` anywhere; loading is plain text/inline `Spinner`/nothing (Home/Accounts/Record render empty data during fetch via `?? []`). Exactly one guided `EmptyState` (Home); ~13 others are plain text. Only one retryable error affordance (`MeView.tsx:110`). Auth errors are not announced (no `aria-live`/`role="alert"`; `AuthWorkspace.tsx:379-380`). Destructive security actions (delete passkey, disable TOTP) have **no** confirmation at all (`PasskeySettingsView.tsx:177`, `TotpSettingsView.tsx:163`). | see `P12`, `P2`, `P9` | `P12` (+ per page) |

### 1.3 Goals (unchanged in intent, restated as outcomes)

1. A single **light/precise/trustworthy** design language: generous whitespace, restrained color,
   clear hierarchy, soft depth, precise numeric typography (`tabular-nums`, aligned, grouped).
2. **One semantic-token-driven light/dark system** covering every route and component — including
   the public pages.
3. **A real multi-column desktop workspace**, not an enlarged phone; phone keeps the thumb-reachable
   bottom navigation.
4. **Every user-visible chart meets the interaction standard** (pointer+touch scrub, follow
   tooltip, keyboard, ARIA), continuing the hand-written SVG approach.
5. **WCAG 2.2 AA** is the launch floor; motion respects `prefers-reduced-motion`.
6. A **product-grade Landing/Boarding** first impression: a realistic live product demo, clear
   value proposition, dual theme, five languages without layout breakage.

### 1.4 Design principles (binding)

1. **Tokens first.** Any color, size, radius, shadow, or motion duration comes from a token; a raw
   literal is a defect. Feature CSS references the **semantic** layer (`--surface`, `--ink`, …), not
   machine `--tone-*` primitives.
2. **Adapt density, never remove capability.** Phones must not hide bookkeeping features
   (`frontend.md` rule).
3. **Build only the primitives the current flows need** — not a general component library.
4. **Motion serves causal understanding** (where things came from, went to, what happened), not
   decoration.
5. **Money and destructive actions are confirmable, reversible, and reviewable** (WCAG 2.2 error
   prevention).
6. **Clean cutover.** New and old mechanisms never coexist; migrating a page deletes that page's
   old styles and hand-rolled components in the same PR.

---

## 2. Design language specification

### 2.1 Token system (the source of truth for `W1`)

Three layers, `@layer tokens`, split across `styles/palette.css` (primitives) and
`styles/tokens.css` (semantic + component):

```
primitive            →  semantic                 →  component (only where needed)
--neutral-0..1000        --surface / -raised          --button-primary-bg
--brand-...              --ink / -muted / -subtle     --nav-item-active-fg
--chart-1..10 (L & D)    --border / -subtle / -strong
                         --accent / -danger / -success / -warning
                         --focus / --focus-ring
```

Concrete requirements (exact L/C/H values are the designer's call within these constraints, then
frozen as the reviewed baseline):

- **Color:** all OKLCH. Ship a *designed* scale, replacing the machine lift: a neutral ramp of
  10–12 steps, one brand family, semantic families (income/expense/transfer/danger/warning/success),
  and a chart categorical set of 8–10 hues **authored in both light and dark**. This chart set
  replaces `reportColors.ts` (`W6`).
- **Light/dark:** the semantic layer flips once under `:root[data-theme="dark"]` (already
  scaffolded in `tokens.css:98-118`); component CSS references only the semantic layer. Set
  `color-scheme: light dark` (and the resolved value per theme) so native form controls,
  scrollbars, and `::selection` theme correctly. Delete `.mobileShellThemeDark/Light` and every
  scattered `@media (prefers-color-scheme)` color rule.
- **Type:** keep the system font stack; a ~1.2 modular scale (≈12/13/15/18/22/28/34, already present
  as `--font-size-*`). **Weights converge to 400/500/600/700 only; nothing above 700.** Money is
  always `tabular-nums`.
- **Spacing/shape:** 4px base (`--space-*` present); radii 4/8/12/16/pill (present); three shadow
  levels (present), low-saturation/low-spread.
- **Motion (new, `W5`):** `--motion-fast: 120ms`, `--motion-base: 200ms`, `--motion-slow: 320ms`;
  `--ease-standard` and `--ease-exit`. Under `prefers-reduced-motion: reduce`, all motion degrades
  to an opacity change or an instant cut.

### 2.2 Layout DSL

Core layouts are described with this notation; developers keep the same notation when expanding a
page into its component tree.

```
syntax:
  page <name> { viewport <breakpoint>: <tree> }
  containers: stack(vertical) | row(horizontal) | grid(cols:[...]) | scroll
  modifiers:  [sticky] [fixed] [collapsible] [max:<width>] [gap:<step>]
  leaves:     ComponentName(args)  ;  fab = floating primary action
breakpoints (match code): phone <768 | tablet 768–1023 | desktop ≥1024
```

---

## 3. Change list

Two layers: horizontal **workstreams `W1–W6`** (infrastructure, first) and vertical **page
migrations `P1–P12`** (depend on the `W` layer). Each item states **Current → Changes →
Acceptance**. File paths are relative to `web/`.

### W1 — Design tokens & unified theming

**Current:** semantic layer + gates scaffolded (`§1.1`); primitive palette empty (`C0`); feature
CSS on machine tones (`C1`); dual-track theming (`C2`).

**Changes:**

| # | File(s) | Change |
| --- | --- | --- |
| W1.1 | `styles/palette.css` | Replace the generated machine palette with the **designed** primitive scale from §2.1 (neutral ramp, brand, semantic, chart ×10, light+dark). Hand-authored and reviewed; delete the "GENERATED — do not edit" header. |
| W1.2 | `styles/tokens.css` | Repoint every semantic role onto the new primitives; keep the light values and the `:root[data-theme="dark"]` block; add `color-scheme` (see W1.4) and the motion tokens (`W5`). |
| W1.3 | all feature/base CSS (`src/features/**`, `src/components/**`, `src/styles.css`) | Repoint every `var(--tone-*)` reference to the correct **semantic** token; delete the machine reference. This is done per page under `S3`, deleting each file's machine tones as the page migrates. |
| W1.4 | `styles/tokens.css`, `contexts/ThemeContext.tsx` | Set `color-scheme` on `:root` and per theme; have `ThemeContext` also set `document.documentElement.style.colorScheme` (or rely on the CSS property). |
| W1.5 | `features/shell/MobileShell.tsx:94,128`; `mobile-shell.css`, `record-entry.css:481-512`, `record-category-sheet.css:214`, `mobile-navigation.css:223` | **Delete the `.mobileShellThemeDark/Light` track entirely** and the `@media (prefers-color-scheme: dark)` color block. Theme comes only from `data-theme`. |
| W1.6 | `app.workspace.test.tsx:57` | Rewrite the assertion (it currently asserts `mobileShellThemeDark`); assert `data-theme="dark"` on `<html>` instead. |
| W1.7 | `scripts/tokenize-colors.mjs`, `package.json` | Retire the literal→`--tone-*` **rewrite** role. Keep/extend `check:tokens --check` to fail if (a) any raw `oklch(`/hex remains in feature CSS **and** (b) any referenced `var(--tone-*)` has no definition in `palette.css` (closes the `C0` gap). Once machine refs reach 0, remove the generator. |
| W1.8 | `App.tsx` (public routes), `features/auth/*`, `features/landing/*` | Ensure `data-theme` is applied on public routes too (mount a lightweight theme signal above `/` and the auth routes, or lift `ThemeProvider`), so Landing/Auth theme correctly (`P1`/`P2`). |

**Acceptance (W1):**
- `grep -REn "oklch\(|#[0-9a-fA-F]{3,8}|rgba?\(" web/src --include='*.css' | grep -v '/styles/palette.css'` → **0**.
- `grep -rn "var(--tone-" web/src --include='*.css'` → **0** (all feature CSS on semantic tokens);
  `palette.css` is non-empty and hand-authored.
- `grep -rn "mobileShellTheme\|prefers-color-scheme" web/src --include='*.css' --include='*.tsx'` →
  only the `ThemeContext` `matchMedia('(prefers-color-scheme: dark)')` read for `system` mode.
- `check:tokens` fails on a deliberately undefined `--tone-*`/empty palette (regression test for `C0`).
- Every route renders correctly in light and dark; native controls adopt the theme (`color-scheme`).

### W2 — Typography & numerics

**Current:** weights 620–950 (`C3`); `tabular-nums` duplicated in 8 files (`C5`); money formatted
in `lib/money.ts` but not componentized.

**Changes:**

| # | File(s) | Change |
| --- | --- | --- |
| W2.1 | all feature CSS | Map every `font-weight` onto 400/500/600/700 (`--font-weight-*`). No literal weights; none above 700. |
| W2.2 | `components/ui/AmountText.tsx` (new), `styles/tokens.css` `.tabularNums` | Add an `AmountText` primitive (see `W3`) that owns money markup, sign/positive-negative color, grouping, and `tabular-nums`. Remove the 8 ad-hoc `font-variant-numeric` declarations in favor of it / the utility. |
| W2.3 | icons (`lucide-react`) | Standardize on two icon sizes and one stroke width via tokens; audit stray sizes. |

**Acceptance (W2):**
- `grep -rhoE "font-weight:\s*[0-9]+" web/src --include='*.css'` yields only `400/500/600/700` (or
  the `--font-weight-*` vars). Add a stylelint gate: disallow `font-weight` numeric > 700.
- Every rendered amount uses `AmountText`; `tabular-nums` is declared in exactly one place.

### W3 — Shared UI primitives (`components/ui/`)

**Current:** 7 primitives exist, ~2% adoption (`C4`); 7 primitives missing (`C5`).

**Changes — build the missing primitives (only what current flows need):**

| # | New file | Notes |
| --- | --- | --- |
| W3.1 | `Select.tsx` | Token-styled native `<select>` wrapper with `FormField` wiring; replaces raw selects in Record/Accounts/Me/Imports/EntryDetailEditor + `LanguageSelector`. |
| W3.2 | `Input.tsx` / extend `FormField` | Token-styled text/number input honoring `FormField`'s `id`/`describedBy`/`invalid`. |
| W3.3 | `SegmentedControl.tsx` | Accessible segmented/tab primitive (roving tabindex, arrow keys, `aria-selected`). Adopt in Record type switch, Reports flow/time filters, Auth tabs. |
| W3.4 | `Skeleton.tsx` | Shimmer/placeholder blocks; used by every view's loading state and the shell Suspense fallback (`P12`). |
| W3.5 | `Toast.tsx` | Fold `NoticeContext` status/error/undo into a token-styled, `aria-live` toast; replaces the ad-hoc `.mobileNotice` markup in the shell. |
| W3.6 | `Tag.tsx` | Entry/category tag chip; used by EntryDetail, Record category chips. |
| W3.7 | `AmountText.tsx` | See `W2.2`. |

**Changes — adopt the existing primitives and delete duplicates (per page under `S3`):**

| # | Replace hand-rolled … | With | Sites (examples) |
| --- | --- | --- | --- |
| W3.8 | `.mobilePrimaryButton/.mobileSecondaryButton/.mobileDangerButton/.primaryButton/.ghostButton` and ~73 raw `<button>` | `<Button variant=…>` | `AuthWorkspace.tsx:382`, `EntryDetailEditor.tsx:90,207`, `MeView.tsx:107`, `TotpSettingsView.tsx:163`, `PasskeySettingsView.tsx:178`, `ImportPreviewView.tsx:252`, `AccountsView.tsx:303`, `CategoryManager.tsx:173`, … |
| W3.9 | `CategorySheet` (`RecordEntryView.tsx:484`), `WorkspaceMenu` (`MobileWorkspaceHeader.tsx:211`) | `<Sheet>` (which provides focus trap/Escape/return-focus) | delete `record-category-sheet.css` overlay + the bespoke popover |
| W3.10 | raw `<label><span/><input/select>` forms | `<FormField>` + `Select`/`Input` | `AuthWorkspace.tsx:283-357`, `EntryDetailEditor.tsx:116-201`, `RecordEntryView`, `AccountsView`, `MeView`, `Imports`, `Search` |
| W3.11 | ~13 `.emptyState`/`.meEmpty`/`.reportEmpty`/`.passkeyEmpty` text blocks | `<EmptyState>` (with guided action where one exists) | `AccountTransactionsView.tsx:186`, `TransactionSearchView.tsx:99`, `ReportVisuals.tsx:276`, `MeView.tsx:85,126`, `PasskeySettingsView.tsx:190`, … |

All primitives must satisfy: 44px touch target, `:focus-visible` ring token, keyboard operability,
complete ARIA. Each new primitive ships with a unit test in `ui.test.tsx` and light/dark coverage.

**Acceptance (W3):**
- `grep -rn "from '@/components/ui'" web/src/features` shows every migrated view importing the
  relevant primitive; `Button`/`Card`/`FormField`/`Notice` have real feature usage.
- No feature file defines a `role="dialog"` overlay by hand; `grep -rn "categorySheetOverlay\|workspaceMenu " web/src` → 0.
- `mobile*Button`, `primaryButton`, `ghostButton`, `secondaryAuthButton` classes are deleted.

### W4 — Application shell: adaptive workspace, not a phone frame

**Current:** shell reflows to rail/sidebar but is literally `.phoneFrame` with a 430px rounded
mockup at 521–767px; minimal desktop chrome (`C6`).

**Target shell:**

```
page AppShell {
  viewport phone:
    stack {
      header[sticky] { BookSwitcher SearchTrigger OverflowMenu }
      scroll { <ActiveView> }
      tabbar[fixed] { Home Accounts fab:Record Reports Me }
    }
  viewport tablet:
    grid(cols:[76px,1fr]) {
      rail { Brand NavIcons ThemeToggle }
      stack { header[sticky] scroll{ <ActiveView> } }
    }
  viewport desktop:
    grid(cols:[248px,minmax(0,1fr)]) {
      sidebar { Brand Nav BookSwitcher UserCard }
      stack { header[sticky]{ Breadcrumb Search QuickRecord }
              scroll[max:1440px] { <ActiveView multi-column> } }
    }
}
```

**Changes:**

| # | File(s) | Change |
| --- | --- | --- |
| W4.1 | `MobileShell.tsx:128-130`, `mobile-shell.css:70-87`, `mobile-navigation.css` | Rename/replace `.phoneFrame` with a semantic shell root; **delete the 430px rounded-mockup style**. Keep the full-bleed phone form (≤767px) and the rail (≥768) / sidebar (≥1024) reflow, rebuilt on real `Sidebar`/`Rail` structure, not `.bottomNav`-as-rail. |
| W4.2 | desktop header | Add a persistent **Quick Record** entry (opens Record's desktop form), Search, and a breadcrumb — recovering the primary action the phone FAB provides. |
| W4.3 | desktop sidebar | Add `BookSwitcher` and a `UserCard` (currently only in the phone header/Me). |
| W4.4 | notices | Route the shell notice area through the `Toast` primitive (`W3.5`); drop the dedicated `notice` grid area. |
| W4.5 | all shell CSS | Repoint to semantic tokens; converge weights (`navBrand` is 930, `mobile-shell.css:174`). |

**Coordinate with the archived overhaul's shell decomposition** — this is a rename/reframe of an
existing structure, not greenfield.

**Acceptance (W4):**
- No `.phoneFrame` selector remains; `grep -rn "phoneFrame" web/src` → 0.
- ≤767px: full-bleed, bottom tab bar + FAB. 768–1023px: 76px rail. ≥1024px: 248px sidebar with
  BookSwitcher + UserCard, top bar with Breadcrumb + Search + Quick Record.
- No rounded phone mockup at any width; three-breakpoint screenshots updated.

### W5 — Motion system

**Current:** no tokens; inconsistent hand-written transitions; ad-hoc reduced-motion (`C7`).

**Changes:**

| # | File(s) | Change |
| --- | --- | --- |
| W5.1 | `styles/tokens.css` | Add duration/easing tokens (§2.1). |
| W5.2 | all CSS with `transition:`/`animation:` | Replace literal durations/easings with tokens; define four standard motions: page transition (subtle fade+shift), overlay (bottom-sheet spring / dialog scale), list add/remove (height+opacity), value change (amount roll). |
| W5.3 | key micro-interactions | Record success confirmation, tab-indicator slide, chart crosshair follow. |
| W5.4 | a global reduced-motion rule | One consistent `@media (prefers-reduced-motion: reduce)` policy; remove the vacuous/partial per-file rules (`reports.css:585` targets a non-animated selector; `C7`). |

**Acceptance (W5):**
- `grep -rnE "transition:|animation:" web/src --include='*.css'` shows only token-based
  durations/easings.
- With reduced-motion forced, no positional/scale animation occurs anywhere.

### W6 — Chart interaction standard (hand-written SVG)

**Current:** `TrendChart` + sparkline near-standard; donut/ranked-bar fail; hardcoded hex; duplicated
scrub logic (`C8`).

**Changes:**

| # | File(s) | Change |
| --- | --- | --- |
| W6.1 | `features/charts/` (new shared module) | Extract the duplicated scrub/tooltip/keyboard/axis logic (currently copied in `AccountTransactionsView.tsx:311-387` and `ReportVisuals.tsx:365-433`) into one hook/utility (`useScrub`, tooltip + axis helpers). Still hand-written SVG, no library. |
| W6.2 | `reportColors.ts` → chart tokens | Delete the 8-hex array; source segment/bar/line colors from the `--chart-*` tokens (light+dark). Remove inline `style={{ background: … }}` / `stroke={…}` color application (`ReportVisuals.tsx:208,242,296,305`). |
| W6.3 | `DonutChart` (`ReportVisuals.tsx:164`) | Add pointer+touch scrub across the arc, a follow tooltip, keyboard stepping through all segments with focus, and reduced-motion coverage for `.donutSegment` (`reports.css:255`). |
| W6.4 | `RankedList` (`ReportVisuals.tsx:260`) | Make rows focusable with keyboard stepping + tooltip/summary; add `role="img"`+ARIA summary; reduced-motion for `.rankedList li` (`reports.css:310`). |
| W6.5 | sparkline (`AccountTransactionsView.tsx:259`) | Replace the `aria-hidden` SVG + hardcoded gradient stops (`:394-395`) with a `role="img"`+summary and token-driven gradient. |
| W6.6 | `TrendChart` | Fix the vacuous reduced-motion rule (`reports.css:585` targets `.trendDot`, which has no transition). |

**Acceptance (W6):** every user-visible chart (sparkline, donut, ranked bar, trend) satisfies all
seven criteria — pointer scrub, touch scrub, follow tooltip, keyboard L/R + focus, `role="img"`+ARIA
summary, token colors, reduced-motion degradation. `grep -rn "reportColors" web/src` → 0.

---

### P1 — Landing / Boarding

**Current (`C9`):** sticky header + hero with a **static `<div>` faked ledger mockup**
(`LandingPage.tsx:63-90`); proof/workflow/trust/model sections; light-only; no theme control; raw
`--tone-*` + `--landing-*` aliases; weights to 950.

```
page Landing {
  viewport phone:
    stack {
      header[sticky] { Logo row{ LanguageSelector ThemeToggle SignIn } }
      scroll {
        Hero { Headline Subline row{ CTA:Register CTA2:View demo }
               LiveProductDemo(auto-play; real components rendering a record→report micro-script) }
        ValueProps(grid:1col, 3–4 cards)
        FeatureTour(alternating text/visual, phone+desktop forms)
        TrustAndDataOwnership
        FinalCTA  Footer
      }
    }
  viewport desktop:
    same structure; Hero → grid(cols:[5fr,7fr]) text-left/demo-right; ValueProps 3col; FeatureTour alternating
}
```

**Changes:** replace the faked mockup with a `LiveProductDemo` that renders **real** components with
seed data, looping "record an entry → categorize → report updates" (static first frame under
reduced-motion); add a `ThemeToggle` and apply `data-theme` on `/` (`W1.8`); migrate `landing.css`
to semantic tokens + `Button`; converge weights; verify five-language layout.

**Acceptance:** dual-theme, five-language, three-breakpoint screenshots pass; live demo replaces the
static mockup and degrades to a static frame under reduced-motion; Lighthouse targets in §5 met;
`landing.css` has zero `--tone-*` and zero `font-weight > 700`.

### P2 — Auth (login / TOTP / register / verify / recover)

**Current:** two-column hero+card; all five flows + passkey + SSO present; light-only; not under
`ThemeProvider`; hand-rolled forms (no `Button`/`FormField`); **errors not announced**
(`AuthWorkspace.tsx:379-380`); tabs are `role="tablist"` but buttons lack `role="tab"`/`aria-selected`;
Turnstile `aria-label` hardcoded English (`TurnstileWidget.tsx:85`).

```
page Auth {
  viewport desktop: grid(cols:[5fr,7fr]) { BrandPanel(atmosphere + one-line value)
                                            stack[max:420px]{ StepIndicator? FormCard AltActions } }
  viewport phone:   stack[max:420px]{ Logo FormCard AltActions } }
```

**Changes:** adopt `FormField`/`Input`/`Button`/`SegmentedControl` (tabs); announce errors via
`role="alert"`/`aria-live` (or `FormField`'s built-in wiring); apply `data-theme` (`W1.8`) and
migrate `auth.css` + the `.authError/.authStatus` pills (`styles.css:50-66`) to semantic tokens;
add `role="tab"`/`aria-selected`; i18n the Turnstile label and give it a loading placeholder.

**Acceptance:** dual-theme; axe 0 serious/critical; keyboard-only completion of sign-in→TOTP and
register→verify; errors announced by a screen reader; `auth.css` on semantic tokens.

### P3 — Home

**Current:** only a synthetic budget card + day-grouped recent entries; **no net-summary card, no
quick actions** (the `.quickRecord` CSS is dead), **no desktop layout** (`mobile-home.css` has no
`@media`/`@container`); uses the one real `EmptyState`.

```
page Home {
  viewport phone: stack[gap:m] {
    GreetingRow(date + book)
    NetSummaryCard(month income/expense/net, amount-roll motion, mini trend)
    QuickActions(row: Record / Transfer / Import)
    BudgetCard(progress → ring or segmented bar, overspend warning color)
    RecentEntries(day-grouped, EmptyState: guide first entry)
  }
  viewport desktop: grid(cols:[2fr,1fr]) {
    stack{ NetSummaryCard(30-day trend chart, W6-compliant) RecentEntries }
    stack{ BudgetCard QuickActions UpcomingHints? }
  }
}
```

**Changes:** add `NetSummaryCard` (surface the real net figure, not the synthetic budget proxy at
`HomeView.tsx:46-47`); render `QuickActions`; add the desktop two-column grid; use
`Card`/`AmountText`; day totals to `tabular-nums`.

**Acceptance:** the DSL layout is present at phone and desktop; net/income/expense shown as real
figures; new-user empty state guides "create account → first entry"; W6-compliant trend chart on
desktop.

### P4 — Accounts

**Current:** card-grouped accordions (heuristic grouping), desktop 2-column **controls+list**
(`accounts.css:23-71`) — not master-detail; no per-account trend; rows show opening balance, not a
running balance; **zero dark rules** in `mobile-account.css`.

**Changes:** adopt a real desktop **master-detail** 2-pane (tree left, selected-account detail
right) instead of navigating away; card-grouped tree with token-driven account color dots/icons;
optional per-account mini trend (W6); fold create-account / book rename / currency into `Sheet`;
migrate to semantic tokens (adds full dark support); `Button`/`FormField`/`Select`.

**Acceptance:** desktop shows tree+detail side by side; dark theme correct; no `--tone-*` in
`accounts.css`/`mobile-account.css`; forms/buttons use primitives.

### P5 — Account Transactions

**Current:** polished scrub sparkline + month accordions with subtotals and running balance; **no
desktop layout** (fixed 132px chart, centered at ≤900px); raw `<p>` empty/not-found states; zero
dark rules.

**Changes:** on desktop, promote the sparkline to a page-top banner chart and give the list a
two-column density; strengthen month subtotal rows; apply the W6 shared chart module + tokens;
replace raw `<p>` empties with `EmptyState`; semantic tokens (dark support); weights (currently up
to 920).

**Acceptance:** desktop banner chart present and W6-compliant; dark theme correct; `EmptyState`
used; `account-transactions.css` on semantic tokens, weights ≤700.

### P6 — Record (core capture)

**Current:** genuine desktop 2-pane already exists (`@container appMain (min-width:680px)`,
`record-entry.css:526-601`); **four** entry types (expense/income/transfer/**borrow**) as a
scrolling tab strip; amount on the category card is `tabular-nums`, calculator output is not; top-7
quick-category grid + "All"; save is a keypad key (no separate fixed submit bar); **no
continuous-entry mode**; hand-rolled `CategorySheet` (no focus trap); local `--record-*` aliases on
machine tones; legacy `.mobileShellThemeDark` dark rules.

**Changes:** convert the type strip to `SegmentedControl` (**keep all four types** — see Decision
D1); make the calculator output `tabular-nums` via `AmountText`; add a fixed submit bar with a
**continuous-entry** toggle that stays on the page after save (today `handleSave` clears fields but
`RecordEntryView.tsx:106` navigates to `/home`); move `CategorySheet`/`CategoryManager` onto
`Sheet`; add the 200ms success confirmation (W5); migrate `--record-*` and machine tones to semantic
tokens and delete the legacy dark rules.

**Acceptance:** three-tap common entry; continuous mode does not leave the page; category management
uses `Sheet` (focus trap/Escape/return-focus); dark via `data-theme` only; `record-entry.css` on
semantic tokens, weights ≤700.

### P7 — Reports

**Current:** 7-dimension navigation is an accessible **tab list** (roving tabindex);
`.reportSegmented` flow (`all/expense/income`) and time (`year/month`) filters already exist; desktop
body is a 2-column chart|list at `@container reportView (min-width:720px)` — not a 3-section
overview+chart+list; donut/ranked-bar fail W6; 72 machine tones, no dark, weights to 930.

**Changes:** adopt `SegmentedControl` for the flow/time filters (and reuse it for the dimension tabs'
styling while keeping tab semantics for 7 items — see Decision D2); restructure desktop to
overview-cards row + main chart + detail list; apply W6 to donut/ranked-bar + chart tokens; add
inter-dimension transition motion (W5); semantic tokens (dark) + weight convergence.

**Acceptance:** all charts W6-compliant with token colors; desktop three-section layout; dark theme
correct; `reports.css` on semantic tokens, weights ≤700.

### P8 — Imports

**Current:** 7-stage state machine (`empty|ready|staging|staged|applying|applied|failed`) **not**
visualized as a stepper; desktop is conservative CSS-only (comment confirmed at
`import-preview.css:324-327`), no 2-pane mapping+preview; failure is a single string; empty state has
no action.

**Changes:** visualize the state machine as a **stepper**; build the desktop **2-pane** mapping-config
+ preview split (resolving the deferred follow-up); make the failure state an **actionable error
list** (surface per-row diagnostics that today only appear in the staged table); `EmptyState` with a
guided action; `Button`/`FormField`/`Select`; semantic tokens (dark).

**Acceptance:** stage progression shown as a stepper; desktop 2-pane; failures list actionable rows;
dark theme correct; semantic tokens, weights ≤700.

### P9 — Me / Security (Passkey, TOTP)

**Current:** MeView is a drill-in index with no desktop breakpoint; passkey has a desktop card grid,
TOTP a 2-col enrollment form; **the theme 3-option selector lives in the header overflow menu**
(`MobileWorkspaceHeader.tsx:233-262`), not here; **destructive actions have no confirmation at all**
— delete passkey (`PasskeySettingsView.tsx:177`) and disable TOTP (`TotpSettingsView.tsx:163`) fire
immediately; no skeletons; no retry.

**Changes:** put every destructive action behind `ConfirmDialog` with clear consequence copy (mirror
the correct EntryDetail delete flow) and, where reversible, an undo toast; place the canonical
`system/light/dark` selector in Me/Security (keep a quick toggle in the desktop/tablet chrome — see
Decision D3); give MeView a desktop multi-column card grid; migrate to semantic tokens (dark) +
weights; `Button`/`FormField`.

**Acceptance:** no destructive security action executes without a `ConfirmDialog`; theme selector
present in Me/Security; desktop card grid; dark theme correct; semantic tokens, weights ≤700.

### P10 — Search

**Current:** header-persistent search trigger; own result-list shape; reuses the `.emptyState`
*class* (not the primitive); **no search-term highlighting** (`includes()` filter, plain `<strong>`);
desktop 2-col at ≥1024px; no skeleton/retry.

**Changes:** reuse the RecentEntries list shape and the `EmptyState` primitive; add search-term
highlighting (`<mark>`); skeleton loading + retryable error; semantic tokens (dark).

**Acceptance:** matched terms highlighted; empty/loading/error via primitives; dark theme correct;
`transaction-search.css` on semantic tokens.

### P11 — Entry Detail / Edit

**Current:** hero amount is neutral ink (weight 920) with **no type/positive-negative color**
(`entry-detail.css:41-46`); editor is a bespoke `.entryDetailForm` (raw `<label>/<input>/<select>`),
**not** reusing Record's form primitives; **delete correctly uses `ConfirmDialog` + 10s undo toast**
(`EntryDetailEditor.tsx:95`, `EntryDetailRoute.tsx:84-103`) — the model to replicate elsewhere; only
the read-only hero/card/note have legacy dark rules; editor form has none.

**Changes:** color the hero amount by type/sign via `AmountText`; rebuild the editor on shared form
primitives (`FormField`/`Input`/`Select`/`Sheet` category picker); keep the exemplary delete→undo
flow; migrate to semantic tokens (full dark, incl. the editor form); weights ≤700.

**Acceptance:** hero amount carries type color; editor uses shared primitives; delete keeps
ConfirmDialog + undo; dark theme correct including the editor; semantic tokens.

### P12 — Tri-state & onboarding (cross-cutting)

**Current (`C10`):** no `Skeleton` anywhere; loading is plain text / inline `Spinner` / nothing;
one guided `EmptyState`; one retryable error affordance; shell Suspense fallback is plain text
(`MobileShell.tsx:171`); pre-auth splash is plain text (`App.tsx:57-67`).

**Changes:** every view gets a **skeleton** loading state (`Skeleton`, `W3.4`), a **guided
EmptyState**, and a **retryable error** state; replace the shell Suspense fallback and the pre-auth
splash with skeletons; first-run guidance is the Home empty state itself (create first account →
first entry), not a separate onboarding wizard.

**Acceptance:** `P1–P11` each present skeleton/guided-empty/retryable-error; a new user with an empty
book can complete "create account → first entry" from the Home empty state.

---

## 4. Implementation plan (stages & working method)

| Stage | Content | Depends on | Exit condition |
| --- | --- | --- | --- |
| **S0.0** (pre-flight) | **Fix `C0`:** run `node scripts/tokenize-colors.mjs` to regenerate `palette.css` (restores the 312 value-exact primitives, un-breaks rendering); harden `check:tokens` (`W1.7`) so an empty/stale palette fails CI; commit. | — | App renders with intended colors again; hardened gate proven by a failing fixture. |
| **S0.1** | `W1` designed tokens + light/dark + `color-scheme`; `W2` type scale; `W5.1` motion tokens. Establish Playwright screenshot baselines + the `font-weight>700` stylelint gate. | S0.0 | Gates live; semantic layer + designed primitives in place. |
| **S1** | `W3` primitives (build the 7 missing; ready the adopters) + `W5` motion + `W6.1` shared chart module. | S0.1 | Each primitive has a test + light/dark + a11y; shared chart hook exists. |
| **S2** | `W4` shell (remove phone-frame; real rail/sidebar; desktop header chrome; Toast). | S1 | Three-breakpoint baselines updated; navigation a11y passes. |
| **S3** | `P1–P11` migrated **one page per PR**, in order: **P6 Record → P3 Home → P7 Reports → P5/P4 → P1 Landing → P2 Auth → P8–P11.** Each PR repoints that page to semantic tokens, adopts primitives, converges weights, and **deletes the page's old CSS + hand-rolled components + its `--tone-*` refs**. | S2 | Each page meets its §3 acceptance row + the per-page template in §6. |
| **S4** | `P12` tri-state sweep + global polish (alignment/spacing/consistency) + a11y & performance audit; delete the now-unused generator + machine palette. | S3 | §5 matrix all green; `grep var(--tone-` → 0. |

**Working method (binding):**

- Every PR attaches three-breakpoint × dual-theme screenshots; the visual-regression baseline is
  updated with the PR and the reviewer diffs it as the design review.
- Five-language layout check is part of each page's exit (at least `zh`/`en`/`ja` screenshots;
  `es`/`fr` spot-checked); `check:i18n` stays green.
- No feature flags, no old-style toggles — each page cutover is atomic.

---

## 5. Test matrix

| Dimension | Method | Coverage | Pass criteria |
| --- | --- | --- | --- |
| Visual regression | Playwright screenshot diff | `P1–P11` × {phone,tablet,desktop} × {light,dark} | Pixel-consistent with the reviewed baseline |
| Accessibility | axe (Playwright) + manual keyboard walk | all pages + `W3` primitives + all charts | axe 0 serious/critical; keyboard-only sign-in→record→report→sign-out |
| Contrast | token-level automated check | every semantic pair, both themes | body ≥ 4.5:1, large/graphic ≥ 3:1 (WCAG 2.2 AA) |
| Responsive | manual + screenshots at 375/430/768/1024/1440 | all pages | no horizontal scroll, no occlusion, touch targets ≥ 44px |
| Theme | screenshot matrix + toggle-instantaneity | all pages incl. Landing/Auth | no unthemed blocks; no white flash on toggle; `color-scheme` applied |
| Chart interaction | Playwright pointer/touch/keyboard scripts | sparkline, donut, ranked bar, trend | scrub/tooltip/keyboard/ARIA all pass |
| Motion | manual + forced `prefers-reduced-motion` | the four standard motions | no positional/scale animation under reduce; durations/easings from tokens |
| i18n | five-language screenshot spot-check + `pnpm --dir web run check:i18n` | full for P1/P2/P6/P7, spot-check others | no overflow/truncation/breakage; key check green |
| Performance | Lighthouse (Landing, Home, Reports) | cold load + interaction | LCP ≤ 2.5s, INP ≤ 200ms, CLS ≤ 0.1 (lab) |
| Token/typography gate | `check:tokens` + `lint:css` + stylelint | all CSS | 0 color literals outside `palette.css`; 0 `var(--tone-*)`; 0 `font-weight > 700` |
| Existing unit/E2E | Vitest + existing Playwright + `test:coverage` | all changed pages | existing suites green (assertions may update for DOM changes; e.g. `app.workspace.test.tsx:57`) |

Reproducible metric commands (for reviewers):

```
# color literals outside the primitive file
grep -REn "oklch\(|#[0-9a-fA-F]{3,8}|rgba?\(" web/src --include='*.css' | grep -v '/styles/palette.css'
# machine-primitive references remaining
grep -rn "var(--tone-" web/src --include='*.css' | wc -l
# font weights above 700
grep -rhoE "font-weight:\s*[0-9]+" web/src --include='*.css' | sort | uniq -c
# legacy theme track
grep -rn "mobileShellTheme\|prefers-color-scheme\|phoneFrame" web/src --include='*.css' --include='*.tsx'
# primitive adoption
grep -rn "from '@/components/ui'" web/src/features
```

---

## 6. Acceptance criteria

The overhaul is complete when **all** of the following hold:

1. **Token coverage 100%.** Outside `styles/palette.css`, the color-literal grep in §5 returns 0;
   `var(--tone-*)` in feature CSS returns 0 (feature CSS is on the semantic layer); `palette.css`
   is a hand-authored designed scale; `check:tokens` fails on an empty/undefined-primitive palette.
2. **Dual theme complete.** Every route (incl. Landing/Auth) is correct in light and dark; theming
   is `data-theme`-only; `.mobileShellThemeDark/Light` and scattered `prefers-color-scheme` color
   rules are deleted; `color-scheme` is set.
3. **Typography converged.** `font-weight` is only 400/500/600/700; money is `AmountText` with
   consistent `tabular-nums`.
4. **Desktop workspace.** No `.phoneFrame` and no rounded phone mockup at any width;
   Home/Reports/Record/Accounts render the DSL multi-column forms; ≤767px keeps the full-bleed phone
   form with a working bottom nav.
5. **Chart standard.** Every user-visible chart meets the seven W6 criteria; `reportColors.ts` is
   deleted and replaced by chart tokens.
6. **Primitives.** The `W3` set is built and actually used by the corresponding pages; per-feature
   duplicate buttons/overlays/forms/empty-states are deleted; every destructive action (delete
   entry, delete passkey, disable TOTP, apply import) goes through `ConfirmDialog` with clear
   consequence copy.
7. **Tri-state.** `P1–P11` each have skeleton loading, a guided empty state, and a retryable error
   state; a new user can complete "create account → first entry" from the Home empty state.
8. **Landing.** The live product demo replaces the faked mockup; dual-theme, five-language,
   three-breakpoint all hold; Lighthouse targets in §5 met.
9. **Accessibility.** The accessibility and contrast rows in §5 are green; money and destructive
   operations meet WCAG 2.2 error prevention (confirmable/reversible); auth errors are announced.
10. **Test matrix green.** Every §5 row passes; baselines committed; existing Vitest/Playwright
    green.
11. **Docs synced.** `docs/arch/frontend.md` is updated to describe the shipped tokens/theme/shell
    (and corrected re: the palette). This handbook is moved to `docs/proposals/archives/` once the
    matrix is green.

**Per-page acceptance template** (apply when expanding each `P*` into tasks): dual-theme
three-breakpoint screenshots reviewed; axe 0 serious/critical; keyboard-complete; i18n spot-check
unbroken; the page's old CSS and hand-rolled components deleted; the page's `var(--tone-*)` count is
0; each DSL region verified against the render.

---

## 7. Risks & decisions

### Open decisions (resolved here for the dev team)

- **D1 — Record entry types.** The proposal named three (expense/income/transfer); the code has
  **four** (adds `borrow`). **Keep all four**; render them in `SegmentedControl` (scrollable if
  space-constrained on the narrowest phones).
- **D2 — Reports dimension nav.** The proposal suggested a `SegmentedControl`; seven dimensions are
  too many for a segmented control. **Keep tab semantics** (accessible `role="tablist"`, already
  implemented) restyled to the primitive; use `SegmentedControl` for the binary flow/time filters.
- **D3 — Theme selector location.** The canonical `system/light/dark` selector moves to
  **Me/Security** (per the proposal); a quick light/dark toggle stays in the desktop/tablet chrome
  (`W4` rail/header). Remove it from the phone header overflow to declutter.
- **D4 — Accounts desktop.** Adopt true **master-detail** (tree + detail side by side) on desktop;
  the current controls+list + navigate-away pattern is replaced.

### Risks

| Risk | Mitigation |
| --- | --- |
| The empty-palette regression (`C0`) means the app currently renders with fallback colors | `S0.0` fixes it before any other work and adds a gate so it cannot recur |
| Mixed old/new visuals during the sweep | `S3` is atomic per page; product is pre-launch, so no user exposure |
| Designing the full token scale up front is costly | `S0.1` fixes primitive + semantic; the component layer is added per page as needed |
| Landing live-demo weight/perf | reuse real components + existing seed data (no video/large images); lazy-load; keep a static first frame for LCP |
| Screenshot-baseline churn | baselines update only on page-level PRs; the reviewer diffs them as the design review |
| Overlap with the (now-archived) shell decomposition | `W4` is a reframe of the existing rail/sidebar structure, not a second large rewrite of the same files |
