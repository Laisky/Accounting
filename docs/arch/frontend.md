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

- Keep global primitives in `web/src/styles/tokens.css`; feature CSS can define local semantic
  tokens when the values are not yet shared.
- Preserve mobile-first behavior and add larger layouts with `min-width` media queries or
  container queries.
- Do not hide critical bookkeeping controls on phones. Adapt density, scrolling, or disclosure
  while keeping the workflow complete.
- Keep all visible UI copy in i18n bundles and verify locale shape with `pnpm --dir web run check:i18n`.
