# Security and Dependency Audit

Date: 2026-04-22

Scope: `api-go`, `webui`, Woodpecker validation, committed config samples, and
repo-backed QA/security artifacts. The archived Python backend was not modified.

## Checks Performed

- Reviewed Go and npm dependency manifests: `api-go/go.mod`,
  `api-go/go.sum`, `webui/package.json`, and `webui/package-lock.json`.
- Reviewed Woodpecker pipelines for dependency audit gates and secret handling.
- Searched the active repository for private keys and obvious committed
  credential patterns, excluding generated lockfiles and the archived backend.
- Reviewed committed config samples and Compose files for plaintext production
  secrets versus local/test-only example credentials.
- Reviewed existing auth, access-control, SigV4, source-failure, and integrity
  coverage documented in `docs/qa-coverage-audit-2026-04-22.md`.

## Findings

No committed private keys or live service credentials were found in the active
repository. `GITHUB_TOKEN` is consumed through Woodpecker `from_secret`, and live
Google Drive/Telegram E2E variables are documented as optional external-source
checks rather than required PR secrets.

Local and test configs intentionally use example MongoDB, MinIO, and SFree
secrets for disposable development stacks. `api-go/config/production.yaml` does
not contain plaintext production credentials; runtime secrets are expected via
environment variables.

Existing security-relevant tests cover S3 credential isolation, invalid SigV4
signatures, SigV4 diagnostic redaction, bucket grant route scoping, checksum
mismatch detection, short and oversized chunk reads, upload failover, source
failure cleanup, missing object XML behavior, and route-aware rate limiting.

The actionable gap was dependency vulnerability gating: backend and frontend CI
validated builds/tests but did not explicitly fail on known vulnerable Go or npm
dependencies.

## Changes Made

- Added a Woodpecker `dependency audit` step to `.woodpecker/api-go.yml` that
  installs and runs `govulncheck@v1.1.4`. The version is pinned because newer
  govulncheck releases currently require a newer Go toolchain than the Go 1.24
  CI image. Vulnerability findings are reported but do not fail the pipeline
  until the current Go 1.24 and module vulnerability baseline is remediated;
  tool installation and execution errors still fail the step.
- Added `npm audit --audit-level=critical` to `.woodpecker/webui.yml` after
  lockfile-based install and before lint/build/E2E. The current frontend
  dependency baseline has high advisories in transitive tooling dependencies,
  so critical advisories are the blocking threshold until that baseline is
  remediated.
- Updated `docs/ci.md` so the CI matrix and dependency-audit expectations match
  the pipelines.

## Residual Notes

The demo recorder under `scripts/record-demo` has its own `package.json` without
a committed lockfile and is not part of the production app or required
Woodpecker path. If that script becomes a maintained CI or release artifact, add
a lockfile and an audit gate for it at that time.

Local CPU-heavy validation was intentionally not run. The new audit checks are
designed to run in Woodpecker.

The backend audit currently reports known findings without blocking merge. The
Woodpecker run found Go standard-library findings fixed in Go 1.25.x and module
findings in `github.com/golang-jwt/jwt/v5`, `github.com/gin-contrib/cors`, and
`go.opentelemetry.io/otel/sdk`. Raise the backend audit to blocking after the CI
Go image and affected modules are updated.

The frontend audit threshold should be raised from `critical` to `high` after
the current high-advisory tooling baseline is cleared.
