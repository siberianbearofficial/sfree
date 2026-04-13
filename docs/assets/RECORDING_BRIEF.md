# Demo GIF Recording Brief

Target file: `docs/assets/demo.gif`

## Specs

- Format: GIF (or MP4 converted to GIF)
- Duration: 20-25 seconds
- Resolution: 1280x720 (renders well on GitHub at `width="720"`)
- File size: under 10 MB
- Frame rate: 10-15 fps (keeps GIF size down)
- Browser window: clean, no bookmarks bar, minimal chrome

## Recording environment

Run locally with Docker Compose (`docker compose up --build`).
Frontend at `http://localhost:3000`, MinIO at `http://minio:9000` (internal).

Pre-create the user account before recording. Start the recording already
logged in to avoid showing credentials on screen.

## Scene sequence

### Scene 1 — Sources page (3s)

1. Start on `/sources` showing one pre-created S3-compatible source named
   "local-minio" (card visible with S3 chip).
2. Pause briefly so the viewer sees the sources list.

### Scene 2 — Create a bucket (6s)

1. Navigate to `/buckets`.
2. Click "Add Bucket".
3. Type `demo-bucket` in the Key field.
4. Check the "local-minio" source checkbox.
5. Click "Create".
6. Pause on the generated S3 credentials screen (1s), then close.

### Scene 3 — Upload a file (5s)

1. Click the new `demo-bucket` card to open it.
2. Click "Upload File".
3. Select a small sample file (e.g. `hello.txt`, ~100 bytes).
4. Wait for upload to complete — file appears in the table.

### Scene 4 — Browse files (3s)

1. Pause on the file list showing the uploaded file with name, size, and date.

### Scene 5 — Download (3s)

1. Click the download icon next to the file.
2. Brief pause to show the download initiated.

### Scene 6 — End (2s)

1. Hold on the bucket detail page for a moment to close cleanly.

## Recording tips

- Use a clean browser profile (no extensions visible).
- Set browser to light mode for better GIF contrast.
- Move the mouse deliberately and slowly — fast cursors look janky in GIFs.
- If using `mcp__claude-in-chrome__gif_creator`, capture extra frames before
  and after each action for smooth playback.
- Use a tool like `gifsicle` or `ffmpeg` to optimize the final GIF size.

## Automated recording

An automated Puppeteer script can record the demo with a mock API (no Docker
required). It drives a real browser through the 5-scene sequence above and
encodes the screenshots into a GIF.

```bash
cd scripts/record-demo
npm install            # installs Puppeteer + Chrome + GIF encoder
npm run record         # outputs docs/assets/demo.gif
```

Requirements: Node.js 20+ and a machine that can run headless Chrome (not a
minimal container). The script starts its own mock API and Vite dev server
automatically.

## Embed location

README.md already references this file:

```html
<p align="center">
  <img src="docs/assets/demo.gif" alt="SFree demo — create a source, upload a file, browse and download" width="720" />
</p>
```
