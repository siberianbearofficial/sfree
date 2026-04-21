# SFree CI/CD

SFree uses Woodpecker self-hosted as its only CI/CD system. Do not add GitHub
Actions workflows to this repository.

## Pipeline Matrix

| Pipeline | Paths | `pull_request` | `push` to `main` | Behavior |
| --- | --- | --- | --- | --- |
| `.woodpecker/api-go.yml` | `.woodpecker/api-go.yml`, `api-go/**` | Yes | Yes | Runs `golangci-lint run`, Go unit tests, and Python and Go E2E suites against the local S3-compatible MinIO source. Pushes to `main` also publish the backend image. |
| `.woodpecker/webui.yml` | `.woodpecker/webui.yml`, `webui/**` | Yes | Yes | Runs frontend dependency install, lint, production build, and Chromium Playwright E2E validation with `npm run test:e2e`. Pushes to `main` also publish the frontend image. |
| `.woodpecker/smoke.yml` | `.woodpecker/smoke.yml`, `docker-compose.yml`, `scripts/woodpecker-smoke.sh`, `api-go/**`, `webui/**` | Yes | Yes | Starts the root Compose stack in Woodpecker, creates a user and MinIO-backed source, creates a bucket, uploads and downloads a file with the CLI, and verifies S3-compatible credential download bytes. |

## Required Secrets

| Secret | Used by | Purpose |
| --- | --- | --- |
| `GITHUB_TOKEN` | `api-go.yml`, `webui.yml` | Authenticates GHCR pushes for the published images. |

Frontend validation and backend pull-request validation do not need live source
E2E secrets. The backend E2E gate uses the local MinIO source so external Google
Drive or Telegram availability cannot block otherwise healthy PRs.

## Web UI E2E

The webui pipeline has two required validation steps in Woodpecker:

- `validate`: runs `npm ci --include=dev`, `npm run lint`, and `npm run build`.
- `e2e tests`: runs `npm ci --include=dev`, `npm run build`,
  `npx playwright install --with-deps chromium`, and `npm run test:e2e`.

Playwright browser installation and browser-heavy E2E execution belong in
Woodpecker. Local frontend validation should normally stop at lint and build
unless a maintainer explicitly asks for local browser reproduction.

## Live Source E2E

The Python E2E suite still supports `gdrive` and `telegram` source modes for
manual or non-blocking validation. Those modes require:

- `E2E_GDRIVE_KEY` for Google Drive.
- `E2E_TELEGRAM_TOKEN` and `E2E_TELEGRAM_CHAT_ID` for Telegram.

Do not put live source checks in the required PR path unless the external
service dependency is deliberately accepted as a merge blocker.

The stack smoke pipeline uses only local Woodpecker services and does not need
repository secrets.

## Published Images

Pushes to `main` publish these images to GitHub Container Registry:

- `ghcr.io/siberianbearofficial/sfree-api-go:main`
- `ghcr.io/siberianbearofficial/sfree-webui:main`

## Deployment Expectations

- The repository pipelines stop at validation plus image publication.
- Environment rollout is expected to happen from external infrastructure that
  consumes the `:main` images published by Woodpecker.
- If deploy behavior, registry targets, or required secrets change, update
  `.woodpecker/` and this document in the same PR.
