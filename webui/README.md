# SFree Web UI

`webui/` is the React/Vite frontend for the SFree repository. It currently
covers the browser-safe flow that is ready to describe publicly today.

## Current Scope

- landing page plus signup and login dialogs
- source listing plus Google Drive source creation
- bucket creation, listing, deletion, file upload, download, and deletion

The backend supports Telegram and S3-compatible sources, but this frontend does
not yet expose source creation flows for them.

## Development

Install dependencies and run the dev server:

```bash
npm install
npm run dev
```

Validation commands:

- `npm run lint`
- `npm run build`

## Backend Configuration Caveat

The checked-in frontend calls
`https://s3aas-api.dev.nachert.art/api/v1` directly from:

- `src/shared/api/users.ts`
- `src/shared/api/buckets.ts`
- `src/shared/api/sources.ts`

Update those modules before using the UI against a fully local backend.
