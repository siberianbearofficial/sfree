# SFree CI/CD

SFree uses Woodpecker self-hosted as its only CI/CD system. Do not add GitHub
Actions workflows to this repository.

## Pipeline Matrix

| Pipeline | Paths | `pull_request` | `push` to `main` | Behavior |
| --- | --- | --- | --- | --- |
| `.woodpecker/api-go.yml` | `.woodpecker/api-go.yml`, `api-go/**` | Yes | Yes | Runs `golangci-lint run`, Go unit tests, blocking `govulncheck` dependency auditing, generated API docs freshness, and Python and Go E2E suites against the local S3-compatible MinIO source. Pushes to `main` also publish the backend image. |
| `.woodpecker/webui.yml` | `.woodpecker/webui.yml`, `webui/**` | Yes | Yes | Runs frontend dependency install, `npm audit --audit-level=high`, lint, and production build in `validate`, then runs Chromium Playwright E2E validation in `e2e tests`. Pushes to `main` also publish the frontend image. |
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

- `validate`: runs `npm ci --include=dev`, `npm audit --audit-level=high`,
  `npm run lint`, and `npm run build`.
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

## Dependency Audits

Backend pull requests run `govulncheck@v1.1.4` in Woodpecker on Go 1.25. Known
reachable Go vulnerability findings fail the `dependency audit` step. Tool
installation and execution errors also fail the step.

Frontend pull requests run `npm audit --audit-level=high` after
`npm ci --include=dev`, so high and critical advisory matches fail before lint,
build, and browser validation. The webui lockfile currently has no high or
critical npm audit findings. Run dependency audits locally only when you need to
reproduce the CI failure; otherwise leave dependency auditing to Woodpecker.

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
