/** @type {import('stylelint').Config} */
export default {
  extends: ['stylelint-config-standard'],
  rules: {
    // Existing CSS uses React-era camelCase class names. Phase 4 will replace
    // bespoke feature CSS with prefixed classes and shared primitives.
    'container-name-pattern': null,
    'selector-class-pattern': null,

    // Existing feature styles predate the cascade-layer/token pass and have
    // selector ordering debt. Phase 4's layer migration owns that cleanup.
    'no-descending-specificity': null,

    // Existing OKLCH literals omit `deg`; keep the current syntax stable until
    // Phase 4 tokenizes feature colors and blocks new feature color literals.
    'hue-degree-notation': null,

    // Existing global CSS intentionally uses these platform spellings.
    'font-family-name-quotes': null,
    'media-feature-range-notation': null,
    'property-no-deprecated': null,
    'value-keyword-case': null,
  },
};
