/** @type {import('stylelint').Config} */
export default {
  extends: ['stylelint-config-standard'],
  rules: {
    // Existing CSS uses React-era camelCase class names. The prefixed-class rename
    // ships incrementally with the CSS overhaul; primitives use ui-/semantic classes.
    'container-name-pattern': null,
    'selector-class-pattern': null,

    // Feature styles predate the cascade-layer pass and carry selector ordering debt;
    // the layer migration owns that cleanup.
    'no-descending-specificity': null,

    // Primitive OKLCH tokens in palette.css omit `deg`; keep the syntax stable.
    'hue-degree-notation': null,

    // Existing global CSS intentionally uses these platform spellings.
    'font-family-name-quotes': null,
    'media-feature-range-notation': null,
    'property-no-deprecated': null,
    'value-keyword-case': null,
  },
  overrides: [
    {
      // Raw color literals may only live in the token files. Feature/base CSS must
      // reference semantic tokens (tokens.css) or primitive tokens (palette.css).
      files: ['src/**/*.css'],
      ignoreFiles: ['src/styles/palette.css', 'src/styles/tokens.css'],
      rules: {
        'function-disallowed-list': [
          ['oklch', 'rgb', 'rgba', 'hsl', 'hsla'],
          {
            message:
              'Use color tokens (var(--surface), var(--tone-*), ...) instead of raw color literals outside token files.',
          },
        ],
        'color-no-hex': true,
      },
    },
  ],
};
