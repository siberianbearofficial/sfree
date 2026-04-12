# SFree

SFree is an experimental source-backed object storage project. The maintained
backend accepts uploads, splits them into chunks, stores those chunks across
user-provided sources, and exposes both a browser UI and S3-compatible object
routes for retrieval.

## Repository Layout

- `api-go/` is the primary backend. It owns the supported HTTP API,
  S3-compatible routes, Swagger docs, and the MongoDB-backed metadata model.
- `webui/` is a React/Vite frontend for signup, bucket management, and browser
  file operations.
- `api/` is a deprecated Python backend kept for historical reference. Do not
  treat it as the active product path.
- `.woodpecker/` contains the self-hosted CI/CD pipelines for the backend and
  frontend images. Exact trigger, secret, and image-publish details live in
  [docs/ci.md](docs/ci.md).
- `docs/architecture.md` documents the current architecture, supported source
  types, and public launch caveats.

## Supported Scope Today

- Backend-supported source types: Google Drive, Telegram, and S3-compatible
  storage.
- Browser UI-ready flow: signup, Google Drive source creation, bucket creation,
  bucket file upload/download/delete.
- Programmatic flow: REST API plus S3-compatible object access from `api-go`.

## Launch Caveats

- File chunks are distributed across sources in round-robin order. There is no
  replication, erasure coding, or durability guarantee if an upstream source is
  lost.
- Authentication is HTTP Basic Auth, and the current web UI stores credentials
  in `localStorage`. Source create/list responses also echo stored credential
  payloads. This repository should not be described as hardened security
  infrastructure.
- Source observability differs by backend. Google Drive exposes the richest
  info, while Telegram and S3-compatible sources expose narrower metadata.
- Telegram and S3 source creation are backend-supported, but the checked-in
  browser UI currently creates Google Drive sources only.

## Local Quickstart

1. Start MongoDB:

   ```bash
   cd api-go
   docker compose up -d
   ```

2. Run the Go API:

   ```bash
   cd api-go
   ENV=local go run ./cmd/server
   ```

3. Open Swagger at `http://localhost:8080/swagger/index.html` or call the API
   directly on `http://localhost:8080`.

4. Run the frontend when you need the browser flow:

   ```bash
   cd webui
   npm install
   npm run dev
   ```

5. The checked-in frontend points at
   `https://s3aas-api.dev.nachert.art/api/v1` in `webui/src/shared/api/*.ts`.
   Update those modules before relying on a fully local UI loop.

## Contributing

Contribution guidelines live in [CONTRIBUTING.md](CONTRIBUTING.md). Start with
the architecture notes in [docs/architecture.md](docs/architecture.md) so new
work preserves the current public launch boundaries. CI/CD expectations for
contributors and maintainers live in [docs/ci.md](docs/ci.md).

## License

SFree is released under the MIT license. See [LICENSE.txt](LICENSE.txt).
