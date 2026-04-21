# QA Coverage and Regression Audit

Date: 2026-04-21

Scope: `api-go` automated tests, Woodpecker backend validation, recent `origin/main` changes, and open backend PRs as of 2026-04-21 18:55 UTC.

## Recent Inputs

- `origin/main` includes the latest S3 range GET, ListObjectsV2 pagination, checksum verification, multipart checksum preservation, one-file-per-bucket-object enforcement, S3 GET stream-failure semantics, DeleteObjects, CopyObject, source-client factory routing, SigV4 redaction, malformed multipart error-shape coverage, S3 helper unit coverage, object mutation manager extraction, unit-only `api-go` Makefile `test`, and Woodpecker lint enforcement.
- Merged commit `85aac70` (`THE-494`, PR #208) adds pure unit coverage for S3 range parsing, `max-keys`, list pagination, common prefixes, continuation tokens, and `max-keys=0`.
- Merged commit `b53e8ed` (`THE-449`, PR #187) adds `golangci-lint run` to `.woodpecker/api-go.yml` for `api-go/**` PRs and main pushes.
- Merged commit `045a8b6` (`THE-486`) adds malformed and unsupported multipart S3 XML error-shape coverage.
- Merged commit `ea8c5df` (`THE-448`) moves S3 object mutations into `internal/manager/s3_object.go` with focused unit coverage for PUT, CopyObject, DeleteObject, CompleteMultipartUpload cleanup, and failure cleanup.
- Merged commit `9ca6ce2` (`THE-468`) adds SigV4 mismatch redaction tests for header auth and presigned URLs.
- Merged commit `672ac1f` (`THE-459`, PR #194) routes source utility operations through the source-client factory and adds manager tests for client selection, wrapping, inspection, download, and stream lifetime behavior.
- Open PR #214 (`THE-503`) fixes short-read upload chunking and adds a focused `internal/manager/file_test.go` regression for short reads before EOF.
- Open PR #213 (`THE-500`) speeds up Woodpecker by combining web UI validation steps and aligning the API Go E2E Mongo image tag for reuse.
- Open PR #212 (`THE-367`) adds OpenAPI documentation routes and router coverage.
- Open PR #210 (`THE-484`) cleans bucket-owned file metadata, multipart metadata, and unreferenced chunks during bucket deletion. It adds manager unit coverage for successful cleanup and cleanup error propagation.
- Open PR #209 (`THE-493`) makes multipart part replacement atomic. It adds handler/repository coverage for replaced part chunk selection and one-step repository part replacement.
- Open PR #207 (`THE-490`) adds Python SDK `head_object` E2E coverage and refreshes this audit document.
- Open PR #202 (`THE-475`) centralizes source-client construction and keeps streamed source downloads open for callers.
- Open PR #195 (`THE-458`) cleans already uploaded chunks when a later upload/read failure aborts upload, with manager unit coverage for later upload and read failures.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Go S3 E2E covers PUT, LIST, GET, overwrite, DELETE, CopyObject, HEAD, presigned GET/PUT, and multipart lifecycle. Python SDK E2E covers upload/download, overwrite, list, range GET, CopyObject, DeleteObjects, DeleteObject metadata cleanup, and multipart. | Covered for implemented core object flows. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, source-client reuse, failover, cleanup on upload failures in PR #195, checksum-verified streaming, and pending short-read chunk fill behavior in PR #214. | Covered at unit level once PR #195 and PR #214 merge. |
| Metadata integrity | Repository tests cover one authoritative file document per bucket/object name. Manager/object tests cover overwrite and copy cleanup. HEAD returns stable basic headers in Go E2E, and SDK HEAD is pending PR #207. | Covered for file metadata and basic S3 headers; weak for user metadata and content-type because product behavior is not implemented. |
| Placement logic | Round-robin, weighted selection, empty-source behavior, default strategy selection, and failover are covered in manager unit tests. | Covered. |
| Reconstruction/retrieval | Manager tests cover checksum-verified full and ranged streaming, legacy chunks, oversized chunks, truncated chunks, and checksum mismatches. Handler tests cover S3 stream failures before success headers/body are emitted. Go/Python E2E cover full and ranged downloads. | Covered. |
| Corruption detection | SHA-256 mismatch, short chunk, oversized chunk, and range verification paths are unit-tested. | Covered at unit level. |
| Source/backend failure cases | Resilience wrappers cover retry, timeout, circuit breaker, upload body replay, and retry exhaustion. Manager upload failover and pending cleanup-on-failure PR #195 cover source failures. | Covered for retry/failover; pending PR #195 closes cleanup gaps. |
| Critical S3-compatible API behavior | Go E2E covers object lifecycle, auth, presign, HEAD, range GET, ListObjectsV2, DeleteObjects, CopyObject, multipart, and malformed multipart errors. Handler unit tests now cover S3 range and list helper edge cases from `THE-494`. Python SDK E2E covers SDK list/range/copy/delete/multipart and pending SDK HEAD in PR #207. | Strong for implemented API surface; unsupported metadata behavior remains intentionally unresolved. |
| Recent bug regressions | Recent checksum, list, range, multipart checksum, DeleteObjects, CopyObject, S3 stream failure, file-manager cache, upload retry body replay, source factory, object mutation manager, malformed error-shape, S3 helper edge cases, SigV4 redaction, bucket grant route scoping, and short-read chunking work have focused tests or open test branches. | Covered once open PRs merge. |
| Recently changed code paths | `.woodpecker/api-go.yml` runs lint, Go unit tests, Python S3 E2E, and Go S3 E2E on `api-go/**` PRs and main pushes. `make test` remains unit-only so CPU-heavy suites stay in Woodpecker. | Covered for backend changes. |

## Prioritized Missing Tests

1. Add an S3/API integration regression for bucket deletion cleanup after PR #210 merges.
   Acceptance: create a bucket with at least one object and one multipart upload, delete the bucket through the API, then verify the object is no longer listable/downloadable through S3 behavior or file APIs and cleanup failures surface as a non-success response. Prefer Go E2E or a focused integration test in `api-go`; run it in Woodpecker only.

2. Add failure-path coverage for multipart part replacement preserving the previous part when replacement upload persistence fails, if PR #209 does not already assert that exact behavior.
   Acceptance: simulate replacement of part N where the new chunk upload succeeds but repository update fails; the previous part remains selected for completion and newly uploaded replacement chunks are cleaned or reported according to the product contract.

3. Decide product behavior for S3 `Content-Type` and `x-amz-meta-*`, then add tests.
   Acceptance: PUT with `Content-Type` and `x-amz-meta-*` either preserves those values through HEAD/GET and CopyObject COPY, or returns an intentional S3-compatible unsupported-feature response. This needs a product/engineering decision before QA writes the final assertion.

4. Add web UI API-helper unit coverage if `webui/src/shared/api/client.ts` remains the centralized request path.
   Acceptance: API helpers consistently attach credentials, parse JSON and empty responses, map non-2xx responses into the shared error type, and preserve user-visible message fallbacks. This likely requires adding a lightweight frontend unit-test harness; do not use Playwright locally.

5. Add a grant-listing route-scoping regression only if requirements expect symmetry with update/delete grant scoping.
   Acceptance: listing grants for bucket A never exposes grants from bucket B, even when users or fixture setup overlap.

## Work Completed In This Audit

- Refreshed the audit against current `origin/main` and the current open GitHub PR list.
- Confirmed `THE-486` closes the prior malformed/unsupported multipart S3 error-shape gap with `TestS3CompatMalformedUnsupportedMultipartErrors`.
- Confirmed `THE-448` closes the prior manager-only S3 object mutation concern with `TestObjectServicePutObjectUpdatesFileAndDeletesOldChunks`, `TestObjectServiceCopyObjectPreservesChunksAndCleansOverwrittenDestination`, `TestObjectServiceDeleteObjectRemovesMetadataAndUnreferencedChunks`, and `TestObjectServiceCompleteMultipartUploadBuildsFinalFileAndCleansUnusedChunks`.
- Confirmed `THE-468` adds focused SigV4 mismatch redaction tests for header auth and presigned URLs.
- Confirmed `THE-459` adds source-client factory and stream lifetime tests in `internal/manager/file_test.go`.
- Confirmed `THE-449` adds backend lint enforcement to Woodpecker.
- Confirmed `THE-494` closes S3 helper unit gaps with `TestParseObjectRange`, `TestParseListMaxKeys`, `TestBuildListBucketPageDelimiterCommonPrefixes`, `TestBuildListBucketPageContinuationToken`, and `TestBuildListBucketPageMaxKeysZero`.
- Confirmed open PR #214 targets short-read upload chunking with a focused manager regression.
- Confirmed open PR #207 targets SDK-level HEAD coverage.
- Identified bucket deletion cleanup E2E/API integration as the highest-value remaining regression target because PR #210 currently advertises manager unit coverage but no API-level deletion cleanup proof.

## Verification

Local CPU-heavy validation was intentionally not run. No Docker, Playwright, `go test`, or lint commands were executed locally; Woodpecker should run backend validation for branches that change `api-go/**`.
