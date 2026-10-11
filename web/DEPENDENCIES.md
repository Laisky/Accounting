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
