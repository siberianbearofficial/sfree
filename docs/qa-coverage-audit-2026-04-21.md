# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, recent `origin/main` changes, and open backend branches as of 2026-04-21 12:16 UTC.

## Recent Inputs

- `origin/main` now includes focused S3 range GET, ListObjectsV2 pagination, checksum verification, S3 GET stream-failure semantics, DeleteObjects support, CopyObject support, and Woodpecker image-pinning changes.
- Merged PR [#180](https://github.com/siberianbearofficial/sfree/pull/180) adds S3 CopyObject support plus Go E2E coverage for same-bucket copy, cross-bucket copy, response XML fields, unsupported metadata replacement, and copied-object survival after deleting the source object.
- Merged PR [#178](https://github.com/siberianbearofficial/sfree/pull/178) adds S3 GET stream-failure semantics and handler regression tests for corrupt chunks before success headers/body are emitted.
- Merged PR [#176](https://github.com/siberianbearofficial/sfree/pull/176) adds S3 DeleteObjects support plus E2E coverage for normal delete, missing-key idempotency, quiet mode, list-after-delete, and oversized XML rejection.
- Open PR [#171](https://github.com/siberianbearofficial/sfree/pull/171) adds CI hardening and additional S3 regression assertions, but overlaps older coverage now present on `origin/main`.
- Open branch `origin/THE-431-copyobject-sdk-coverage` adds aiobotocore-backed CopyObject coverage for same-bucket copy, cross-bucket copy, copied content retrieval, missing-source errors, and unsupported metadata replacement errors.
- Open branch `origin/THE-437-file-manager-cache` refactors file manager source-client caching and adds tests for cache reuse, upload client reuse, and delete client reuse.
- Open branch `origin/THE-435-rest-handler-helpers` extracts REST handler request helpers and adds helper-level unit coverage.
- Open branch `origin/THE-434-router-wiring-split` splits router wiring and adds route-registration unit coverage.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, and failover. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested. S3 ETag, range headers, and basic object headers are checked in E2E. | Weak for user metadata and content type because those are not implemented. |
| Placement logic | Round-robin, weighted selection, and upload failover have unit coverage. | Covered for manager logic. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object and ranged download. Handler tests cover S3 full and ranged GET failures before success headers/body are emitted. | Covered for manager logic and pre-commit S3 HTTP error behavior. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover and resilience wrappers have tests. S3 GET stream-failure handler coverage is merged on `origin/main`. | Covered for upload/retry logic and pre-commit S3 GET failure behavior. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, expired presign, range GET, ListObjectsV2 prefix/delimiter/pagination, DeleteObjects, CopyObject, and multipart lifecycle. Python SDK E2E covers ListObjectsV2, range GET, DeleteObjects, and multipart; `origin/THE-431-copyobject-sdk-coverage` adds SDK CopyObject coverage. | Covered for currently implemented core operations once THE-431 merges; weak for metadata and unsupported/malformed request error shapes. |
| Recent bug regressions | Recent checksum, ListObjectsV2, range GET, multipart, DeleteObjects, CopyObject, S3 GET stream-failure, and file-manager cache work have focused tests or open test branches. Web UI lockfile fix belongs to web UI CI. | Covered for latest backend regressions; cache branch should rely on Woodpecker for validation. |
| Recently changed code paths | Backend CI runs Go unit tests plus Python and Go E2E through Woodpecker for `api-go/**`. | Covered by CI routing. |

## Prioritized Missing Tests

1. Merge SDK-level CopyObject compatibility coverage from `origin/THE-431-copyobject-sdk-coverage`.
   Acceptance: aiobotocore `copy_object` succeeds for same-bucket and cross-bucket copies, copied content is readable with `get_object`, missing source returns a stable S3 XML error, and unsupported metadata replacement remains an intentional S3-compatible error.

2. Add S3 object metadata/header behavior coverage when metadata support is implemented.
   Acceptance: PUT with `Content-Type` and `x-amz-meta-*` either preserves those values through HEAD/GET or returns an intentional S3-compatible unsupported-feature response.

3. Add S3 malformed/unsupported request error-shape coverage at SDK and raw HTTP levels.
   Acceptance: unsupported operations and malformed query combinations return XML S3 errors with stable status codes instead of generic JSON or empty responses.

4. Add SDK-level HEAD and presigned URL coverage if the Python SDK compatibility suite is expected to be the long-term compatibility matrix rather than a focused SDK smoke branch.
   Acceptance: aiobotocore `head_object` exposes stable `ContentLength`, `ETag`, and status metadata; generated presigned GET/PUT URLs work without client-specific signing hacks.

5. Add an integration-style file-manager cache regression only if `THE-437` changes beyond the current helper-level cache tests.
   Acceptance: streaming, ranged streaming, upload, and deletion each reuse one source client per source across repeated chunks without changing chunk ordering, failover, or checksum validation behavior.

## Work Completed In This Audit

- Confirmed `TestS3CompatGetObjectRange` covers valid bounded, open-ended, suffix, invalid, and full-object GET range behavior.
- Confirmed `TestS3CompatListObjectsV2PrefixDelimiterAndPagination` covers V2 prefix filtering, delimiter common prefixes, continuation-token pagination, and V1 delimiter regression behavior.
- Confirmed `TestS3CompatMultipartUploadLifecycle` covers create, upload, re-upload part, list parts, complete, GET completed object, abort cleanup, and missing upload IDs.
- Confirmed merged PR #178 contains focused S3 GET stream-failure tests for full and ranged GETs.
- Confirmed merged PR #176 contains DeleteObjects E2E coverage and XML body size-limit regression coverage.
- Confirmed merged PR #180 contains Go E2E coverage for CopyObject and copied-object survival after source deletion.
- Confirmed the Python SDK suite covers ListObjectsV2 prefix/delimiter/pagination, ranged GetObject, DeleteObjects, and multipart upload flow.
- Confirmed `origin/THE-431-copyobject-sdk-coverage` contains the SDK-level CopyObject coverage previously identified as missing.
- Confirmed `origin/THE-437-file-manager-cache` adds focused unit coverage for client-cache reuse across upload and delete paths; the direct cache test covers factory reuse for repeated source IDs.
- Confirmed REST helper and router split branches include focused unit tests for the extracted request-helper and route-registration behavior.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main run Go unit tests plus S3-backed Python and Go E2E suites in Woodpecker.

## Verification

Local CPU-heavy validation was intentionally not run. The backend suite should be executed by Woodpecker for this branch.
