# S3 Compatibility Matrix

Last updated: 2026-04-24

Baseline: `origin/main` at `ea8c3bcc33b4edc6a0474c70c86b155371aaf773`. SFree exposes its S3-compatible API on the root path using path-style addressing: `/{bucket}/{key}`. The legacy `/api/s3/{bucket}/{key}` alias remains supported for backward compatibility.

Reference: [Amazon S3 API operations](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Operations_Amazon_Simple_Storage_Service.html).

## Summary

SFree supports the core object lifecycle: upload, download, head, copy, single-delete, multi-delete, ListObjectsV2, byte-range downloads, multipart upload, and minimal bucket-discovery probes. The main compatibility gaps are full account-style bucket administration, checksums, and advanced bucket/object APIs.

Validated v0.2 SDK compatibility scope:

1. ListObjectsV2 plus prefix, delimiter, and pagination support.
2. Range requests for GetObject.
3. SDK-generated presigned `PUT` and `GET` URLs for simple object upload/download flows.
4. DeleteObjects multi-delete.
5. CopyObject for same-user same-bucket and cross-bucket copies.
6. Multipart upload lifecycle.
7. PutObject/HeadObject/GetObject preservation for `Content-Type` and `x-amz-meta-*` user metadata.

Bucket administration APIs, ACLs, versioning, lifecycle, object lock, tagging, and policy APIs are not v0.2.0 targets.

## Operation Matrix

### Object Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| GetObject | Partial | Basic full-object download and byte `Range` requests work. Stored Content-Type and user metadata are returned. Conditional headers, response header overrides, and checksum-related response semantics are missing. |
| PutObject | Partial | Basic upload and overwrite work. Content-Type and `x-amz-meta-*` user metadata are persisted. Object tags, storage class, checksum validation, and server-side encryption headers are ignored. |
| DeleteObject | Implemented | Single-object delete is idempotent and returns no content for missing keys. |
| HeadObject | Partial | Returns ETag, Last-Modified, Content-Length, Content-Type, and user metadata. Range awareness, checksum headers, and conditional request behavior are missing. |
| CopyObject | Partial | Basic same-bucket and cross-bucket copies are implemented and covered by Go and Python SDK E2E. COPY preserves object bytes, Content-Type, and user metadata. `x-amz-metadata-directive: REPLACE` returns XML `NotImplemented`. |
| DeleteObjects | Implemented | SDK multi-delete is covered by `test_s3_sdk_delete_objects_removes_multiple_keys`, including missing-key idempotency. |
| GetObjectAcl / PutObjectAcl | Missing | ACLs are not modeled in the S3 API. |
| GetObjectTagging / PutObjectTagging / DeleteObjectTagging | Missing | Object tags are not stored. |
| GetObjectAttributes | Missing | No object attributes response surface. |
| RestoreObject / SelectObjectContent | Missing | Archive restore and S3 Select are out of scope. |

### List Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| ListObjects (V1) | Partial | Prefix, delimiter, marker, and max-keys behavior is covered by Go e2e. SDK validation focuses on V2 because modern clients prefer it. |
| ListObjectsV2 | Implemented | Prefix, delimiter, max-keys, and continuation-token behavior are covered by Go e2e and the Python SDK e2e fixture. |
| ListObjectVersions | Missing | Versioning is not modeled. |

### Multipart Upload Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| CreateMultipartUpload | Implemented | `POST /{bucket}/{key}?uploads`; the legacy `/api/s3/{bucket}/{key}?uploads` alias also works. Captures Content-Type and user metadata for completion. |
| UploadPart | Implemented | `PUT /{bucket}/{key}?uploadId=...&partNumber=...`; the legacy `/api/s3/{bucket}/{key}` alias also works. |
| CompleteMultipartUpload | Implemented | Validates part existence, ETags, ascending part order, and persists metadata captured during CreateMultipartUpload. |
| AbortMultipartUpload | Implemented | Deletes uploaded part chunks and the multipart record. |
| ListMultipartUploads | Partial | Bounded listing now supports `max-uploads`, `prefix`, `key-marker`, and `upload-id-marker` with S3-style truncation markers. `delimiter` still returns XML `NotImplemented`. |
| ListParts | Partial | Bounded listing now supports `max-parts` and `part-number-marker` with truncation markers. Other advanced multipart response fields remain minimal. |
| UploadPartCopy | Missing | Server-side part copy is not implemented. |

### Bucket Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| ListBuckets | Partial | `GET /` and `GET /api/s3` now return the single bucket bound to the signed SFree bucket credentials. This covers setup/discovery probes but does not model account-wide multi-bucket inventory. |
| HeadBucket | Implemented | `HEAD /{bucket}` and `HEAD /api/s3/{bucket}` validate bucket existence for the matching bucket credentials and return `x-amz-bucket-region: us-east-1`. |
| CreateBucket / DeleteBucket | Missing | Bucket lifecycle uses `/api/v1/buckets`. |
| GetBucketLocation | Implemented | `GET /{bucket}?location` and the legacy alias return an empty `LocationConstraint`, matching `us-east-1` semantics for setup and endpoint validation. |
| Bucket policy, ACL, CORS, lifecycle, versioning, tagging, website, logging, replication APIs | Missing | Not modeled for the S3 API. |

