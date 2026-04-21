# SFree Web UI

`webui/` is the React/Vite frontend for the SFree repository.

## Current Scope

- landing page plus signup and login dialogs
- source listing and creation for Google Drive, Telegram, and S3-compatible sources
- bucket creation, listing, deletion, file upload, download, and deletion

## Development

Install dependencies from the lockfile and run the dev server:

```bash
cd webui
npm ci
npm run dev
```

For standalone development against a local Go API:

```bash
VITE_API_BASE=http://localhost:8080/api/v1 npm run dev
```

Normal local validation:

- `npm run lint`
- `npm run build`

Playwright/browser E2E is validated in Woodpecker for webui changes. Do not run
it locally by default unless a maintainer asks for local reproduction.

See [../CONTRIBUTING.md](../CONTRIBUTING.md) and [../docs/ci.md](../docs/ci.md)
for full repository validation expectations.

## Backend Configuration

The frontend reads `VITE_API_BASE` at build time (defaults to `/api/v1`).
When running via the Docker Compose stack, the webui container proxies API
requests to the backend automatically. For standalone dev, set the env var
to point at the running API.
