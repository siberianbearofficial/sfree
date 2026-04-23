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

## 2026-04-22 Audit

Recent `api-go` changes added or updated coverage for fixed S3 E2E waits, multipart part replacement cleanup ordering, CopyObject `REPLACE` errors, REST file lifecycle through `ObjectService`, bucket deletion cleanup, source health, retry behavior, weighted source validation, bounded SigV4 hashing, raw-query canonicalization, file-list search, short-read upload chunking, source provider config validation, bucket grant lookup errors, share-link request validation, object-key normalization, range corruption, direct source download preflight, and download context lifetime. Open pull requests at the time of audit included smoke registry timeout hardening, duplicate download-preflight read removal, owned-source helper extraction, source capability decoupling, handler/router test harness work, share-link and bucket-grant cleanup failure handling, weighted upload failover, operation-specific `ObjectService` construction, escaped source download keys, bucket detail API, dependency audit work, S3 object metadata, missing bucket source uploads, multipart abort cleanup failures, and rate-limit routing.

Covered critical behavior:

- Object write/read roundtrip, overwrite, delete, range reads, `HEAD`, copy, missing-object S3 XML errors, and cross-bucket credential isolation are covered in the Go S3 E2E suite.
- Chunking, short reads, per-chunk checksum storage, checksum verification, range reconstruction, weighted and round-robin placement, source failover, and source/backend upload failures are covered in manager unit tests.
- Multipart create/upload/list/complete/abort flows, part replacement cleanup ordering, invalid completion requests, and completed chunk checksum preservation are covered by Go E2E and manager/handler unit tests.
- Bucket deletion cleanup covers completed objects, pending multipart uploads, shared chunk references, and metadata cleanup ordering.
- CI routes the API module through Woodpecker lint, `go test ./...`, docs freshness, Python E2E, Go S3 E2E, and the stack smoke workflow.

Test improvement added during this audit:

- `api-go/internal/manager/s3_object_test.go` now asserts `ObjectService.CopyObject` preserves chunk order and checksum metadata, not only chunk name/reference.

Prioritized missing tests:

1. Cleanup-failure propagation for share links and bucket grants. Add public REST tests proving bucket/file delete either removes dependent share-link/grant records or returns a stable non-success response when cleanup fails, without reporting partial success.
2. REST/S3 parity after REST file overwrite and delete. Add an integration or E2E check that uploads through `/api/v1/buckets/{id}/upload`, overwrites the same filename, verifies S3 GET returns only the replacement bytes, deletes through the REST file endpoint, and verifies S3 GET returns `NoSuchKey`. This remains high value because REST lifecycle was rerouted through `ObjectService`.
3. Escaped direct source download keys. Add handler or source-client coverage proving escaped slash, plus, and space variants reach the backend source key parser as intended.
4. Weighted upload failover. Add a manager regression proving failed weighted primary sources are skipped for retry, successful alternate placement preserves chunk metadata, and selection does not retry the same failed source for the same chunk.
5. Cross-bucket CopyObject authorization and source lookup matrix. Existing E2E covers same-user cross-bucket copy and service-level cross-user rejection; add S3 handler/E2E coverage for wrong destination credentials, unknown source bucket, unknown source key, and copy from a bucket owned by a different user.
6. Multipart completion cleanup on partial completion. Current coverage verifies replacement and abort cleanup; add an E2E or manager test that uploads parts 1, 2, and 3, completes only parts 1 and 3, then verifies unrequested part 2 chunks are cleaned while completed object reconstruction remains correct.
7. Source/backend download failure mapping on S3 `GET` and REST download. Unit tests cover preflight failure behavior with injected checksum errors; add a concrete source-client download failure case to ensure the API returns a clear non-success response before success headers are committed.
8. Metadata parity after CopyObject and multipart completion. Existing unit coverage protects chunk metadata; add S3 `HEAD`/list checks that size and ETag behavior remain stable after copy and multipart completion.

## Validation

No CI behavior changes are required. Woodpecker `.woodpecker/api-go.yml` remains the validation source for Docker-backed E2E execution.
