# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, and recent `origin/main` changes.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, and failover. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested. S3 ETag and basic object headers are checked in E2E. | Weak for user metadata and content type because those are not implemented. |
| Placement logic | Round-robin, weighted selection, and upload failover have unit coverage. | Covered for manager logic. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object download. | Covered for manager logic; weak for HTTP error behavior on backend corruption. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover and resilience wrappers have tests. | Covered for upload/retry logic; weak for GET failure surfaced through S3 HTTP. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, and expired presign. | Covered for simple operations; weak for multipart and listing semantics. |
| Recent bug regressions | Recent checksum work has unit tests. Web UI lockfile fix belongs to web UI CI. | Partially covered. |
| Recently changed code paths | Backend CI runs Go unit tests plus Python and Go E2E through Woodpecker for `api-go/**`. | Covered by CI routing. |

## Prioritized Missing Tests

1. Add live S3 multipart E2E coverage in `api-go/internal/e2e/s3_compat_test.go`.
   Acceptance: create multipart upload, upload two parts, list parts, complete upload, GET combined object, re-upload a part, abort an upload, and assert missing/invalid upload IDs return S3 errors.

2. Add S3 GET backend corruption/failure regression coverage.
   Acceptance: a corrupted or unavailable chunk must not look like a successful full-object download to an S3 client. Current handler code writes `200 OK` before `manager.StreamFile` returns, so this likely needs Software Engineer Head review before QA can write a stable expected-behavior test.

3. Add ListObjectsV2 and prefix/delimiter/pagination E2E once that implementation lands.
   Acceptance: SDK-style `list-type=2` requests respect `prefix`, `delimiter`, `max-keys`, and continuation tokens, and V1 behavior is not regressed.

4. Add Range GET/HEAD E2E when range support lands.
   Acceptance: valid byte ranges return `206`, `Content-Range`, and partial content; invalid ranges return the expected S3 error; HEAD advertises range support without a body.

5. Add DeleteObjects and CopyObject E2E after those code paths are implemented.
   Acceptance: batch delete is idempotent and reports per-key errors; copy preserves content and expected metadata/ETag behavior.

## Work Completed In This Audit

- Added `TestStreamFileChecksumRejectsTruncatedChunk` to cover short chunk reads under checksum verification.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main already run Go unit tests plus S3-backed Python and Go E2E suites in Woodpecker.

## Verification

Local CPU-heavy validation was intentionally not run. The backend suite should be executed by Woodpecker for this branch.
