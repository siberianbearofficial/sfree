# SFree CI/CD

SFree uses Woodpecker self-hosted as its only CI/CD system. Do not add GitHub
Actions workflows to this repository.

## Pipeline Matrix

| Pipeline | Paths | `pull_request` | `push` to `main` | Behavior |
| --- | --- | --- | --- | --- |
| `.woodpecker/api-go.yml` | `.woodpecker/api-go.yml`, `api-go/**` | Yes | Yes | Runs the Go backend E2E suite for `gdrive`, `telegram`, and `s3`. Pushes to `main` also publish the backend image. |
| `.woodpecker/webui.yml` | `.woodpecker/webui.yml`, `webui/**` | Yes | Yes | Runs `npm ci --include=dev`, `npm run lint`, and `npm run build`. Pushes to `main` also publish the frontend image. |
| `.woodpecker/smoke.yml` | `.woodpecker/smoke.yml`, `docker-compose.yml`, `scripts/woodpecker-smoke.sh`, `api-go/**`, `webui/**` | Yes | Yes | Starts the root Compose stack in Woodpecker, creates a user and MinIO-backed source, creates a bucket, uploads and downloads a file with the CLI, and verifies S3-compatible credential download bytes. |

## Required Secrets

| Secret | Used by | Purpose |
| --- | --- | --- |
| `GITHUB_TOKEN` | `api-go.yml`, `webui.yml` | Authenticates GHCR pushes for the published images. |
| `E2E_GDRIVE_KEY` | `api-go.yml` | Credentials for the Google Drive E2E source flow. |
| `E2E_TELEGRAM_TOKEN` | `api-go.yml` | Token for the Telegram E2E source flow. |
| `E2E_TELEGRAM_CHAT_ID` | `api-go.yml` | Chat destination for the Telegram E2E source flow. |

The frontend validation step does not need repository secrets. Backend pull
requests do need the E2E secrets configured in Woodpecker because the pipeline
executes the real multi-source end-to-end flow.

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
