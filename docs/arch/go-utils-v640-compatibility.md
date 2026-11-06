# go-utils v6.4.0 consumer compatibility

Accounting upgrades its backend from v6.3.2-0.20261008165128-506ec9758d5f
to the stable v6.4.0 release, tag commit
6a02887d3dbf60ecadf21a67b5d3b9449fd942e6. The release requires Go 1.26;
Accounting already declares Go 1.26.4. The CLI does not depend on go-utils.

## Consumer contracts

The backend uses the logger interfaces, console logger construction, levels,
structured hooks and context propagation, plus NewRateLimiter for optional
alert delivery. The relevant upstream logger, alert, alert-policy and
rate-limiter source files are identical between these two versions.
internal/logger/utils_compatibility_test.go exercises real Accounting setup
with an in-process synthetic GraphQL receiver, proving error metadata delivery,
Info suppression, structured field exclusion remotely, structured field
availability locally, context logger identity and the one-token alert limit.
Invalid limiter configuration and canceled consumption are negative controls.
No real alert service or credential is used.

Accounting does not call go-utils password, ciphertext or file helpers.
Its own new password writer remains Argon2id (m=19456,t=2,p=1), with legacy
PBKDF2-SHA256 verification at 600,000 iterations and successful-login rehash.
The existing password, login, keyring, TOTP and file-store regressions are
retained. Existing enc:v1: TOTP ciphertext, retired decrypt-only keys, JSON
persistence, authorization and empty/error contracts are unchanged.
The upstream PasswordHash compatibility change does not affect this consumer.

## Acceptance and release

Run the logging regressions against both dependency versions, then the required
root make lint and make test gates, backend/CLI builds and frontend build.
These gates include backend and CLI race tests, existing auth/persistence
controls and frontend Vitest. API/UI behavior is unchanged, so this dependency
update does not require a new browser e2e matrix.
External PostgreSQL tests need an explicit isolated fixture; a local run without
DATABASE_URL skips those tests and must not be reported as PostgreSQL acceptance.
Exact commit and CI evidence belong in the accompanying Draft PR description.

This is a dependency update without a schema, stored-data or configuration
migration. Deployment remains a separate operator action after review and CI.
Rollback restores the prior module pin and checksums, rebuilds, and deploys the
previous reviewed artifact. Existing password and ciphertext readers/writers
are unchanged, so rollback needs no data conversion.

Sources:
- https://github.com/Laisky/go-utils/releases/tag/v6.4.0
- https://github.com/Laisky/go-utils/tree/6a02887d3dbf60ecadf21a67b5d3b9449fd942e6
- docs/arch/arch.md (authentication and persistence contracts)
