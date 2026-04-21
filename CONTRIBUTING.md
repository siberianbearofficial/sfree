# Contributing to SFree

Thanks for contributing. Keep changes small, explicit, and aligned with the
current launch boundaries documented in [README.md](README.md) and
[docs/architecture.md](docs/architecture.md).

## Start With A Tracked Issue

- Prefer working from an existing GitHub issue so the scope is clear enough for
  outside contributors to pick up.
- If your change is not already tracked, open or refine an issue before making a
  large implementation pass.

## Repository Ground Rules

- **`api-go/` is the primary backend.** New backend features belong there.
- **`api-python-archived/` is the archived Python backend** kept for historical
  reference only. Do not modify it.
- **Woodpecker self-hosted** is the only supported CI/CD path for this
  repository. Do not add GitHub Actions workflows. Pipeline triggers, required
  secrets, and published image targets are documented in
  [docs/ci.md](docs/ci.md).
- Preserve the current public caveats:
  - backend-supported sources are Google Drive, Telegram, and S3-compatible
    storage
  - the browser UI supports Google Drive, Telegram, and S3-compatible sources
  - uploads are split into chunks and distributed across sources without
    redundancy guarantees
  - auth and credential handling are functional but not production-hardened

## Local Setup

### Backend (Go)

Prerequisites:

- Go 1.24+
- Docker, or another reachable MongoDB instance
- [`golangci-lint`](https://golangci-lint.run/welcome/install/) for the
  standard lint pass

Start local MongoDB:

```bash
cd api-go
docker compose up -d
```

Run the API:

```bash
cd api-go
ENV=local go run ./cmd/server
```

Swagger is served from `http://localhost:8080/swagger/index.html`.

### Frontend (React / Vite)

Prerequisites:

- Node.js 20+ with npm

Install **all** dependencies (including dev tools like TypeScript and ESLint):

```bash
cd webui
npm ci
```

> **Why `npm ci` instead of `npm install`?** `npm ci` installs from the
> lockfile for deterministic builds and includes dev dependencies by default.
> Avoid running with `NODE_ENV=production`, which omits `devDependencies` and
> breaks `npm run lint` and `npm run build` (they need `tsc` and `eslint`).

Start the Vite dev server:

```bash
npm run dev
```

The dev server proxies `/api` requests to `http://localhost:8080` automatically
(configurable via `VITE_API_URL`). Make sure the Go API is running locally
before starting the frontend. See `webui/.env.example` for available env vars.

For development against the Docker Compose stack, no extra config is needed —
the webui container proxies API requests via nginx.

For standalone dev against a local Go API:

```bash
VITE_API_BASE=http://localhost:8080/api/v1 npm run dev
```

## Validation

Run the checks that match the area you changed before opening a PR.

### Backend

```bash
cd api-go
golangci-lint run
make test
```

The default `make test` target runs unit tests only. Integration, Go E2E,
Python E2E, and Docker-backed local E2E suites are available through their
explicit Make targets when broader validation is needed.

### Frontend

```bash
cd webui
npm ci           # ensure deps are installed from lockfile
npm run lint     # ESLint
npm run build    # TypeScript type-check + Vite production build
```

Both `lint` and `build` must pass — this matches what Woodpecker CI runs on
pull requests (see [docs/ci.md](docs/ci.md)).

### Optional: E2E Tests

```bash
cd api-go
make test-e2e-local
```

`make test-e2e-local` uses `docker-compose.e2e.yml`. The `s3` E2E flow works
with the bundled MinIO service. The Google Drive and Telegram flows require
their corresponding credentials.

## API Changes

When you add, remove, or change API endpoints in `api-go/`:

- update the Swagger comments in Go source
- regenerate `api-go/internal/docs/docs.go`
- avoid committing generated artifacts such as `swagger.json` or `swagger.yaml`

## Pull Requests

- Explain the behavior change and any remaining caveats in the PR description.
- Reference the issue that motivated the change.
- Expect the relevant Woodpecker checks to pass before review or merge.
- If you change contributor-facing behavior, update the relevant docs in the
  same PR.
