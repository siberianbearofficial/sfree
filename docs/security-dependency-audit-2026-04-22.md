# Security and Dependency Audit

Date: 2026-04-22

Scope: `api-go` vulnerability baseline, backend Go toolchain images, `webui`
npm dependency baseline, and Woodpecker validation. The archived Python backend
was not modified.

## Backend Remediation

- Moved backend CI and build defaults from Go 1.24 to Go 1.25 so standard
  library findings reported for `crypto/x509`, `crypto/tls`, `html/template`,
  `os`, and `net/url` are covered by the newer toolchain.
- Updated reachable vulnerable backend modules:
  `github.com/golang-jwt/jwt/v5` to `v5.2.2`,
  `github.com/gin-contrib/cors` to `v1.6.0`, and
  `go.opentelemetry.io/otel/sdk` to `v1.40.0`.
- Added a blocking Woodpecker `dependency audit` step for `api-go` that installs
  and runs `govulncheck@v1.1.4`.

## Backend Validation

No CPU-heavy local backend validation was run. Docker, E2E, full test suites,
and `govulncheck` execution are expected to run in Woodpecker for this
repository.

## Frontend Findings

The committed `webui/package-lock.json` has no high or critical findings from
`npm audit --audit-level=high`.

## Changes Made

- Updated the webui tooling dependency baseline within existing major versions
  so the lockfile resolves fixed transitive versions for the high-severity audit
  findings in `vite`, `rollup`, `tar`, `picomatch`, `minimatch`, and `flatted`.
- Added narrow npm overrides for `rollup` and `flatted` because those
  transitives remained below fixed versions after direct tooling upgrades.
- Added `npm audit --audit-level=high` to `.woodpecker/webui.yml` after
  lockfile-based install and before lint/build validation.
- Updated `docs/ci.md` so the webui pipeline and dependency audit notes describe
  the final high threshold.

## Frontend Validation

- Ran `npm audit --audit-level=high` in `webui`; it reported zero
  vulnerabilities.
- Ran `npx -y npm@10.8.2 ci --include=dev` followed by
  `npx -y npm@10.8.2 audit --audit-level=high`; both completed with zero
  vulnerabilities.
- Woodpecker remains responsible for the required lint, build, Playwright, and
  PR audit gates.