### Authentication And Request Features

| Feature | Status | Notes |
| --- | --- | --- |
| AWS Signature V4 header auth | Implemented | Validates `AWS4-HMAC-SHA256` requests against bucket access credentials. Requests with bodies must send `X-Amz-Content-Sha256`; otherwise validation rejects the request without buffering the body. |
| AWS Signature V4 presigned URLs | Implemented | Query-string presign validation supports default S3 unsigned payload behavior and a max TTL of seven days. Woodpecker-runnable Python SDK coverage now verifies aiobotocore-generated presigned `PUT` and `GET` URLs for simple object upload/download flows. |
| AWS Signature V2 | Missing | Legacy clients that require V2 are unsupported. |
| Anonymous access | Missing | S3 API requests require signed bucket credentials. |
| Virtual-hosted-style addressing | Missing | Path-style routing is supported on `/{bucket}` and the legacy `/api/s3/{bucket}` alias, but bucket-in-host virtual-hosted requests are still unsupported. |
| Range requests | Partial | GetObject byte ranges and `Accept-Ranges` are covered by Go e2e and the Python SDK e2e fixture. HeadObject range behavior is not validated as a v0.2 SDK path. |
| Conditional requests | Missing | `If-Match`, `If-None-Match`, `If-Modified-Since`, and related headers are not evaluated. |
| User metadata | Partial | `x-amz-meta-*` headers are stored with lowercase keys, returned on HeadObject/GetObject, replaced on overwrite, and preserved by CopyObject COPY. Metadata search/listing and tags are not supported. |
| Content-Type persistence | Partial | PutObject and CreateMultipartUpload store request Content-Type; HeadObject/GetObject return it and legacy objects default to `application/octet-stream`. |
| Checksums | Missing | S3 checksum headers are not validated or returned. |
| Server-side encryption headers | Missing | SSE request headers are not interpreted. |

## ETag Behavior

Single-part object ETags are SFree-generated SHA-256 values based on file metadata and chunk metadata, not MD5 hashes of object bytes. Multipart completion returns the AWS-style `"md5-of-part-etags-N"` shape, but part ETags are based on SFree chunk names rather than the raw part payload.

Clients should treat ETags as opaque validators. Clients that compare ETags to content MD5 values may warn or skip integrity assumptions.

## Client Compatibility Checks

These checks are based on the current S3 API surface and known request patterns for real clients. Live client runs should execute in Woodpecker or another external CI environment; local Docker builds, local full-stack runs, and other CPU-heavy validation are intentionally avoided on agent machines.

### rclone

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| Configure S3 remote with path-style endpoint | Partial | Requires explicit endpoint and path-style configuration. |
| `rclone lsd` / bucket discovery | Expected for the signed bucket credentials | Minimal `ListBuckets`, `HeadBucket`, and `GetBucketLocation` probes now exist, but discovery remains bucket-scoped because SFree credentials map to a single bucket. Live rclone execution is still not automated in this PR. |
| `rclone ls remote:bucket` | Expected for direct bucket paths | ListObjectsV2 prefix/delimiter/pagination is implemented, but live rclone validation is still manual/not automated in this PR. |
| `rclone copy local remote:bucket` | Partial | Basic PutObject, multipart upload, copy, and listing paths exist; live rclone validation is still manual/not automated in this PR. |
| `rclone cat remote:bucket/key` | Yes for full-object reads | Basic GetObject works. |
| `rclone delete remote:bucket/key` | Yes for single keys | DeleteObject works. |
| Recursive delete or sync | Partial | ListObjectsV2, DeleteObjects, and basic CopyObject exist; sync still needs live-client validation and may hit metadata fidelity gaps. |
| `rclone mount` | Partial | GetObject range support exists, but live mount behavior is not automated in this PR. |

### s3cmd

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| `s3cmd ls` | Expected for the signed bucket credentials | Bucket-discovery probes now exist, but the returned inventory is intentionally limited to the bucket associated with the presented SFree bucket credentials. Live s3cmd validation is not automated in this PR. |
| `s3cmd ls s3://bucket/` | Partial | V1 list behavior has Go e2e coverage for prefix/delimiter, but live s3cmd validation is not automated in this PR. |
| `s3cmd put file s3://bucket/key` | Yes for simple uploads | PutObject works. |
| `s3cmd get s3://bucket/key file` | Yes for full-object downloads | GetObject works. |
| `s3cmd del s3://bucket/key` | Yes | DeleteObject works. |
| Recursive delete or sync | Partial | Prefix-aware listing, DeleteObjects, and basic CopyObject exist; sync remains live-client/manual validation because metadata semantics are still limited. |

