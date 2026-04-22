# QA Coverage and Regression Audit

Date: 2026-04-22

Scope: `api-go` automated tests, Woodpecker validation, recent `origin/main` changes, and open backend/web UI PRs as of 2026-04-22 11:25 UTC.

## Recent Inputs

- `origin/main` now includes focused S3 range GET, ListObjectsV2 pagination, checksum verification, multipart checksum preservation, atomic multipart part replacement cleanup, one-file-per-bucket-object enforcement, S3 GET stream-failure semantics, DeleteObjects support, CopyObject support, SDK CopyObject and HEAD compatibility coverage, bucket grant route scoping, S3 helper coverage, multipart malformed error-shape coverage, SigV4 redaction coverage, SigV4 payload-hash bounds, upload-failure cleanup coverage, short-read upload chunking coverage, weighted source validation, bucket-content cleanup on delete, S3 missing-object and cross-bucket credential-isolation E2E coverage, unsupported CopyObject metadata-directive XML error coverage, manager-level S3 object mutation coverage, REST lifecycle routing through the shared object service, REST/share download preflight coverage, direct source download preflight coverage, S3 bucket deletion cleanup E2E coverage, deterministic S3 Go E2E readiness/expired-presign coverage, manager chunk I/O split coverage continuity, web UI download-failure E2E coverage, web UI role-action E2E coverage, share download proxy smoke coverage, OpenAPI docs route coverage, Woodpecker image-pinning, split web UI Woodpecker gates, lint enforcement, Woodpecker E2E image-pull speedups, and the unit-only `api-go` Makefile `test` target.
- Open PR [#261](https://github.com/siberianbearofficial/sfree/pull/261) validates source provider configs and adds handler/unit client coverage for S3 and Telegram configuration paths.
- Open PR [#277](https://github.com/siberianbearofficial/sfree/pull/277) remediates the `api-go` govulncheck baseline; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#276](https://github.com/siberianbearofficial/sfree/pull/276) validates malformed share-link request bodies; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#275](https://github.com/siberianbearofficial/sfree/pull/275) removes dead multipart handler helpers; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#274](https://github.com/siberianbearofficial/sfree/pull/274) adds a range corruption regression test; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#273](https://github.com/siberianbearofficial/sfree/pull/273) handles escaped source download keys; Woodpecker API, smoke, and web UI checks were pending during this refresh.
- Open PR [#272](https://github.com/siberianbearofficial/sfree/pull/272) adds a bucket detail API; Woodpecker API was failing and smoke/web UI checks were pending during this refresh.
- Open PR [#271](https://github.com/siberianbearofficial/sfree/pull/271) and [#269](https://github.com/siberianbearofficial/sfree/pull/269) are parallel QA audit refresh branches; they should be reconciled before relying on either as the latest audit artifact.
- Open PR [#270](https://github.com/siberianbearofficial/sfree/pull/270) rejects malformed share-link bodies; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#268](https://github.com/siberianbearofficial/sfree/pull/268) aligns S3 compatibility evidence; smoke was green during this refresh.
- Open PR [#267](https://github.com/siberianbearofficial/sfree/pull/267) regenerates source-health docs; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#266](https://github.com/siberianbearofficial/sfree/pull/266) makes served API docs use generated docs and was merge-conflicted during this refresh.
- Open PR [#265](https://github.com/siberianbearofficial/sfree/pull/265) removes timing sleeps from Go tests; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#264](https://github.com/siberianbearofficial/sfree/pull/264) requires bucket ownership checks before aborting multipart uploads; Woodpecker API and smoke checks were pending during this refresh.
- Merged commit `350dd9c` (`THE-564`) replaces fixed sleeps in S3 Go E2E readiness and expired-presign coverage with bounded Mongo readiness retry and deterministic expired presign signing.
- Merged commit `a29ffd9` (`THE-561`) adds Go S3 E2E coverage proving unsupported CopyObject `x-amz-metadata-directive: REPLACE` returns a stable XML `NotImplemented` error.
- Merged PR [#238](https://github.com/siberianbearofficial/sfree/pull/238) routes REST file lifecycle mutations through the shared object service and adds manager coverage for REST delete cleanup, bucket-content cleanup, and cleanup-failure propagation.
- Merged PR [#237](https://github.com/siberianbearofficial/sfree/pull/237) adds Go S3 E2E coverage proving bucket deletion cleans object files, multipart uploads, and chunks.
- Merged PR [#220](https://github.com/siberianbearofficial/sfree/pull/220) adds preflight coverage for REST file downloads and shared downloads so stream failures return errors before success headers.
- Merged PR [#228](https://github.com/siberianbearofficial/sfree/pull/228) adds an S3/API testing plan plus Go S3 E2E coverage for missing-object `NoSuchKey` XML behavior and cross-bucket credential isolation.
- Merged PR [#234](https://github.com/siberianbearofficial/sfree/pull/234) adds source health APIs and UI status display with handler, manager, provider-client, and Telegram/S3 client unit coverage for healthy, degraded, unsupported, missing, and invalid-source paths.
- Merged PR [#222](https://github.com/siberianbearofficial/sfree/pull/222) adds generated OpenAPI docs freshness validation to Woodpecker.
- Merged commit `d90b94c` (`THE-484`) cleans bucket contents on delete and adds manager regressions for object chunk cleanup, multipart part cleanup, cleanup-failure behavior, and repository delete helpers.
- Merged commit `6914409` (`THE-552`) validates weighted source weights and adds handler validation tests plus selector coverage.
- Merged commit `e2a3aac` (`THE-542`) bounds SigV4 validator payload hashing and adds tests for explicit payload hashes, missing payload hashes, unknown-length non-empty bodies, and unknown-length empty bodies.
- Merged PR [#233](https://github.com/siberianbearofficial/sfree/pull/233) adds REST bucket file filename search with repository and handler integration coverage for bucket scoping, case-insensitive matching, literal regex-character handling, blank-query fallback, and grant access.
- Merged PR [#221](https://github.com/siberianbearofficial/sfree/pull/221) shares S3 bucket/object lookup helpers while preserving missing bucket, wrong access key, missing object, and GET stream-failure preflight coverage.
- Merged PR [#225](https://github.com/siberianbearofficial/sfree/pull/225) refreshed this QA audit around SigV4 body-hash buffering, REST/S3/OpenAPI test branches, and the remaining S3 auth/error-shape gap.
- Merged PR [#209](https://github.com/siberianbearofficial/sfree/pull/209) makes multipart part replacement cleanup atomic and adds repository and handler coverage for replacement behavior.
- Merged PR [#219](https://github.com/siberianbearofficial/sfree/pull/219) adds Go S3 E2E assertions for DeleteObjects oversized and malformed XML error codes.
- Merged PR [#215](https://github.com/siberianbearofficial/sfree/pull/215) extracts S3 listing handlers while preserving existing handler and Go E2E coverage for delimiter common prefixes, prefix filtering, continuation-token pagination, and V1 delimiter behavior.
- Merged PR [#214](https://github.com/siberianbearofficial/sfree/pull/214) fixes short-read upload chunking and adds `TestUploadFileChunksFillsChunksAcrossShortReads`.
- Merged PR [#212](https://github.com/siberianbearofficial/sfree/pull/212) adds an OpenAPI docs route with router coverage for the generated spec endpoint.
- Merged PR [#208](https://github.com/siberianbearofficial/sfree/pull/208) adds S3 helper unit coverage for query parsing, bucket-name validation, metadata extraction, error XML, request signing context, and object key trimming.
- Merged PR [#207](https://github.com/siberianbearofficial/sfree/pull/207) adds Python SDK `head_object` coverage.
- Merged PR [#187](https://github.com/siberianbearofficial/sfree/pull/187) adds `api-go` lint enforcement to Woodpecker and updates CI docs.
- Merged commit `49cc4d5` (`THE-500`) speeds Woodpecker validation by using pre-pulled/pinned E2E images and removing web UI dependency installs where unnecessary.
- Merged commit `6589a19` (`THE-458`) cleans uploaded chunks after later upload/read failures and adds manager regressions for both paths.
- Merged commit `7ad432d` (`THE-513`) makes router setup fail on repository dependency initialization errors and adds unit/integration coverage for nil-Mongo route registration and dependency errors.
- Merged commit `045a8b6` (`THE-486`) adds Go E2E coverage for malformed and unsupported multipart request error shapes.
- Merged commit `ea8c5df` (`THE-448`) moves S3 object mutation logic into a manager service and adds unit coverage for PUT, CopyObject, DeleteObject, CompleteMultipartUpload cleanup, and metadata-save failure cleanup.
- Merged commit `9ca6ce2` (`THE-468`) redacts SigV4 mismatch diagnostics and adds header-auth plus presigned-url tests that assert submitted signatures, expected signatures, canonical headers, and credentials do not leak in errors.
- Merged commit `672ac1f` (`THE-459`) routes source utilities through the source-client factory and adds manager coverage for source-client parsing, wrapping, inspection, download, upload retry body replay, and stream lifetime behavior.
- Merged commit `5c4f0a2` (`THE-467`) scopes bucket grant update/delete mutations to the route bucket and adds integration coverage for cross-bucket grant IDs returning 404 without mutating the grant in its real bucket.
- Merged commit `a5ac19e` (`THE-445`) preserves multipart chunk checksums and adds `TestCompletedMultipartChunksPreservesChecksums`.
- Merged commit `9895536` (`THE-447`) enforces one file document per bucket object name and adds repository integration coverage for unique bucket/name replacement plus legacy index migration.
- Merged PR [#196](https://github.com/siberianbearofficial/sfree/pull/196) makes the `api-go` Makefile test target unit-only and documents that integration/E2E suites belong in Woodpecker.
- Merged PR [#180](https://github.com/siberianbearofficial/sfree/pull/180) adds S3 CopyObject support plus Go E2E coverage for same-bucket copy, cross-bucket copy, response XML fields, unsupported metadata replacement, and copied-object survival after deleting the source object.
- Merged PR [#178](https://github.com/siberianbearofficial/sfree/pull/178) adds S3 GET stream-failure semantics and handler regression tests for corrupt chunks before success headers/body are emitted.
- Merged PR [#176](https://github.com/siberianbearofficial/sfree/pull/176) adds S3 DeleteObjects support plus E2E coverage for normal delete, missing-key idempotency, quiet mode, list-after-delete, and oversized XML rejection.
- Open PR [#231](https://github.com/siberianbearofficial/sfree/pull/231) implements minimal S3 object metadata persistence and adds handler, manager, repository, Go S3 E2E, and Python SDK E2E coverage for Content-Type and user metadata.
- Open PR [#241](https://github.com/siberianbearofficial/sfree/pull/241) keeps wrapped download contexts alive until response body close and adds unit coverage for successful stream reads before close plus timeout-before-body behavior.
- Open PR [#254](https://github.com/siberianbearofficial/sfree/pull/254) drops returned upload names on wrapped upload errors and adds `TestWrapperUploadErrorDropsReturnedName`.
- Open PR [#253](https://github.com/siberianbearofficial/sfree/pull/253) adds dependency audit gates for backend and web UI Woodpecker validation.
- Open PR [#258](https://github.com/siberianbearofficial/sfree/pull/258) preserves multipart abort cleanup failures; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#257](https://github.com/siberianbearofficial/sfree/pull/257) adds focused CLI helper coverage for local command helpers; Woodpecker API and smoke checks were pending during this refresh.
- Open PR [#251](https://github.com/siberianbearofficial/sfree/pull/251) fixes SigV4 raw query canonicalization with backend and smoke checks pending.
- Open PR [#246](https://github.com/siberianbearofficial/sfree/pull/246) consolidates legacy Swagger routes onto the canonical OpenAPI docs surface and adds router coverage for redirect behavior.
- Open PR [#245](https://github.com/siberianbearofficial/sfree/pull/245) fails uploads when buckets reference missing sources and adds repository ordering/missing-source coverage plus manager-level partial-source rejection coverage.
- Open PR [#243](https://github.com/siberianbearofficial/sfree/pull/243) routes REST uploads through `ObjectService` and adds manager coverage for source validation, no-source short-circuiting, and distribution strategy selection.
- Open PR [#239](https://github.com/siberianbearofficial/sfree/pull/239) surfaces bucket grant lookup errors and adds permission middleware coverage for owners, missing grants, valid grants, and lookup failures.
- Open PR [#235](https://github.com/siberianbearofficial/sfree/pull/235) moves rate limiting to route-aware public, protected, and S3 middleware and adds unit/router coverage for authenticated identity selection, unauthenticated fallback, route wiring, and limiter configuration.
- Open PR [#227](https://github.com/siberianbearofficial/sfree/pull/227) fixes S3 multipart part replacement cleanup order with manager tests that assert metadata replacement happens before old-chunk deletion and failed metadata replacement cleans only new chunks.
- Open PR [#224](https://github.com/siberianbearofficial/sfree/pull/224) routes REST bucket file mutations through the object service and adds handler coverage for overwrite routing and delete cleanup-failure propagation.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE with an S3 source. | Covered for simple objects. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, failover, and short-read chunk filling. | Covered at unit level. |
| Metadata integrity | File chunk size/checksum metadata is unit-tested, including completed multipart checksum propagation. S3 ETag, range headers, and basic object headers are checked in E2E. PR #231 adds Content-Type and `x-amz-meta-*` persistence coverage across handlers, manager, repository, Go S3 E2E, and Python SDK E2E. | Covered for minimal metadata once PR #231 merges; broader tags/checksum headers/metadata-directive REPLACE behavior remains out of scope. |
| Placement logic | Round-robin, weighted selection, upload failover, and weighted source request validation have unit/handler coverage. | Covered for manager logic and request validation. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, and corruption. S3 E2E covers full-object and ranged download. Handler tests cover S3 full and ranged GET failures before success headers/body are emitted. | Covered for manager logic and pre-commit S3 HTTP error behavior. |
| Corruption detection | SHA-256 mismatch paths are unit-tested, including short and oversized chunk reads. | Covered at unit level. |
| Source/backend failure cases | Manager failover, resilience wrappers, upload retry body replay, uploaded-chunk cleanup after later failures, S3 GET stream-failure handler behavior, REST/share/direct-source download preflight behavior, and source health paths have tests. Open PR #241 adds direct download-context lifetime regressions. Open PR #245 adds missing-source repository and manager coverage. | Covered for upload/retry/cleanup logic, pre-commit S3/REST/shared/direct-source download failure behavior, and source health; missing-source handler status/error-shape coverage remains open. |
| Critical S3-compatible API behavior | Go E2E covers source/bucket creation, object lifecycle, auth failure, presigned GET/PUT, HEAD, expired presign, range GET, ListObjectsV2 prefix/delimiter/pagination, DeleteObjects, CopyObject, unsupported CopyObject metadata-directive XML errors, missing-object XML errors, cross-bucket credential isolation, multipart lifecycle, malformed multipart errors, and DeleteObjects malformed/oversized error codes. PR #231 adds metadata-header E2E coverage. Python SDK E2E covers ListObjectsV2, range GET, DeleteObjects, CopyObject, multipart, HEAD, and metadata once PR #231 merges. | Covered for currently implemented core operations once PR #231 merges; still weak for broader unsupported/malformed request combinations. |
| Recent bug regressions | Recent checksum, ListObjectsV2, range GET, multipart checksum propagation, DeleteObjects, CopyObject, S3 GET stream-failure, REST/share/direct-source download preflight, SigV4 body-hash buffering, SigV4 payload-hash bounds, file-manager cache, upload retry body replay, upload failure cleanup, source-client factory, one-file-per-object enforcement, bucket grant route scoping, bucket grant lookup errors, short-read chunking, download context lifetime, multipart replacement atomicity, bucket delete cleanup, REST lifecycle cleanup routing, filename search, route-aware rate limiting, source health, web UI download failures, web UI role actions, share download proxying, and multipart replacement cleanup-order work have focused tests or open test branches. | Covered for latest merged regressions; open branch coverage still depends on Woodpecker results and merge. |
| Recently changed code paths | Backend CI runs lint, Go unit tests, generated docs freshness, Python E2E, and Go E2E through Woodpecker for `api-go/**`; `make test` is unit-only on `origin/main`, keeping CPU-heavy integration/E2E work in CI. Web UI CI now has split lint/build and Playwright gates through Woodpecker for `webui/**`. THE-564's S3 E2E readiness/presign timing paths, THE-561's CopyObject unsupported metadata-directive path, PR #238's REST lifecycle routing, PR #237's S3 bucket deletion cleanup, PR #220's download preflight paths, PR #249's direct source download preflight paths, PR #250's chunk I/O split, PR #252's share proxy path, PR #260's web UI CI split, PR #262's download-failure UI paths, PR #244's role-action UI paths, PR #234's source health paths, PR #222's OpenAPI freshness check, PR #233's filename search paths, PR #228's S3 missing-object/auth paths, THE-484's bucket cleanup paths, THE-552's weighted validation paths, and THE-542's SigV4 body-hash paths are already covered on main. | Covered by CI routing for merged paths; several open PRs still show pending or failing Woodpecker checks and should not be treated as landed coverage yet. |

## Prioritized Missing Tests

1. Add public handler regressions for missing bucket-source upload failures before PR #245 merges.
   Acceptance: REST upload, S3 PutObject, and S3 UploadPart requests against a bucket whose `source_ids` include a deleted/missing source return stable client errors (`400`/S3 XML `InvalidRequest`) without attempting chunk upload, and repository/manager missing-source errors do not leak as generic `500` responses.

2. Add source provider config validation matrix coverage around PR #261 before it merges.
   Acceptance: S3 config validation rejects missing endpoint, missing credentials, and invalid bucket/region combinations with stable `400` responses; Telegram config validation rejects missing bot tokens/chat IDs; valid configs for each provider remain accepted. Tests should live in existing `api-go/internal/handlers` and provider-client unit suites.

3. Expand malformed/unsupported request error-shape coverage at SDK and raw HTTP levels.
   Acceptance: remaining missing-bucket, missing-upload, invalid-range, unsupported-operation, and malformed query combinations return XML S3 errors with stable status codes instead of generic JSON or empty responses.

4. Add focused advanced metadata compatibility coverage only when product support expands beyond PR #231's minimal scope.
   Acceptance: supported checksum headers, object tags, response header overrides, or `x-amz-metadata-directive: REPLACE` each get one SDK or raw-HTTP regression when implemented; unsupported metadata features return stable S3 XML errors.

5. Add SDK-level presigned URL coverage if the Python SDK compatibility suite is expected to be the long-term compatibility matrix rather than a focused SDK smoke branch.
   Acceptance: aiobotocore-generated presigned GET/PUT URLs work without client-specific signing hacks and return stable S3-compatible responses.

6. Keep one public-route overwrite/delete regression for every user-visible cleanup path that manager-only coverage cannot prove.
   Acceptance: repeated S3 PUT, REST upload over an existing file, CopyObject over an existing destination, CompleteMultipartUpload over an existing key, and bucket delete leave exactly the expected file/multipart records, preserve retrievable latest content where applicable, and clean only chunks no longer referenced by any file or multipart upload.

7. Add web UI failure-state coverage for preview/modal paths if PR #262's merged coverage only exercises toolbar download actions.
   Acceptance: bucket-file preview/download and source-file download failures show actionable errors without navigating away, and success paths still close or retain UI state according to existing behavior. Keep this in `webui/e2e/files.spec.ts` or the closest existing Playwright suite and run it only through Woodpecker.

8. Add a lightweight permission regression for grant listing only if product requirements expect route-bucket scoping symmetry beyond update/delete.
   Acceptance: listing grants for bucket A never exposes grants from bucket B even when grant IDs or users overlap in setup fixtures.

9. Add a REST bucket-delete integration regression if users depend on REST route semantics beyond the S3 bucket deletion path now covered by PR #237.
   Acceptance: deleting a bucket with objects and incomplete multipart uploads through the REST route returns the expected HTTP status, removes the bucket, and leaves no retrievable object or multipart residue through repository-backed checks.

## Work Completed In This Audit

- Confirmed `TestS3CompatGetObjectRange` covers valid bounded, open-ended, suffix, invalid, and full-object GET range behavior.
- Confirmed `TestS3CompatListObjectsV2PrefixDelimiterAndPagination` covers V2 prefix filtering, delimiter common prefixes, continuation-token pagination, and V1 delimiter regression behavior.
- Confirmed `TestS3CompatMultipartUploadLifecycle` covers create, upload, re-upload part, list parts, complete, GET completed object, abort cleanup, and missing upload IDs.
- Confirmed merged PR #178 contains focused S3 GET stream-failure tests for full and ranged GETs.
- Confirmed merged PR #176 contains DeleteObjects E2E coverage and XML body size-limit regression coverage.
- Confirmed merged PR #180 contains Go E2E coverage for CopyObject and copied-object survival after source deletion.
- Confirmed the Python SDK suite covers ListObjectsV2 prefix/delimiter/pagination, ranged GetObject, DeleteObjects, CopyObject, multipart upload flow, and HEAD.
- Confirmed `origin/THE-437-file-manager-cache` adds focused unit coverage for client-cache reuse across upload and delete paths; the direct cache test covers factory reuse for repeated source IDs.
- Confirmed REST helper and router split branches include focused unit tests for the extracted request-helper and route-registration behavior.
- Confirmed `TestCompletedMultipartChunksPreservesChecksums` covers checksum preservation when completed multipart parts are flattened into final file chunks.
- Confirmed `TestFileRepositoryUniqueBucketNameAndReplace` and `TestFileRepositoryMigratesLegacyBucketNameIndex` cover one authoritative file document per bucket/object name and legacy index migration.
- Confirmed `TestUpdateGrantRejectsCrossBucketGrantID` and `TestDeleteGrantRejectsCrossBucketGrantID` cover the latest bucket grant route-scoping regression.
- Confirmed merged THE-458 coverage handles uploaded chunk cleanup after both later upload failure and later read failure.
- Confirmed merged SigV4 redaction coverage covers both header-auth and presigned-url mismatch paths.
- Confirmed merged object-service unit coverage covers overwrite cleanup and metadata-save failure cleanup around S3 object mutations.
- Confirmed merged PR #219 adds Go S3 E2E assertions that DeleteObjects oversized and malformed XML requests return stable S3 XML error codes (`InvalidRequest` and `MalformedXML`).
- Confirmed merged PR #214 adds short-read upload chunking coverage for readers that return less than the requested chunk size without EOF.
- Confirmed merged PR #212 adds OpenAPI route coverage for the generated spec endpoint.
- Confirmed merged PR #228 adds an S3/API coverage plan and Go S3 E2E coverage for missing-object `NoSuchKey` XML responses and cross-bucket credential isolation.
- Confirmed merged THE-552 coverage adds handler validation for malformed weighted source configurations and selector coverage for cumulative weighted selection.
- Confirmed merged THE-484 coverage adds manager/repository cleanup coverage for deleting buckets with objects and multipart state.
- Confirmed merged PR #209 adds repository and handler coverage for atomic multipart part replacement cleanup.
- Confirmed open PR #227 adds manager-level coverage for multipart replacement cleanup order and metadata-replacement failure cleanup.
- Confirmed merged PR #207 adds SDK-level `head_object` coverage.
- Confirmed merged PR #215 is a handler extraction with existing list-object handler/E2E coverage still targeting the moved behavior.
- Confirmed `origin/main` now has `make test` scoped to unit tests, matching the QA instruction to leave CPU-heavy suites to Woodpecker.
- Confirmed merged THE-542 coverage adds SigV4 validator tests for explicit payload hashes, missing payload hashes, unknown-length non-empty bodies, and unknown-length empty bodies without full request-body buffering.
- Confirmed PR #224 adds handler-level tests proving REST upload/delete mutations route through the shared object service and surface cleanup failures.
- Confirmed merged PR #222 adds an OpenAPI generated-docs freshness check target in Woodpecker validation.
- Confirmed merged PR #221 is now on `origin/main`, so missing bucket, wrong access key, missing object, and GET stream-failure preflight coverage are no longer only open-branch coverage.
- Confirmed merged THE-564 replaces fixed S3 Go E2E sleeps with a bounded Mongo readiness retry helper and deterministic already-expired presigned URL generation; local E2E execution remains Woodpecker-only.
- Confirmed open PR #231 adds Content-Type and user metadata coverage at handler, manager, repository, Go S3 E2E, and Python SDK E2E levels.
- Confirmed merged PR #233 adds `TestFileRepositoryListByBucketByNameQuery` for case-insensitive filename search, bucket scoping, literal regex-character handling, and blank-query fallback.
- Confirmed merged PR #233 adds `TestListFilesSearchQueryWithGrantAccess` for REST filename search through a granted bucket while preserving 404 behavior for users without bucket access.
- Confirmed open PR #235 adds route-aware rate limit coverage for protected authenticated identity, public IP fallback, S3 credential identity, route registration, and limiter configuration behavior.
- Confirmed merged PR #234 adds source health coverage across handler authorization/validation paths, manager health aggregation, provider health checks, and S3/Telegram health client behavior.
- Confirmed open PR #241 adds resilience wrapper coverage for keeping download contexts alive until body close and returning `DeadlineExceeded` when a download times out before returning a body.
- Confirmed open PR #246 adds router coverage for redirecting `/swagger/index.html` to `/api/docs`.
- Confirmed open PR #245 adds repository and manager coverage for missing bucket-source resolution, but handler-level REST/S3 status and S3 XML error-shape assertions are still missing.
- Confirmed open PR #243 adds manager coverage for REST upload source validation, no-source short-circuiting, and bucket distribution strategy selection through `ObjectService`.
- Confirmed merged PR #244 adds web UI E2E coverage for bucket/file actions exposed to owner, editor, and viewer roles.
- Confirmed merged THE-561 adds a Go S3 E2E assertion that unsupported CopyObject `REPLACE` returns an S3 XML `NotImplemented` error.
- Confirmed open PR #239 adds permission middleware coverage for owner access, missing grants, valid grants, and grant lookup failures.
- Confirmed merged PR #220 adds REST and shared download preflight tests for stream failures before success headers.
- Confirmed merged PR #237 adds Go S3 E2E coverage for deleting buckets with object and multipart residue.
- Confirmed merged PR #238 adds object-service coverage for REST lifecycle routing and cleanup-failure propagation.
- Confirmed merged PR #249 adds direct source download preflight coverage.
- Confirmed merged PR #250 is a file split for manager chunk I/O; existing manager chunking, checksum, range, failover, and cleanup tests still target the moved behavior.
- Confirmed open PR #251 adds SigV4 raw-query canonicalization coverage.
- Confirmed merged PR #252 adds web UI/smoke coverage for frontend-origin share download proxying.
- Confirmed open PR #253 adds dependency audit gates; backend and web UI Woodpecker checks were still pending during this refresh.
- Confirmed open PR #254 adds `TestWrapperUploadErrorDropsReturnedName` for the upload-name-on-error regression.
- Confirmed open PR #257 adds CLI helper unit coverage; API and smoke Woodpecker checks were pending during this refresh.
- Confirmed open PR #258 adds multipart abort cleanup-failure preservation coverage; API and smoke Woodpecker checks were pending during this refresh.
- Confirmed merged PR #260 splits web UI Woodpecker validation into separate lint/build and Playwright gates, improving failure localization without changing the test surface.
- Confirmed open PR #261 adds source provider config validation coverage for handler/client paths; the main remaining risk is matrix completeness around invalid provider-specific fields.
- Confirmed merged PR #262 adds web UI E2E coverage for download failure toasts in bucket/source download flows.
- Confirmed open PR #274 adds focused range corruption regression coverage; Woodpecker API and smoke checks were pending during this refresh.
- Confirmed open PR #277 adds a backend dependency-audit baseline remediation path; Woodpecker API and smoke checks were pending during this refresh.
- Confirmed open PR #272 adds bucket detail API coverage but had a failing Woodpecker API check during this refresh.
- Reviewed `.woodpecker/api-go.yml`; backend PRs and pushes to main run lint, Go unit tests, generated docs freshness, and S3-backed Python and Go E2E suites in Woodpecker.
- Reviewed open PR check state with `gh pr list`; several open branches had pending Woodpecker checks, and PR #272 showed a failing API check during this refresh.

## Verification

Local CPU-heavy validation was intentionally not run. This docs-only refresh should be validated through review and Woodpecker if repository rules emit checks for the branch.
