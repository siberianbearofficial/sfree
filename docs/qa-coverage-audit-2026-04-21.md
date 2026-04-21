# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, recent `origin/main` changes, and open backend PRs as of 2026-04-21 16:25 UTC.

## Recent Inputs

- `origin/main` now includes focused S3 range GET, ListObjectsV2 pagination, checksum verification, multipart checksum preservation, one-file-per-bucket-object enforcement, S3 GET stream-failure semantics, DeleteObjects support, CopyObject support, SDK CopyObject compatibility coverage, bucket grant route scoping, Woodpecker image-pinning, and the unit-only `api-go` Makefile `test` target.
- Merged commit `5c4f0a2` (`THE-467`) scopes bucket grant update/delete mutations to the route bucket and adds integration coverage for cross-bucket grant IDs returning 404 without mutating the grant in its real bucket.
- Merged commit `a5ac19e` (`THE-445`) preserves multipart chunk checksums and adds `TestCompletedMultipartChunksPreservesChecksums`.
- Merged commit `9895536` (`THE-447`) enforces one file document per bucket object name and adds repository integration coverage for unique bucket/name replacement plus legacy index migration.
- Merged PR [#196](https://github.com/siberianbearofficial/sfree/pull/196) makes the `api-go` Makefile test target unit-only and documents that integration/E2E suites belong in Woodpecker.
- Merged PR [#180](https://github.com/siberianbearofficial/sfree/pull/180) adds S3 CopyObject support plus Go E2E coverage for same-bucket copy, cross-bucket copy, response XML fields, unsupported metadata replacement, and copied-object survival after deleting the source object.
- Merged PR [#178](https://github.com/siberianbearofficial/sfree/pull/178) adds S3 GET stream-failure semantics and handler regression tests for corrupt chunks before success headers/body are emitted.
- Merged PR [#176](https://github.com/siberianbearofficial/sfree/pull/176) adds S3 DeleteObjects support plus E2E coverage for normal delete, missing-key idempotency, quiet mode, list-after-delete, and oversized XML rejection.
- Open PR [#202](https://github.com/siberianbearofficial/sfree/pull/202) centralizes source-client construction and adds unit coverage for source-client parsing, wrapping, inspection, download, and stream lifetime behavior.
- Open PR [#197](https://github.com/siberianbearofficial/sfree/pull/197) redacts SigV4 mismatch diagnostics and adds header-auth plus presigned-url tests that assert submitted signatures, expected signatures, canonical headers, and credentials do not leak in errors.
- Open PR [#195](https://github.com/siberianbearofficial/sfree/pull/195) fixes cleanup of chunks uploaded before later upload/read failures and adds manager regressions for both paths.
- Open PR [#191](https://github.com/siberianbearofficial/sfree/pull/191) moves S3 object mutation logic into a manager service and adds unit coverage for PUT, CopyObject, DeleteObject, CompleteMultipartUpload cleanup, and metadata-save failure cleanup.
- Open PR [#187](https://github.com/siberianbearofficial/sfree/pull/187) adds `api-go` lint enforcement to Woodpecker and updates CI docs.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, and failover. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested, including completed multipart checksum propagation. S3 ETag, range headers, and basic object headers are checked in E2E. | Weak for user metadata and content type because those are not implemented. |
| Placement logic | Round-robin, weighted selection, and upload failover have unit coverage. | Covered for manager logic. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object and ranged download. Handler tests cover S3 full and ranged GET failures before success headers/body are emitted. | Covered for manager logic and pre-commit S3 HTTP error behavior. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover and resilience wrappers have tests. S3 GET stream-failure handler coverage is merged on `origin/main`; open PR #202 adds source-client construction and stream lifetime coverage. | Covered for upload/retry logic and pre-commit S3 GET failure behavior. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, expired presign, range GET, ListObjectsV2 prefix/delimiter/pagination, DeleteObjects, CopyObject, and multipart lifecycle. Python SDK E2E covers ListObjectsV2, range GET, DeleteObjects, CopyObject, and multipart. | Covered for currently implemented core operations; weak for metadata and unsupported/malformed request error shapes. |
| Recent bug regressions | Recent checksum, ListObjectsV2, range GET, multipart checksum propagation, DeleteObjects, CopyObject, S3 GET stream-failure, file-manager cache, upload retry body replay, upload failure cleanup, source-client factory, one-file-per-object enforcement, and bucket grant route scoping work have focused tests or open test branches. Web UI lockfile fix belongs to web UI CI. | Covered for latest backend regressions once open PRs merge. |
| Recently changed code paths | Backend CI runs Go unit tests plus Python and Go E2E through Woodpecker for `api-go/**`; `make test` is unit-only on `origin/main`, keeping CPU-heavy integration/E2E work in CI. Open PR #187 adds lint enforcement to the same backend Woodpecker path. | Covered by CI routing. |

## Prioritized Missing Tests

1. Add S3 object metadata/header behavior coverage when metadata support is implemented.
   Acceptance: PUT with `Content-Type` and `x-amz-meta-*` either preserves those values through HEAD/GET or returns an intentional S3-compatible unsupported-feature response.

2. Add S3 malformed/unsupported request error-shape coverage at SDK and raw HTTP levels.
   Acceptance: unsupported operations and malformed query combinations return XML S3 errors with stable status codes instead of generic JSON or empty responses.

3. Add SDK-level HEAD and presigned URL coverage if the Python SDK compatibility suite is expected to be the long-term compatibility matrix rather than a focused SDK smoke branch.
   Acceptance: aiobotocore `head_object` exposes stable `ContentLength`, `ETag`, and status metadata; generated presigned GET/PUT URLs work without client-specific signing hacks.

4. After PR #191 merges, add one handler-level or E2E overwrite regression if manager-only coverage proves insufficient.
   Acceptance: repeated S3 PUT, CopyObject over an existing destination, and CompleteMultipartUpload over an existing key leave exactly one file document, preserve retrievable latest content, and clean only chunks no longer referenced by any file.

5. Add a lightweight permission regression for grant listing only if product requirements expect route-bucket scoping symmetry beyond update/delete.
   Acceptance: listing grants for bucket A never exposes grants from bucket B even when grant IDs or users overlap in setup fixtures.

## Work Completed In This Audit

- Confirmed `TestS3CompatGetObjectRange` covers valid bounded, open-ended, suffix, invalid, and full-object GET range behavior.
- Confirmed `TestS3CompatListObjectsV2PrefixDelimiterAndPagination` covers V2 prefix filtering, delimiter common prefixes, continuation-token pagination, and V1 delimiter regression behavior.
- Confirmed `TestS3CompatMultipartUploadLifecycle` covers create, upload, re-upload part, list parts, complete, GET completed object, abort cleanup, and missing upload IDs.
- Confirmed merged PR #178 contains focused S3 GET stream-failure tests for full and ranged GETs.
- Confirmed merged PR #176 contains DeleteObjects E2E coverage and XML body size-limit regression coverage.
- Confirmed merged PR #180 contains Go E2E coverage for CopyObject and copied-object survival after source deletion.
- Confirmed the Python SDK suite covers ListObjectsV2 prefix/delimiter/pagination, ranged GetObject, DeleteObjects, CopyObject, and multipart upload flow.
- Confirmed `origin/THE-437-file-manager-cache` adds focused unit coverage for client-cache reuse across upload and delete paths; the direct cache test covers factory reuse for repeated source IDs.
- Confirmed REST helper and router split branches include focused unit tests for the extracted request-helper and route-registration behavior.
- Confirmed `TestCompletedMultipartChunksPreservesChecksums` covers checksum preservation when completed multipart parts are flattened into final file chunks.
- Confirmed `TestFileRepositoryUniqueBucketNameAndReplace` and `TestFileRepositoryMigratesLegacyBucketNameIndex` cover one authoritative file document per bucket/object name and legacy index migration.
- Confirmed `TestUpdateGrantRejectsCrossBucketGrantID` and `TestDeleteGrantRejectsCrossBucketGrantID` cover the latest bucket grant route-scoping regression.
- Confirmed open PR #195 adds manager coverage for uploaded chunk cleanup after both later upload failure and later read failure.
- Confirmed open PR #197 adds redaction coverage for both header-auth and presigned-url SigV4 mismatch paths.
- Confirmed open PR #191 adds object-service unit coverage for overwrite cleanup and metadata-save failure cleanup around S3 object mutations.
- Confirmed open PR #202 adds focused source-client construction coverage and keeps streamed source downloads open for callers.
- Confirmed `origin/main` now has `make test` scoped to unit tests, matching the QA instruction to leave CPU-heavy suites to Woodpecker.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main run Go unit tests plus S3-backed Python and Go E2E suites in Woodpecker.

## Verification

Local CPU-heavy validation was intentionally not run. The backend suite should be executed by Woodpecker for this branch.
