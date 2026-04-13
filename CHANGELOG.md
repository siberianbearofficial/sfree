# Changelog

## v0.1.0 — 2026-04-13

First tagged release of SFree. This release consolidates all work across
backend, frontend, CLI, and infrastructure into a single versioned artifact.

### Features

- **S3-compatible object storage** — GET, PUT, LIST, DELETE via an
  S3-compatible endpoint backed by Google Drive, Telegram, and S3-compatible
  storage sources.
- **Browser UI** — React 19 + Vite frontend for signup, source management,
  bucket operations, file upload/download/delete, and file preview.
- **Public share links** — generate time-limited public URLs for individual
  files.
- **File preview** — inline preview and download modal in the bucket view.
- **sfree CLI** — command-line tool for power users to manage buckets, sources,
  files, and S3 access keys.
- **Configurable chunk distribution** — round-robin and weighted strategies for
  distributing chunks across sources per bucket.
- **GitHub OAuth** — authenticate via GitHub in addition to HTTP Basic Auth.
  JWT-based token login for API and CLI use.
- **Error messages and UX feedback** — toast notifications, loading spinners,
  and empty states across the web UI.

### Infrastructure

- **Observability** — Prometheus metrics endpoint (`/metrics`), structured
  `slog` logging with trace IDs, and request duration/status middleware.
- **OpenTelemetry tracing** — distributed tracing via OTLP/gRPC exporter with
  Jaeger integration in Docker Compose.
- **Woodpecker CI** — self-hosted CI/CD with unit tests, E2E tests (GDrive,
  Telegram, S3), and Docker image publishing.
- **Python API archived** — legacy Python backend moved to
  `api-python-archived/` for historical reference.
- **Developer docs** — polished CONTRIBUTING.md, architecture docs, and CI
  documentation.

### Known Limitations

- No chunk redundancy or erasure coding — losing an upstream source means data
  loss for chunks stored there.
- Auth and credential handling are functional but not production-hardened.
- No rate limiting (planned for a future release).
- Single-node deployment only.
