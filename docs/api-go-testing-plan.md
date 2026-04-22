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

1. S3 error-shape matrix for missing bucket, missing upload, unsupported copy metadata directive, invalid range, and auth mismatch paths.
2. HTTP API/S3 parity checks for file metadata after S3 overwrite, copy, delete, and multipart completion.
3. Negative auth coverage across presigned URLs, wrong bucket keys, wrong access keys, and cross-bucket copy attempts.

## 2026-04-22 Regression Audit

Recent merged changes on `origin/main` added share-link request body validation, range corruption regression coverage, multipart abort bucket ownership checks, direct source download preflight coverage, source health behavior, Web UI download failure handling, and manager chunk I/O extraction. Open PRs at audit time included source download query-key handling, share-link cascade cleanup, graceful API shutdown, bucket grant cleanup/error surfacing, weighted upload failover, operation-specific object service construction, S3 object key normalization, multipart abort cleanup failure preservation, missing bucket source upload failure, download context lifetime, authenticated rate-limit identity, and S3 object metadata persistence.

Covered critical behavior:

- Object write/read roundtrip, overwrite, delete, ranged `GET`, `HEAD`, copy, missing-object S3 XML errors, and cross-bucket credential isolation are covered in the Go S3 E2E suite.
- Chunking, short reads, range reconstruction, per-chunk checksum storage, corruption detection, weighted and round-robin placement, failover, and uploaded-chunk cleanup on source/backend failure are covered in manager unit tests.
- Multipart create, upload part, list uploads, list parts, complete, abort, replacement, invalid completion, ownership, and checksum preservation paths are covered across Go E2E, handler unit tests, and manager unit tests.
- Direct source downloads now have preflight failure coverage before success headers are committed, and share-link creation rejects malformed request bodies before records are created.

Test improvement added during this audit:

- `api-go/internal/manager/s3_object_test.go` now asserts `ObjectService.CopyObject` preserves chunk order and checksum metadata, not only chunk name/reference, while still cleaning overwritten destination chunks.

Prioritized missing tests:

1. REST/S3 parity after REST file overwrite and delete. Add an integration or E2E check that uploads through `/api/v1/buckets/{id}/upload`, overwrites the same filename, verifies S3 `GET` returns only the replacement bytes, deletes through the REST file endpoint, and verifies S3 `GET` returns `NoSuchKey`.
2. Cross-bucket CopyObject authorization and source lookup matrix. Existing coverage checks same-user cross-bucket copy and service-level cross-user rejection; add handler or E2E coverage for wrong destination credentials, unknown source bucket, unknown source key, and source bucket owned by another user.
3. Multipart completion cleanup on partial completion. Add an E2E or manager test that uploads parts 1, 2, and 3, completes only parts 1 and 3, then verifies part 2 chunks are cleaned while completed object reconstruction remains correct.
4. Share-link cascade cleanup API path. The cleanup PR adds repository and bucket deletion coverage; after merge, add a public API or E2E check proving revoked bucket/file share links are no longer listable or usable through the user-facing routes after file and bucket deletion.
5. Source download query-key route compatibility. The open source-download PR adds unit coverage for escaped query keys; after merge, add a Web UI or API compatibility smoke that covers S3 object keys containing `+`, `#`, `?`, and nested slashes through the public route.

## Validation

No CI behavior changes are required. Woodpecker `.woodpecker/api-go.yml` remains the validation source for Docker-backed E2E execution.
