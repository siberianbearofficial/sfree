# S3 Compatibility Matrix

Last updated: 2026-04-20

Baseline: `origin/main` at `eff5880` before the v0.2.0 S3 compatibility PRs. SFree exposes its S3-compatible API under `/api/s3` and uses path-style addressing: `/api/s3/{bucket}/{key}`.

Reference: [Amazon S3 API operations](https://docs.aws.amazon.com/AmazonS3/latest/API/API_Operations_Amazon_Simple_Storage_Service.html).

## Summary

SFree supports the core object lifecycle: upload, download, head, delete, and multipart upload. The main compatibility gaps are list semantics, byte ranges, multi-delete, server-side copy, and S3 bucket-discovery calls used by general-purpose clients.

Top v0.2.0 implementation scope:

1. ListObjectsV2 plus prefix, delimiter, and pagination support.
2. Range requests for GetObject and HeadObject.
3. DeleteObjects multi-delete.
4. CopyObject.

Bucket administration APIs, ACLs, versioning, lifecycle, object lock, tagging, and policy APIs are not v0.2.0 targets.

## Operation Matrix

### Object Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| GetObject | Partial | Basic full-object download works. `Range`, conditional headers, response header overrides, and checksum-related response semantics are missing. |
| PutObject | Partial | Basic upload and overwrite work. Content-Type, user metadata, object tags, storage class, checksum validation, and server-side encryption headers are ignored. |
| DeleteObject | Implemented | Single-object delete is idempotent and returns no content for missing keys. |
| HeadObject | Partial | Returns ETag, Last-Modified, Content-Length, and Content-Type. Range awareness, checksum headers, metadata, and conditional request behavior are missing. |
| CopyObject | Missing | A PUT with `x-amz-copy-source` is not dispatched to copy behavior on `origin/main`. |
| DeleteObjects | Missing | Bucket-level `POST ?delete` with XML body is not routed. |
| GetObjectAcl / PutObjectAcl | Missing | ACLs are not modeled in the S3 API. |
| GetObjectTagging / PutObjectTagging / DeleteObjectTagging | Missing | Object tags are not stored. |
| GetObjectAttributes | Missing | No object attributes response surface. |
| RestoreObject / SelectObjectContent | Missing | Archive restore and S3 Select are out of scope. |

### List Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| ListObjects (V1) | Partial | Returns bucket objects, but ignores `prefix`, `delimiter`, `marker`, and `max-keys`; response says `MaxKeys=1000` while returning all entries. |
| ListObjectsV2 | Missing | `list-type=2` currently falls through to the V1 handler. PR #166 is the active v0.2.0 implementation slice. |
| ListObjectVersions | Missing | Versioning is not modeled. |

### Multipart Upload Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| CreateMultipartUpload | Implemented | `POST /api/s3/{bucket}/{key}?uploads`. |
| UploadPart | Implemented | `PUT /api/s3/{bucket}/{key}?uploadId=...&partNumber=...`. |
| CompleteMultipartUpload | Implemented | Validates part existence, ETags, and ascending part order. |
| AbortMultipartUpload | Implemented | Deletes uploaded part chunks and the multipart record. |
| ListMultipartUploads | Partial | Lists active uploads for a bucket but lacks prefix, delimiter, key-marker, upload-id-marker, and pagination support. |
| ListParts | Partial | Lists uploaded parts but lacks pagination and part-number-marker support. |
| UploadPartCopy | Missing | Server-side part copy is not implemented. |

### Bucket Operations

| S3 operation | Status | Notes |
| --- | --- | --- |
| ListBuckets | Missing | Buckets are managed through SFree REST APIs, not S3 `GET /`. |
| HeadBucket | Missing | No S3 bucket existence probe route. |
| CreateBucket / DeleteBucket | Missing | Bucket lifecycle uses `/api/v1/buckets`. |
| GetBucketLocation | Missing | Many clients use this during setup or endpoint validation. |
| Bucket policy, ACL, CORS, lifecycle, versioning, tagging, website, logging, replication APIs | Missing | Not modeled for the S3 API. |

### Authentication And Request Features

| Feature | Status | Notes |
| --- | --- | --- |
| AWS Signature V4 header auth | Implemented | Validates `AWS4-HMAC-SHA256` requests against bucket access credentials. |
| AWS Signature V4 presigned URLs | Implemented | Query-string presign validation supports default S3 unsigned payload behavior and a max TTL of seven days. |
| AWS Signature V2 | Missing | Legacy clients that require V2 are unsupported. |
| Anonymous access | Missing | S3 API requests require signed bucket credentials. |
| Virtual-hosted-style addressing | Missing | Only path-style routing under `/api/s3/{bucket}` is supported. |
| Range requests | Missing | `Range` is ignored and full-object responses are returned. |
| Conditional requests | Missing | `If-Match`, `If-None-Match`, `If-Modified-Since`, and related headers are not evaluated. |
| User metadata | Missing | `x-amz-meta-*` headers are not persisted or returned. |
| Content-Type persistence | Missing | Downloads always return `application/octet-stream`. |
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
| `rclone lsd` / bucket discovery | No | `ListBuckets` is missing. |
| `rclone ls remote:bucket` | No by default | rclone defaults to ListObjectsV2; V2 is missing. V1 fallback still needs prefix and delimiter behavior. |
| `rclone copy local remote:bucket` | Partial | Basic PutObject and multipart upload paths exist. Directory reconciliation still needs listing support. |
| `rclone cat remote:bucket/key` | Yes for full-object reads | Basic GetObject works. |
| `rclone delete remote:bucket/key` | Yes for single keys | DeleteObject works. |
| Recursive delete or sync | No | Needs ListObjectsV2 and DeleteObjects. |
| `rclone mount` | No | Needs Range requests for seek/read-ahead behavior. |

### s3cmd

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| `s3cmd ls` | No | `ListBuckets` is missing. |
| `s3cmd ls s3://bucket/` | Partial | V1 listing works only as a flat bucket dump; prefix and delimiter are ignored. |
| `s3cmd put file s3://bucket/key` | Yes for simple uploads | PutObject works. |
| `s3cmd get s3://bucket/key file` | Yes for full-object downloads | GetObject works. |
| `s3cmd del s3://bucket/key` | Yes | DeleteObject works. |
| Recursive delete or sync | No | Needs prefix-aware listing and DeleteObjects. |

### AWS CLI

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| `aws s3 ls` | No | `ListBuckets` is missing. |
| `aws s3 ls s3://bucket/` | No | High-level `aws s3` listing uses ListObjectsV2. |
| `aws s3 cp local s3://bucket/key` | Yes for simple uploads | PutObject works; metadata/content-type persistence is missing. |
| `aws s3 cp s3://bucket/key local` | Yes for full-object downloads | GetObject works. |
| `aws s3 rm s3://bucket/key` | Yes | DeleteObject works. |
| `aws s3 sync` | No | Needs ListObjectsV2, CopyObject, DeleteObjects, metadata behavior, and stronger ETag/checksum compatibility. |
| `aws s3 presign s3://bucket/key` | Yes for downloads | Presigned SigV4 requests are supported. |

### MinIO Client (`mc`)

| Workflow | Expected result on `origin/main` | Blocking gaps |
| --- | --- | --- |
| Configure alias with SFree endpoint | Partial | Requires path-style endpoint behavior through `/api/s3`; bucket discovery still fails. |
| `mc ls alias/bucket` | No | Modern listing expects ListObjectsV2 semantics. |
| `mc cp file alias/bucket/key` | Partial | PutObject should work for simple uploads; recursive copy needs list semantics. |
| `mc cat alias/bucket/key` | Yes for full-object reads | Basic GetObject works. |
| `mc rm alias/bucket/key` | Yes for single keys | DeleteObject works. |
| Recursive remove or mirror | No | Needs ListObjectsV2 and DeleteObjects. |

## v0.2.0 Scope Alignment

| Priority | Paperclip task | Scope | Public status |
| --- | --- | --- | --- |
| Critical | THE-6 | ListObjectsV2 plus V1 prefix/delimiter/pagination backfill | PR #166 open. |
| High | THE-9 | GetObject byte Range support and `Accept-Ranges` headers | Blocked behind THE-6. |
| High | THE-8 | DeleteObjects multi-delete | Blocked behind THE-6 and THE-9. |
| Medium | THE-7 | CopyObject | Parked behind the higher-impact list/range/delete slices. |

The public SDK compatibility issue, [#164](https://github.com/siberianbearofficial/sfree/issues/164), should remain open until live SDK/client validation is run and documented. If this matrix merges before live boto3, AWS SDK, and MinIO client checks are executed, update #164 with the matrix link and keep it open as the SDK validation tracker.
