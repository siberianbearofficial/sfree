# api-go S3/API Testing Plan

## Purpose

This plan scopes the first bounded QA slice for public GitHub issue #89. It records the current `api-go` S3/API coverage, identifies the most useful near-term gaps, and keeps new work aligned with the existing Woodpecker-only validation path.

## Current Coverage

### April 22, 2026 Audit Snapshot

Recent merged backend work added targeted coverage for S3 range corruption, multipart abort bucket ownership, source health documentation, manager chunk I/O splitting, direct source download preflight, and bucket deletion cleanup. Open PR review found these relevant active branches:

- PR #282 / THE-681 adds a focused manager regression for weighted upload failover when a 1000:1 high-weight source fails and the healthy low-weight source must still be tried.
- PR #281 / THE-682 narrows ObjectService construction to operation-specific services without changing the tested manager behavior.
- PR #280 / THE-631 adds shared S3 object-key parsing coverage for normal, leading-slash, and empty wildcard keys.
- PR #279 / THE-673 splits S3 object handler helpers while retaining copy-source parser coverage.

The current practical gap is not broad unit coverage. It is cross-boundary regression coverage where S3-compatible requests, REST file lifecycle requests, metadata persistence, and source-placement behavior interact.

### `api-go/internal/e2e/s3_compat_test.go`

This file covers API-backed source and bucket creation through the Go E2E harness. It verifies that created S3 sources appear in the source list, bucket S3 access credentials are issued, and deleting an in-use source returns `409 Conflict`.

The broader Go S3 E2E suite is split into sibling files under `api-go/internal/e2e/`:

- `s3_compat_object_test.go` covers object upload, download, overwrite, delete, range reads, `HEAD`, and copy behavior.
- `s3_compat_list_test.go` covers ListObjects/ListObjectsV2 prefix, delimiter, and pagination behavior.
- `s3_compat_delete_test.go` covers multi-object delete behavior.
- `s3_compat_multipart_test.go` covers multipart create, upload part, list uploads, list parts, complete, abort, and error paths.
- `s3_compat_auth_test.go` covers access-key authentication, signature failures, unsigned requests, presigned GET/PUT, and expired presigned URLs.

### `api-go/e2e/test_api_e2e.py`

The Python E2E suite exercises the public API through the async test client and the AWS SDK-compatible client path. It covers source lifecycle checks, HTTP and S3 upload/download interoperability, S3 overwrite behavior, ListObjectsV2 prefix/delimiter/pagination, ranged GET, `HEAD`, copy object, multi-object delete, single-object delete idempotency, source delete conflicts, and multipart upload flow.

This suite is useful as a cross-client compatibility signal because it goes through `aiobotocore`, but it should stay focused on end-to-end API compatibility rather than duplicating every Go handler edge case.

### `api-go/internal/s3compat/client_test.go`

The unit tests cover S3 source client configuration parsing and pagination handling in the internal S3-compatible source client. They do not exercise HTTP handler error shapes, auth boundaries, or object metadata behavior.

### `.woodpecker/api-go.yml`

The required API Go pull-request pipeline runs:

- `golangci-lint run`
- `go test ./...`
- Python E2E through `docker-compose.e2e.yml` with local MinIO
- Go S3 E2E through `docker-compose.go-e2e.yml` with local MinIO

This is the correct validation target for CPU-heavy and Docker-backed checks. Local agent runs should stay limited to lightweight review unless explicitly requested.

## First Coverage Slice

The first implemented slice targets S3 error behavior that protects compatibility and data isolation:

- Signed `GET` for a missing object returns `404` with S3 XML error code `NoSuchKey`.
- Credentials for one bucket cannot read object bytes from another bucket, even when the object key is known.

These checks live in the existing Go S3 E2E suite so they run in the Woodpecker API Go pipeline without new CI commands.

## Prioritized Backlog

1. Add an API-backed placement regression that uploads enough chunks through a weighted bucket to prove stored file chunk metadata follows the configured source weights, then downloads the object to prove reconstruction still succeeds.
2. Add HTTP API/S3 parity checks for object metadata after S3 overwrite, CopyObject, delete/recreate, and multipart completion. Existing metadata work has strong unit and E2E coverage, but parity across REST and S3 lifecycle paths remains the highest-value integration gap.
3. Add an S3 error-shape matrix for missing bucket, missing multipart upload, unsupported copy metadata directive, invalid range, auth mismatch, and cross-bucket copy attempts.
4. Add source/backend failure coverage for client-construction failures separately from upload failures. PR #282 covers upload-time weighted failover; constructor/preflight failures still need explicit expected behavior before QA should encode it.
5. Add a cleanup regression for multi-source uploads where an early chunk succeeds on one source, later failover exhausts all sources, and previously uploaded chunks must be deleted exactly once.

## Validation

No CI behavior changes are required. Woodpecker `.woodpecker/api-go.yml` remains the validation source for Docker-backed E2E execution.

Local validation for this audit was limited to repository inspection, GitHub PR inspection, and `git diff --check` because backend test, Docker, and E2E execution belong in Woodpecker on this limited machine.
