# QA Coverage and Regression Audit

Date: 2026-04-23

Scope: `api-go` automated tests, Woodpecker validation, recent `origin/main` changes, and open backend/web UI PRs as of 2026-04-23 12:05 UTC.

## Recent Inputs

- `origin/main` now includes focused coverage for S3 range GET, ListObjectsV2 prefix/delimiter/pagination, DeleteObjects, CopyObject, multipart lifecycle, metadata persistence, canonical S3 object ETags, chunk reference indexes, checksum verification, weighted placement and failover, missing bucket-source resolution, upload cleanup, source health capability handling, owned-source helper routing, source provider config validation, escaped source download keys, source key metadata listing, source-client resilience wiring, REST/share/source download preflight behavior, bucket detail API, bucket grant lookup errors, bucket role web UI actions, web UI download failure handling, CLI helpers, OpenAPI docs freshness, graceful API shutdown, Woodpecker smoke hardening, and Woodpecker web UI split validation.
- Recent merged changes since the prior audit include THE-591 missing bucket-source upload handling, THE-585 multipart abort cleanup failures, THE-653 bucket detail API, THE-608 escaped source download keys, THE-697 graceful API shutdown, THE-706 source health capabilities, THE-643 owned source lookup helper, THE-725 Woodpecker smoke image pull hardening, THE-730 bucket deletion cleanup ordering, THE-739 create-bucket source-load failure UI handling, THE-747 canonical S3 ETags, THE-753 chunk reference Mongo indexes, THE-648 web UI audit threshold, and THE-495 empty upload name on error.
- Open PR [#312](https://github.com/siberianbearofficial/sfree/pull/312) fixes an `api-go` server time import; API and smoke checks were pending during this audit.
- Open PR [#311](https://github.com/siberianbearofficial/sfree/pull/311) adds missing-source upload handler regressions; API and smoke checks were pending during this audit.
- Open PR [#310](https://github.com/siberianbearofficial/sfree/pull/310) reduces main branch CI load; API and smoke checks were failing while web UI passed during this audit.
- Open PR [#308](https://github.com/siberianbearofficial/sfree/pull/308) rejects duplicate bucket source IDs; API and smoke checks were pending during this audit.
- Open PR [#302](https://github.com/siberianbearofficial/sfree/pull/302) extracts shared file-size calculation with focused unit coverage; API and smoke checks were pending.
- Open PR [#298](https://github.com/siberianbearofficial/sfree/pull/298) avoids duplicate download preflight reads; API and smoke checks were pending.
- Open PR [#293](https://github.com/siberianbearofficial/sfree/pull/293) extracts a handler test harness; API and smoke checks were pending.
- Open PR [#292](https://github.com/siberianbearofficial/sfree/pull/292) removes router constructor globals; API and smoke checks were green.
- Open PR [#290](https://github.com/siberianbearofficial/sfree/pull/290) cascades share-link cleanup on delete; API and smoke checks were failing during this audit.
- Open PR [#286](https://github.com/siberianbearofficial/sfree/pull/286) shows bucket grant loading failures; smoke was green and web UI was pending.
- Open PR [#285](https://github.com/siberianbearofficial/sfree/pull/285) surfaces bucket grant cleanup failures; API and smoke checks were green.
- Open PR [#281](https://github.com/siberianbearofficial/sfree/pull/281) makes `ObjectService` construction operation-specific; API and smoke checks were failing during this audit.
- Open PR [#279](https://github.com/siberianbearofficial/sfree/pull/279) splits S3 object handler helpers; API and smoke checks were pending.

## Coverage Map

| Area | Current coverage | Assessment |
| --- | --- | --- |
| Object write/read roundtrip | Python E2E and Go S3 E2E cover PUT, LIST, GET, overwrite, DELETE, HEAD, range GET, CopyObject, DeleteObjects, and multipart with an S3 source. Bucket detail coverage verifies REST-side object listing through the new detail route. | Covered for core S3 and REST-backed object flows. |
| Chunking correctness | `internal/manager/file_test.go` covers round-robin chunking, weighted placement, checksum storage, failover, short-read chunk filling, and corruption-oriented range reads. New chunk reference indexes are tracked on main for lookup performance and cleanup support. | Covered at unit level; index behavior is present but should stay covered by repository migration/index tests if index definitions change. |
| Metadata integrity | Repository, manager, handler, Go E2E, and Python SDK E2E coverage verify `Content-Type`, `x-amz-meta-*`, canonical object ETags, overwrite replacement, CopyObject preservation, multipart completion metadata, and completed multipart checksum propagation. | Covered for implemented metadata and ETag behavior; tags, checksum headers, conditionals, and response header overrides remain unsupported compatibility gaps. |
| Placement logic | Bucket validation tests cover malformed source weights; manager tests cover round-robin, weighted selection, weighted failover, and missing-source resolution. Open PR #308 targets duplicate source ID rejection. | Covered for current placement logic once duplicate-source validation lands. |
| Reconstruction/retrieval | Manager tests cover checksum-verified streaming, range streaming across chunks, legacy chunks, oversized chunks, truncated chunks, corruption, and preflight behavior. Public handler tests cover S3, REST, shared-link, direct-source preflight, and escaped source download keys. | Covered for known retrieval regressions. |
| Corruption detection | SHA-256 mismatch paths, short chunks, oversized chunks, range corruption, canonical ETag changes, and stream-failure-before-header paths are covered. | Covered at manager, handler, and E2E levels. |
| Source/backend failure cases | Source health, provider config validation, upload retry body replay, cleanup after later upload/read failures, download context lifetime, direct source download preflight, missing bucket-source resolution, source-client factory wiring, and owned-source helper behavior have focused tests. | Strong coverage for recent backend failure regressions; PR #311 is the remaining high-value public handler layer for missing-source upload error shape. |
| Critical S3-compatible API behavior | Go E2E covers auth, presigned GET/PUT, expired presign, object lifecycle, range GET, listing, DeleteObjects, CopyObject, multipart lifecycle, malformed multipart errors, missing-object XML errors, cross-bucket credential isolation, metadata, and canonical ETags. Python SDK E2E covers ListObjectsV2, range GET, DeleteObjects, CopyObject, multipart, HEAD, and metadata. | Covered for implemented operations; unsupported/malformed request combinations and live-client matrix coverage remain selective. |
| Recent bug regressions | Recent fixes for SigV4 raw query canonicalization, object key parsing, metadata persistence, canonical ETags, failover, source key listing, source config validation, source health capabilities, escaped source download keys, context lifetime, duplicate preflight reads, bucket grant lookup errors, source download preflight, bucket deletion ordering, file-size calculation, missing-source resolution, and upload-name-on-error either have merged tests or open PR tests. | Covered or tracked by open PRs. |
| Recently changed code paths | `.woodpecker/api-go.yml` runs lint, Go tests, generated docs freshness, Python E2E, and Go S3 E2E for `api-go/**`. `.woodpecker/webui.yml` separates lint/build validation from Playwright E2E. Smoke remains Woodpecker-only and was recently hardened for image pulls. | CI routing matches the QA constraint to keep CPU-heavy suites in Woodpecker. |

## Prioritized Missing Tests

1. Finish public handler regressions for missing bucket-source upload failures in PR #311.
   Acceptance: REST upload, S3 PutObject, and S3 UploadPart requests against a bucket whose `source_ids` include a deleted or missing source return stable client errors (`400` or S3 XML `InvalidRequest`) without attempting chunk upload, and missing-source resolution does not leak as a generic `500`.

2. Add duplicate bucket source ID validation coverage in PR #308.
   Acceptance: REST bucket create/update rejects duplicate source IDs before persistence, weighted source maps cannot mask duplicates, and the response is a stable `400` with no partial bucket mutation.

3. Expand raw S3 error-shape coverage for unsupported and malformed requests.
   Acceptance: missing bucket, missing upload, invalid range, unsupported operation, bad copy source, malformed query, and auth mismatch paths return S3 XML errors with stable status codes instead of JSON, empty bodies, or generic server errors.

4. Add REST bucket-delete integration coverage if REST route semantics are user-visible.
   Acceptance: deleting a bucket with completed objects, share links, grants, and incomplete multipart uploads through the REST route returns the expected HTTP status, removes the bucket, and leaves no file, multipart, grant, or share-link residue in repository-backed checks. S3 bucket deletion and manager cleanup ordering already have coverage.

5. Add a small live-client compatibility smoke matrix in Woodpecker only if SFree intends to claim CLI compatibility beyond SDK compatibility.
   Acceptance: AWS CLI, rclone, or MinIO client tests prove direct-bucket list, put, get, copy, and recursive delete/sync behavior for the subset documented in `docs/s3-compatibility.md`; failures are reported as compatibility gaps rather than blocking unsupported features.

6. Add grant-listing scope coverage only if requirements expect route-bucket symmetry beyond update/delete.
   Acceptance: listing grants for bucket A never exposes grants from bucket B, even when users or grant IDs overlap in fixtures.

7. Keep one public-route overwrite/delete regression for every cleanup path that manager-only coverage cannot prove.
   Acceptance: repeated S3 PUT, REST upload over an existing file, CopyObject over an existing destination, CompleteMultipartUpload over an existing key, and bucket delete leave exactly the expected file/multipart records and clean only chunks no longer referenced by any file or multipart upload.

8. Add advanced metadata compatibility coverage only when product support expands.
   Acceptance: supported checksum headers, object tags, response header overrides, conditional requests, or `x-amz-metadata-directive: REPLACE` each get one SDK or raw HTTP regression when implemented; unsupported metadata features return stable S3 XML errors.

## Work Completed In This Audit

- Rebased PR #306 onto current `origin/main` after GitHub reported `mergeStateStatus: DIRTY`.
- Resolved the audit document conflict by refreshing the audit against current main and current open PR state.
- Reclassified newly merged coverage for missing bucket-source resolution, bucket detail API, source health capabilities, owned-source helper routing, escaped source downloads, canonical S3 ETags, chunk reference indexes, graceful shutdown, smoke hardening, and upload-name-on-error.
- Reviewed open PR check state with `gh pr list`; recorded currently pending, green, and failing Woodpecker contexts relevant to QA risk.
- Confirmed no CPU-heavy local test, Docker, or Playwright command was run.

## Verification

Local CPU-heavy validation was intentionally not run. This docs-only audit should be reviewed and validated by Woodpecker on PR #306.
