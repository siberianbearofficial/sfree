# api-go S3/API Testing Plan

## Purpose

This plan scopes the first bounded QA slice for public GitHub issue #89. It records the current `api-go` S3/API coverage, identifies the most useful near-term gaps, and keeps new work aligned with the existing Woodpecker-only validation path.

## Current Coverage

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

1. Multipart metadata surface regression: create a multipart upload with `Content-Type` and `x-amz-meta-*`, complete it, then assert S3 `HEAD` and `GET` return the stored content type and user metadata. Current manager coverage checks `CompleteMultipartUpload` preserves metadata, but the Go and Python E2E suites only prove metadata through simple `PutObject` and `CopyObject`.
2. Missing bucket source handler mapping: add focused handler or Go E2E checks for REST upload, S3 `PutObject`, and multipart `UploadPart` when a bucket references a deleted or unresolved source. The open THE-591 branch adds object-service and source-resolution coverage, but the API/S3 error contract should be pinned so clients see the intended `400` or `InvalidRequest` response instead of a partial write or generic server error.
3. Cross-surface file size consistency: after THE-732 lands, add one regression that uploads a multi-chunk object and compares the same byte size through bucket file listing, REST download `Content-Length`, share-link download `Content-Length`, S3 `HEAD`/`GET`, and S3 list object `Size`. The helper has unit coverage, but the risk is divergent handler wiring.
4. Negative auth coverage across presigned URLs, wrong bucket keys, wrong access keys, and cross-bucket copy attempts.
5. S3 error-shape matrix for missing bucket, missing upload, unsupported copy metadata directive, invalid range, and auth mismatch paths.

## 2026-04-22 Audit Refresh

Recent `origin/main` changes added or refreshed coverage for S3 object metadata persistence, multipart helper cleanup, SigV4 query canonicalization, source provider config validation, source health, bucket cleanup, range corruption detection, and deterministic tests. Open PR review found targeted coverage in the active branches for file-size helper extraction, bucket deletion cleanup ordering, duplicate download preflight removal, weighted upload failover, escaped source download keys, missing bucket source failures, and provider-neutral source capabilities.

The strongest new test candidates are the first three backlog items above. They are concrete, user-visible regressions with existing harness locations:

- Multipart metadata belongs in `api-go/internal/e2e/s3_compat_multipart_test.go` or the Python SDK E2E suite if SDK parity is preferred.
- Missing source error mapping belongs near the existing S3/REST upload handler tests, with a later E2E only if handler coverage cannot verify the wire contract.
- File size consistency should wait for THE-732 to merge, then live in the Go S3 E2E suite or a narrow handler test that exercises all changed response paths.

## Validation

No CI behavior changes are required. Woodpecker `.woodpecker/api-go.yml` remains the validation source for Docker-backed E2E execution. This refresh was based on repository and PR inspection only; local Go, Docker, and E2E test execution were intentionally not run on the limited-resource agent machine.
