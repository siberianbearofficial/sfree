# SFree

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.txt)
[![Go 1.24](https://img.shields.io/badge/Go-1.24-00ADD8.svg)](https://go.dev/)

**Store files across Google Drive, Telegram, and S3-compatible services — access
them through one API or an S3-compatible interface.**

## Why SFree

Cloud storage is cheap in pieces — a free Google Drive here, a Telegram bot
there, a MinIO bucket on a spare VPS. But using them together is a manual mess.

SFree turns multiple storage services into a single object store. Upload a file
and SFree splits it into chunks, distributes them across your configured
sources, and reassembles them on download. You get one REST API, one
S3-compatible endpoint, and one browser UI for everything.

**Who it's for:** Self-hosters, homelab enthusiasts, and developers who want to
unify free-tier and personal storage backends behind a single interface.

**What it is not:** SFree is an experimental prototype. It does not replicate
chunks, provide erasure coding, or guarantee durability if an upstream source is
lost. See [Launch Caveats](#launch-caveats) for the full picture.

## Supported Storage Backends

| Backend | API support | Browser UI support | Notes |
| --- | --- | --- | --- |
| Google Drive | Yes | Yes | Richest quota and file metadata reporting |
| Telegram | Yes | No (API-only) | Uses bot API for chunk storage |
| S3-compatible | Yes | No (API-only) | Works with MinIO, Backblaze B2, Wasabi, etc. |

## Architecture

```mermaid
graph TD
    Client["Client (Browser / S3 SDK / curl)"]
    API["api-go (Go backend)"]
    Mongo["MongoDB (metadata)"]
    GDrive["Google Drive"]
    Telegram["Telegram"]
    S3["S3-compatible storage"]

    Client -->|"REST / S3-compat"| API
    API --> Mongo
    API -->|"chunks (round-robin)"| GDrive
    API -->|"chunks (round-robin)"| Telegram
    API -->|"chunks (round-robin)"| S3
```

1. You register one or more **storage sources** (Google Drive, Telegram, or
   S3-compatible).
2. You create a **bucket** and select which sources back it.
3. Uploads are split into chunks and distributed across the selected sources in
   round-robin order.
4. Downloads reassemble the file from its chunk manifest.

Each bucket also gets generated S3 credentials, so any S3-compatible client can
read and write objects directly.

## Quick Start

> **Full walkthrough:** [Docker Compose Quickstart](docs/quickstart.md) — zero
> to upload in under 5 minutes, with expected output for every step.

### Prerequisites

- Docker and Docker Compose

### 1. Start the full stack

```bash
git clone https://github.com/siberianbearofficial/sfree.git
cd sfree
docker compose up --build
```

This starts MongoDB, the Go API, a React frontend (with nginx), and a MinIO
instance for local S3-compatible source testing.

| Service  | URL                          |
| -------- | ---------------------------- |
| Frontend | http://localhost:3000        |
| API      | http://localhost:8080        |
| Swagger  | http://localhost:8080/swagger/index.html |
| MinIO Console | http://localhost:9001   |

### 2. Try it out

1. Open http://localhost:3000 and sign up.
2. Create an S3-compatible source via the API (using the bundled MinIO):
   ```bash
   # Create user first, note the password from the response
   curl -X POST http://localhost:8080/api/v1/users \
     -H 'Content-Type: application/json' \
     -d '{"username": "demo"}'

   # Use the returned password for Basic Auth (base64 of demo:PASSWORD)
   # Create an S3-compatible source backed by MinIO
   curl -X POST http://localhost:8080/api/v1/sources/s3 \
     -H 'Content-Type: application/json' \
     -H 'Authorization: Basic BASE64_CREDENTIALS' \
     -d '{
       "name": "local-minio",
       "endpoint": "http://minio:9000",
       "bucket": "sfree-data",
       "access_key_id": "minioadmin",
       "secret_access_key": "minioadmin",
       "region": "us-east-1",
       "path_style": true
     }'
   ```
3. Create a bucket in the UI (select the MinIO source).
4. Upload a file through the UI.
5. Download the same file via aws-cli using the bucket's S3 credentials:
   ```bash
   aws s3 ls s3://BUCKET_KEY/ \
     --endpoint-url http://localhost:8080/api/s3

   aws s3 cp s3://BUCKET_KEY/OBJECT_KEY ./local-copy \
     --endpoint-url http://localhost:8080/api/s3
   ```

### Manual dev setup (without Docker Compose)

<details>
<summary>Click to expand</summary>

#### Prerequisites

- Go 1.24+
- Docker (for MongoDB)
- Node.js with npm (for the browser UI)

#### 1. Start MongoDB

```bash
cd api-go
docker compose up -d
```

#### 2. Run the Go API

```bash
cd api-go
ENV=local go run ./cmd/server
```

The API listens on `http://localhost:8080`.

#### 3. Run the browser UI

```bash
cd webui
VITE_API_BASE=http://localhost:8080/api/v1 npm run dev
```

Set `VITE_API_BASE` to point the frontend at your local API. Without it, the
frontend defaults to a relative `/api/v1` path (designed for the Docker Compose
setup where nginx proxies to the API).

</details>

## Repository Layout

| Directory | Purpose |
| --- | --- |
| `api-go/` | Primary Go backend — HTTP API, S3-compatible routes, Swagger docs, MongoDB metadata |
| `webui/` | React 19 + Vite frontend — signup, bucket management, file operations |
| `api/` | Deprecated Python backend (historical reference only) |
| `.woodpecker/` | Self-hosted Woodpecker CI/CD pipelines |
| `docs/` | Architecture notes, CI docs |

## Launch Caveats

SFree is an early-stage project. These constraints are current and intentional:

- **No redundancy.** Chunks are distributed, not replicated. Losing an upstream
  source can make files unrecoverable.
- **Basic auth only.** The API uses HTTP Basic Auth. The browser UI stores
  credentials in `localStorage`. Source API responses echo stored credential
  payloads.
- **Uneven observability.** Google Drive sources expose the richest file and
  quota info. Telegram and S3-compatible sources return less metadata.
- **Browser UI covers Google Drive only.** Telegram and S3-compatible source
  creation requires the API directly.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, validation, and PR guidelines.
Architecture details are in [docs/architecture.md](docs/architecture.md) and CI
expectations in [docs/ci.md](docs/ci.md).

## License

MIT — see [LICENSE.txt](LICENSE.txt).
