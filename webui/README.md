# SFree Web UI

`webui/` is the React/Vite frontend for the SFree repository.

## Current Scope

- landing page plus signup and login dialogs
- source listing and creation for Google Drive, Telegram, and S3-compatible sources
- bucket creation, listing, deletion, file upload, download, and deletion

## Development

Install dependencies and run the dev server:

```bash
npm install
npm run dev
```

Validation commands:

- `npm run lint`
- `npm run build`

## Backend Configuration

The frontend reads `VITE_API_BASE` at build time (defaults to `/api/v1`).
When running via the Docker Compose stack, the webui container proxies API
requests to the backend automatically. For standalone dev, set the env var
to point at the running API.
