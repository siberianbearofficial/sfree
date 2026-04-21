# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, recent `origin/main` changes, and the checked-out multipart E2E coverage branch.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, and failover. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested. S3 ETag, range headers, and basic object headers are checked in E2E. | Weak for user metadata and content type because those are not implemented. |
| Placement logic | Round-robin, weighted selection, and upload failover have unit coverage. | Covered for manager logic. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object and ranged download. | Covered for manager logic; weak for HTTP error behavior on backend corruption. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover and resilience wrappers have tests. | Covered for upload/retry logic; weak for GET failure surfaced through S3 HTTP. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, expired presign, range GET, ListObjectsV2 prefix/delimiter/pagination, and multipart lifecycle on the checked-out branch. | Covered for currently implemented core operations; weak for unimplemented S3 operations and backend-failure HTTP semantics. |
| Recent bug regressions | Recent checksum, ListObjectsV2, range GET, and multipart work have focused tests. Web UI lockfile fix belongs to web UI CI. | Covered for the latest backend regressions. |
| Recently changed code paths | Backend CI runs Go unit tests plus Python and Go E2E through Woodpecker for `api-go/**`. | Covered by CI routing. |

## Prioritized Missing Tests

1. Define and fix S3 GET backend corruption/failure behavior, then add regression coverage.
   Acceptance: a corrupted or unavailable chunk must not look like a successful full-object download to an S3 client. Current handler code writes `200 OK` or `206 Partial Content` before `manager.StreamFile` / `manager.StreamFileRange` returns, so product-code behavior needs Software Engineer Head ownership before QA can add a stable expected-behavior test.

2. Add S3 object metadata/header behavior coverage when metadata support is implemented.
   Acceptance: PUT with `Content-Type` and `x-amz-meta-*` either preserves those values through HEAD/GET or returns an intentional S3-compatible unsupported-feature response.

3. Add DeleteObjects and CopyObject E2E after those code paths are implemented.
   Acceptance: batch delete is idempotent and reports per-key errors; copy preserves content and expected metadata/ETag behavior.

4. Add S3 lifecycle/error-shape coverage for malformed and unsupported requests.
   Acceptance: unsupported operations and malformed query combinations return XML S3 errors with stable status codes instead of generic JSON or empty responses.

## Work Completed In This Audit

- Confirmed `TestS3CompatGetObjectRange` covers valid bounded, open-ended, suffix, invalid, and full-object GET range behavior.
- Confirmed `TestS3CompatListObjectsV2PrefixDelimiterAndPagination` covers V2 prefix filtering, delimiter common prefixes, continuation-token pagination, and V1 delimiter regression behavior.
- Confirmed the checked-out branch adds `TestS3CompatMultipartUploadLifecycle` for create, upload, re-upload part, list parts, complete, GET completed object, abort cleanup, and missing upload IDs.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main run Go unit tests plus S3-backed Python and Go E2E suites in Woodpecker.

## Verification

Local CPU-heavy validation was intentionally not run. The backend suite should be executed by Woodpecker for this branch.