### AWS CLI

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| `aws s3 ls` | Expected for the signed bucket credentials | Minimal bucket-discovery probes now exist, but the returned inventory remains bucket-scoped because SFree uses per-bucket S3 credentials. Live AWS CLI validation is still not automated in this PR. |
| `aws s3 ls s3://bucket/` | Expected for direct bucket paths | High-level `aws s3` listing uses ListObjectsV2, which is now covered through SDK tests; live AWS CLI validation is not automated in this PR. |
| `aws s3 cp local s3://bucket/key` | Yes for simple uploads | PutObject works, including Content-Type and user metadata persistence. |
| `aws s3 cp s3://bucket/key local` | Yes for full-object downloads | GetObject works. |
| `aws s3 rm s3://bucket/key` | Yes | DeleteObject works. |
| `aws s3 sync` | Partial | ListObjectsV2, DeleteObjects, CopyObject COPY, and basic metadata behavior are covered; stronger ETag/checksum compatibility remains a gap. |
| `aws s3 presign s3://bucket/key` | Yes for downloads | Presigned SigV4 downloads are supported, and the same query-signing path now has Woodpecker-runnable aiobotocore GET coverage. |

### MinIO Client (`mc`)

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| Configure alias with SFree endpoint | Yes for root-style endpoints | `mc alias set` accepts `scheme://host[:port]/`, so the root-style S3 endpoint now works without a reverse-proxy remap. The legacy `/api/s3` URL shape is still rejected by `mc` because it includes a resource component. If a bucket key collides with a reserved top-level app route on the same host, keep using the legacy `/api/s3` alias or a dedicated S3 hostname. |
| `mc ls alias/bucket` | Expected for direct bucket paths | Root-style alias setup removes the endpoint-shape blocker; broader `mc` workflows beyond the current smoke coverage still need separate live validation. |
| `mc cp file alias/bucket/key` | Expected for simple uploads | Root-style alias setup is now possible; broader recursive workflows still need separate live validation. |
| `mc cat alias/bucket/key` | Yes | Woodpecker smoke now validates a real `mc` read against the root-style endpoint after uploading a fixture object through SFree. |
| `mc rm alias/bucket/key` | Expected for single keys | Root-style alias setup is now possible; delete semantics still need live-client verification for stronger public claims. |
| Recursive remove or mirror | Partial at best | Even with an alias workaround, broader recursive workflows would still need live-client validation and may hit metadata fidelity gaps. |

## v0.2.0 Scope Alignment

| Priority | Paperclip task | Scope | Public status |
| --- | --- | --- | --- |
| Critical | THE-6 | ListObjectsV2 plus V1 prefix/delimiter/pagination backfill | Completed for v0.2 validation. |
| High | THE-9 | GetObject byte Range support and `Accept-Ranges` headers | Completed for v0.2 validation. |
| High | THE-8 | DeleteObjects multi-delete | Completed for v0.2 validation. |
| Medium | THE-7 | CopyObject | Completed for basic same-bucket and cross-bucket copy validation; metadata replacement remains unsupported. |

The public SDK compatibility issue, [#164](https://github.com/siberianbearofficial/sfree/issues/164), should remain open until live SDK/client validation is run and documented. If this matrix merges before live boto3, AWS SDK, and MinIO client checks are executed, update #164 with the matrix link and keep it open as the SDK validation tracker.

## v0.2 SDK Validation Snapshot

Woodpecker-runnable SDK validation lives in `api-go/e2e/test_api_e2e.py` and uses the pinned Python fixture versions in `api-go/e2e/requirements.txt`:
- `aiobotocore==2.21.1`
- `aiohttp==3.11.14`
- `pytest==8.3.5`
- `pytest-asyncio==0.25.3`

Covered SDK paths:
- `test_s3_sdk_list_objects_v2_prefix_delimiter_and_pagination`: `ListObjectsV2` with prefix, delimiter, `MaxKeys`, and continuation token.
- `test_s3_sdk_get_object_range_returns_partial_content`: `GetObject` with byte `Range`, `ContentRange`, `ContentLength`, and `AcceptRanges`.
- `test_s3_sdk_head_object_returns_metadata`: `HeadObject` for ETag, LastModified, ContentLength, and ContentType response fields.
- `test_s3_sdk_presigned_put_and_get_urls`: aiobotocore-generated presigned `PUT` upload and presigned `GET` download with raw HTTP status/body/header verification.
- `test_s3_sdk_copy_object_compatibility`: `CopyObject` for same-bucket copy, cross-bucket copy, missing-source `NoSuchKey`, and unsupported metadata replacement.
- `test_s3_sdk_delete_objects_removes_multiple_keys`: `DeleteObjects` for multiple keys plus missing-key idempotency.
- `test_s3_sdk_multipart_upload_flow`: `CreateMultipartUpload`, `UploadPart`, `ListMultipartUploads`, `ListParts`, and `CompleteMultipartUpload`.

Not automated in this PR:
- AWS CLI, rclone, and s3cmd live binary smoke tests. They still need extra runtime installation and remain documented/manual in this PR.
- MinIO `mc` live validation still needs Woodpecker or documented manual execution. The root-style endpoint removes the alias-configuration blocker, but this PR does not itself add fresh `mc` runtime evidence.
- AWS SDK for Go/JavaScript client fixtures. The Go e2e suite already validates signed S3 endpoint behavior directly; this PR keeps SDK automation to one pinned SDK path to avoid widening CI dependencies.
