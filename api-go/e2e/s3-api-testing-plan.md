# THE-404: api-go S3/API Testing Plan

**Status date:** 2026-04-21

**Scope:** api-go S3-compatible endpoint behavior and REST-adjacent API side effects.

## Objectives

Issue status: in progress for QA coverage and regression audit.

1. Reduce release risk by pinning S3 API regressions to reproducible scenarios.
2. Increase confidence in request handling for missing resources and auth edge-cases.
3. Keep the first pass intentionally narrow and high-signal, then expand to multipart and protocol gaps in subsequent slices.

## Coverage matrix (api-go)

- S3 object lifecycle (Put/Get/Head/Delete/List + auth): partially covered by existing Python and Go e2e suites.
- Negative paths:
  - Wrong credentials and unsigned requests.
  - Missing/non-existent bucket and object behavior.
  - Empty bucket/list semantics.
- Multipart coverage: still present via route/dispatch tests and existing end-to-end scenarios.

## Test execution

- **Unit:** `go test ./...` (pre-existing unit/integration coverage)
- **E2E (current):** `go test -v -tags=e2e ./internal/e2e/...` in `api-go/docker-compose.go-e2e.yml`
- **Python E2E:** `cd api-go && make test-e2e` (source-type matrix in CI)

## First coverage slice (THIS ISSUE)

- Add Go e2e assertions for API behaviors that are not currently asserted:
  - DeleteObject on missing key is idempotent (`204`) and does not fail.
  - ListObjects on a freshly created, empty bucket returns an empty XML result (`KeyCount: 0`).
  - Requests against non-existent bucket return `NoSuchBucket` XML error.
  - ListObjects against non-existent bucket returns `NoSuchBucket` XML error (added in follow-up slice).
  - Empty object path on signed PUT returns `InvalidRequest`.
  - Object keys containing `//` work end-to-end for PUT/GET/DELETE.
- Track follow-up slice after merge:
  - Expand test coverage for error payload assertions (error code/shape consistency).

## Evidence artifact

The implementation for this issue lands in:

- `api-go/internal/e2e/s3_compat_test.go`
- `api-go/e2e/s3-api-testing-plan.md`
