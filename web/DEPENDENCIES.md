# Frontend dependency compatibility

Use the package-manager version in package.json and a Node version supported by
all selected dependencies. CI uses Node 24; React Router 8.3 requires Node
22.22.0 or newer and React/React DOM 19.2.7 or newer.

The app already uses the React Router 8 package and ESM imports. The 8.1 to 8.3
update requires no router-package migration. Keep navigation, protected routes,
deep links, and desktop/mobile browser acceptance passing when updating it.

Vitest and @vitest/coverage-v8 must resolve to the same release. Upstream
explicitly warns that mixed releases are unsupported. Update both package
specifications and the pnpm lockfile together, then run coverage without lowering
the configured thresholds.

Coverage instruments lazily imported route modules. On bounded CI workers, their
cold transforms can exceed Testing Library's one-second default. The test setup
uses a five-second asynchronous query budget, matching Vitest's default test
budget, while preserving all accessible-role and data assertions.

Run frontend formatting, TypeScript/CSS/dead-code lint, color-token, locale,
generated OpenAPI, unit/coverage, build, and Playwright gates from the repository
root using pinned pnpm. Playwright uses local synthetic accounts and starts its
own API and frontend on configurable ports; it does not require a deployed account.

## Upstream references

- [React Router 8.3 package requirements](https://github.com/remix-run/react-router/blob/react-router%408.3.0/packages/react-router/package.json)
- [React Router versioned changelog](https://github.com/remix-run/react-router/blob/react-router%408.3.0/CHANGELOG.md)
- [Vitest 4.1.11 package requirements](https://github.com/vitest-dev/vitest/blob/v4.1.11/packages/vitest/package.json)
- [Vitest coverage version check](https://github.com/vitest-dev/vitest/blob/v4.1.11/packages/vitest/src/node/coverage.ts)

## Transitive security patches

The lockfile updates compatible transitive versions within their existing parent
ranges. No dependency major version, audit suppression, assertion or coverage
threshold changes are required. The production dependency audit reports no
advisories; Vite and its CSS tooling are included in that audit.

OpenAPI TypeScript stays at 7.13.0. Its existing ^1.34.6 range admits
@redocly/openapi-core 1.34.20, whose published dependency pins js-yaml 4.3.2.
This parent patch removes the old exact YAML pin without an override.

The full development graph still reports these inherited advisories:

- braces 3.0.3: [braces vulnerable to stack-exhaustion denial of service through deeply nested patterns](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm). Paths: .>stylelint>micromatch>braces. Patched range: <0.0.0.

A patched range of <0.0.0 means the advisory has no released fix. In particular,
braces 3.0.3 remains vulnerable; it is not suppressed or described as fixed.
