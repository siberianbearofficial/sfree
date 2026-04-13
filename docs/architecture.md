# SFree Architecture

## System Overview

SFree is a source-backed object storage prototype. The active backend is
`api-go`, which stores metadata in MongoDB, writes file chunks to configured
upstream sources, and exposes both REST and S3-compatible access paths.

At a high level:

1. A user signs up and authenticates with HTTP Basic Auth.
2. The user registers one or more storage sources.
3. A bucket records the selected source set plus generated S3 credentials.
4. Uploads are split into chunks and assigned to sources in round-robin order.
5. Downloads and S3 object reads reconstruct the file by fetching the saved
   chunk manifest from those sources.

## Repository Components

### `api-go/`

- Primary backend and the only supported backend path for new work.
- Exposes:
  - user creation
  - source management
  - bucket management
  - browser upload/download/delete flows
  - S3-compatible object GET, PUT, LIST, and DELETE routes
- Uses MongoDB for users, buckets, sources, and file manifests.

### `webui/`

- React 19 + Vite frontend.
- Current capabilities:
  - signup/login
  - source creation for Google Drive, Telegram, and S3-compatible backends
  - bucket creation, listing, and deletion
  - file upload, download, and deletion

### `api-python-archived/`

- Archived Python backend — kept for historical reference only.
- Not part of the active CI/CD pipelines or Docker Compose stack.
- Do not modify or route new contributors there.

### `.woodpecker/`

- Self-hosted CI/CD entrypoint for this repository.
- `api-go.yml` runs backend E2E coverage and publishes the backend image.
- `webui.yml` validates frontend pull requests and publishes the frontend image
  on `main`.
- `docs/ci.md` records the exact triggers, secrets, and image targets that the
  public repo depends on.

## Supported Source Types

| Source type | Backend support | Browser UI creation | Notes |
| --- | --- | --- | --- |
| Google Drive | Yes | Yes | Richest source info and quota reporting. |
| Telegram | Yes | Yes | Uses bot API for chunk storage. |
| S3-compatible | Yes | Yes | Works with MinIO, Backblaze B2, Wasabi, etc. |

## Data Model And Storage Behavior

- MongoDB stores user records, source definitions, bucket metadata, and file
  chunk manifests.
- The upload manager selects a source per chunk with a round-robin strategy.
- Chunks are not replicated. Losing a configured upstream source can make file
  reconstruction fail.
- S3-compatible bucket credentials are generated per bucket and backed by the
  same file/source metadata used by the browser endpoints.

## Public Launch Caveats

These caveats must stay intact in contributor docs, launch docs, and marketing
copy:

- Do not claim redundancy, erasure coding, or durable multi-provider storage.
  The current design distributes chunks; it does not replicate them.
- Do not describe the auth model as hardened. The API uses Basic Auth and the
  frontend stores credentials in `localStorage`, and source create/list
  responses echo stored credential payloads.
- Do not imply identical observability across source types. Google Drive,
  Telegram, and S3-compatible sources return different levels of file/quota
  detail.

## Local Development Notes

- Default local backend config lives in `api-go/config/local.yaml`.
- The Go API listens on the Gin default port, `:8080`.
- `api-go/docker-compose.yml` starts the local MongoDB dependency.
- The checked-in frontend points at a hosted dev API URL in
  `webui/src/shared/api/*.ts`, so a fully local browser loop still requires a
  small manual config edit.

## Naming

All internal module paths, image names, database names, and user-facing strings
use the `sfree` identifier consistently.
