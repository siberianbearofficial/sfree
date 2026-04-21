# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, recent `origin/main` changes, open backend PRs as of 2026-04-21 10:20 UTC, and the checked-out multipart E2E coverage branch.

## Recent Inputs

- `origin/main` now includes focused S3 range GET, ListObjectsV2 pagination, checksum verification, and Woodpecker image-pinning changes.
- Open PR [#178](https://github.com/siberianbearofficial/sfree/pull/178) adds S3 GET stream-failure semantics and handler regression tests for corrupt chunks before success headers/body are emitted.
- Open PR [#176](https://github.com/siberianbearofficial/sfree/pull/176) adds S3 DeleteObjects support plus E2E coverage for normal delete, missing-key idempotency, quiet mode, list-after-delete, and oversized XML rejection.
- Open PR [#171](https://github.com/siberianbearofficial/sfree/pull/171) adds CI hardening and additional S3 regression assertions, but overlaps older coverage now present on `origin/main`.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, and failover. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested. S3 ETag, range headers, and basic object headers are checked in E2E. | Weak for user metadata and content type because those are not implemented. |
| Placement logic | Round-robin, weighted selection, and upload failover have unit coverage. | Covered for manager logic. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object and ranged download. | Covered for manager logic; weak for HTTP error behavior on backend corruption. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover and resilience wrappers have tests. PR #178 adds S3 GET stream-failure handler coverage. | Covered for upload/retry logic; S3 GET failure semantics are pending PR merge. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, expired presign, range GET, ListObjectsV2 prefix/delimiter/pagination, and multipart lifecycle on the checked-out branch. PR #176 adds DeleteObjects E2E. | Covered for currently implemented core operations once open PRs merge; weak for CopyObject, metadata, and unsupported/malformed request error shapes. |
| Recent bug regressions | Recent checksum, ListObjectsV2, range GET, and multipart work have focused tests. Web UI lockfile fix belongs to web UI CI. | Covered for the latest backend regressions. |
| Recently changed code paths | Backend CI runs Go unit tests plus Python and Go E2E through Woodpecker for `api-go/**`. | Covered by CI routing. |

## Prioritized Missing Tests

1. Merge and keep the S3 GET backend corruption/failure regression from PR #178.
   Acceptance: a corrupted or unavailable chunk must not look like a successful full-object or ranged download to an S3 client. The regression should assert XML `InternalError` and absence of success headers/body before this gap is closed.

2. Merge and keep the DeleteObjects E2E from PR #176.
   Acceptance: batch delete is idempotent for missing keys, quiet mode suppresses deleted entries, list-after-delete proves removed/surviving keys, and oversized XML bodies fail with a stable S3 error.

3. Add S3 object metadata/header behavior coverage when metadata support is implemented.
   Acceptance: PUT with `Content-Type` and `x-amz-meta-*` either preserves those values through HEAD/GET or returns an intentional S3-compatible unsupported-feature response.

4. Add CopyObject E2E after that code path is implemented.
   Acceptance: copy preserves content and expected metadata/ETag behavior, rejects missing source objects with XML S3 errors, and leaves the destination unchanged on failure.

5. Add S3 lifecycle/error-shape coverage for malformed and unsupported requests.
   Acceptance: unsupported operations and malformed query combinations return XML S3 errors with stable status codes instead of generic JSON or empty responses.

## Work Completed In This Audit

- Confirmed `TestS3CompatGetObjectRange` covers valid bounded, open-ended, suffix, invalid, and full-object GET range behavior.
- Confirmed `TestS3CompatListObjectsV2PrefixDelimiterAndPagination` covers V2 prefix filtering, delimiter common prefixes, continuation-token pagination, and V1 delimiter regression behavior.
- Confirmed the checked-out branch adds `TestS3CompatMultipartUploadLifecycle` for create, upload, re-upload part, list parts, complete, GET completed object, abort cleanup, and missing upload IDs.
- Confirmed PR #178 contains focused S3 GET stream-failure tests for full and ranged GETs.
- Confirmed PR #176 contains DeleteObjects E2E coverage and XML body size-limit regression coverage.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main run Go unit tests plus S3-backed Python and Go E2E suites in Woodpecker.

## Verification

Local CPU-heavy validation was intentionally not run. The backend suite should be executed by Woodpecker for this branch.
