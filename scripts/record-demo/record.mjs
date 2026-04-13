#!/usr/bin/env node
/**
 * SFree Demo GIF Recorder
 *
 * Records a product demo GIF by:
 *   1. Starting a lightweight mock API (no database needed)
 *   2. Starting the Vite dev server for the real frontend
 *   3. Driving Chrome through the demo flow via Puppeteer
 *   4. Encoding captured screenshots into an animated GIF
 *
 * Usage:
 *   cd scripts/record-demo
 *   npm install
 *   npm run record
 *
 * The output lands at docs/assets/demo.gif (already referenced in README.md).
 */

import { execSync, spawn } from "node:child_process";
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "../..");
const OUTPUT = path.join(ROOT, "docs/assets/demo.gif");

const MOCK_PORT = 4050;
const VITE_PORT = 3050;
const WIDTH = 1280;
const HEIGHT = 720;
const FRAME_DELAY = 500; // ms per frame in GIF

// ─── Mock API ──────────────────────────────────────────────────────────────────

const sources = [
  { id: "src-001", name: "local-minio", type: "s3", created_at: "2026-04-10T12:00:00Z" },
];
const sourceInfos = {
  "src-001": {
    id: "src-001", name: "local-minio", type: "s3",
    files: [
      { id: "sf-1", name: "backup-2026-04.tar.gz", size: 52428800 },
      { id: "sf-2", name: "logs.json", size: 1048576 },
    ],
    storage_total: 10737418240, storage_used: 53477376, storage_free: 10683940864,
  },
};
let buckets = [];
const bucketFiles = {};
let fileCounter = 0;

function json(res, data, status = 200) {
  res.writeHead(status, {
    "Content-Type": "application/json",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Headers": "*",
    "Access-Control-Allow-Methods": "*",
  });
  res.end(JSON.stringify(data));
}

function readBody(req) {
  return new Promise((resolve) => {
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks)));
  });
}

function startMockApi() {
  return new Promise((resolve) => {
    const server = http.createServer(async (req, res) => {
      const { method, url } = req;
      if (method === "OPTIONS") {
        res.writeHead(204, {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Headers": "*",
          "Access-Control-Allow-Methods": "*",
        });
        return res.end();
      }
      if (method === "GET" && url === "/api/v1/sources") return json(res, sources);
      const siMatch = url.match(/^\/api\/v1\/sources\/([^/]+)\/info$/);
      if (method === "GET" && siMatch) return json(res, sourceInfos[siMatch[1]] || { error: "not found" }, sourceInfos[siMatch[1]] ? 200 : 404);
      if (method === "GET" && url === "/api/v1/buckets") return json(res, buckets);
      if (method === "POST" && url === "/api/v1/buckets") {
        const body = JSON.parse(await readBody(req));
        const b = {
          id: "bkt-" + String(buckets.length + 1).padStart(3, "0"),
          key: body.key, access_key: "AKSFREE7X9MQ2KPL",
          created_at: new Date().toISOString(),
        };
        buckets.push(b);
        bucketFiles[b.id] = [];
        return json(res, { ...b, access_secret: "sfsec_d83ka1mxp7r2vn4t" }, 201);
      }
      const fMatch = url.match(/^\/api\/v1\/buckets\/([^/]+)\/files$/);
      if (method === "GET" && fMatch) return json(res, bucketFiles[fMatch[1]] || []);
      const uMatch = url.match(/^\/api\/v1\/buckets\/([^/]+)\/upload$/);
      if (method === "POST" && uMatch) {
        const raw = await readBody(req);
        const nameMatch = raw.toString("latin1").match(/filename="([^"]+)"/);
        const file = {
          id: "file-" + String(++fileCounter).padStart(3, "0"),
          name: nameMatch ? nameMatch[1] : "uploaded-file.txt",
          size: 42, created_at: new Date().toISOString(),
        };
        const bId = uMatch[1];
        if (!bucketFiles[bId]) bucketFiles[bId] = [];
        bucketFiles[bId].push(file);
        return json(res, file, 201);
      }
      const dMatch = url.match(/^\/api\/v1\/buckets\/([^/]+)\/files\/([^/]+)\/download$/);
      if (method === "GET" && dMatch) {
        res.writeHead(200, {
          "Content-Type": "application/octet-stream",
          "Content-Disposition": 'attachment; filename="hello.txt"',
          "Access-Control-Allow-Origin": "*",
        });
        return res.end("Hello from SFree!\n");
      }
      if (method === "POST" && url === "/api/v1/users") {
        const body = JSON.parse(await readBody(req));
        return json(res, { username: body.username, password: "demo-password" }, 201);
      }
      if (method === "DELETE") return json(res, {});
      json(res, { error: "not found" }, 404);
    });
    server.listen(MOCK_PORT, () => {
      console.log(`Mock API on http://localhost:${MOCK_PORT}`);
      resolve(server);
    });
  });
}

// ─── Vite dev server ───────────────────────────────────────────────────────────

function startVite() {
  return new Promise((resolve, reject) => {
    const webuiDir = path.join(ROOT, "webui");
    // Ensure webui deps are installed
    if (!fs.existsSync(path.join(webuiDir, "node_modules/.bin/vite"))) {
      console.log("Installing webui dependencies...");
      execSync("npm install", { cwd: webuiDir, stdio: "inherit", env: { ...process.env, NODE_ENV: "development" } });
    }
    const vite = spawn(
      path.join(webuiDir, "node_modules/.bin/vite"),
      ["--host", "0.0.0.0", "--port", String(VITE_PORT)],
      {
        cwd: webuiDir,
        env: { ...process.env, VITE_API_BASE: `http://localhost:${MOCK_PORT}/api/v1`, NODE_ENV: "development" },
        stdio: ["ignore", "pipe", "pipe"],
      }
    );
    let started = false;
    vite.stdout.on("data", (d) => {
      const line = d.toString();
      if (!started && line.includes("Local:")) {
        started = true;
        console.log(`Vite dev server on http://localhost:${VITE_PORT}`);
        resolve(vite);
      }
    });
    vite.stderr.on("data", (d) => {
      if (!started) console.error("  vite stderr:", d.toString().trim());
    });
    vite.on("exit", (code) => {
      if (!started) reject(new Error(`Vite exited with code ${code}`));
    });
    setTimeout(() => {
      if (!started) reject(new Error("Vite did not start within 30s"));
    }, 30000);
  });
}

// ─── Recording ─────────────────────────────────────────────────────────────────

async function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function record() {
  const puppeteer = await import("puppeteer");
  const gifenc = await import("gifenc");
  const { PNG } = await import("pngjs");
  const { GIFEncoder, quantize, applyPalette } = gifenc.default || gifenc;

  console.log("\nLaunching Chrome...");
  const browser = await puppeteer.default.launch({
    headless: true,
    args: ["--no-sandbox", "--disable-setuid-sandbox", "--disable-gpu", "--disable-dev-shm-usage", `--window-size=${WIDTH},${HEIGHT}`],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: WIDTH, height: HEIGHT });

  const BASE = `http://localhost:${VITE_PORT}`;

  // Set auth (start "logged in")
  await page.goto(BASE, { waitUntil: "networkidle0", timeout: 30000 });
  await page.evaluate(() => {
    localStorage.setItem("auth", btoa("demo-user:demo-pass"));
    localStorage.setItem("auth_type", "basic");
    localStorage.setItem("username", "demo-user");
  });

  const frames = [];
  async function capture(label, count = 1) {
    for (let i = 0; i < count; i++) {
      frames.push(await page.screenshot({ type: "png" }));
    }
    console.log(`  [${frames.length}] ${label}`);
  }

  async function clickButtonByText(text) {
    const buttons = await page.$$("button");
    for (const btn of buttons) {
      const t = await page.evaluate((el) => el.textContent.trim(), btn);
      if (t === text) { await btn.click(); return true; }
    }
    return false;
  }

  // ─── Scene 1: Sources page (pause 3s worth of frames) ───
  console.log("\n--- Scene 1: Sources page ---");
  await page.goto(`${BASE}/sources`, { waitUntil: "networkidle0" });
  await sleep(1000);
  await capture("Sources page showing local-minio", 6);

  // ─── Scene 2: Create a bucket ───
  console.log("\n--- Scene 2: Create bucket ---");
  await page.goto(`${BASE}/buckets`, { waitUntil: "networkidle0" });
  await sleep(500);
  await capture("Buckets page (empty)", 3);

  await clickButtonByText("Add Bucket");
  await sleep(1000);
  await capture("Create Bucket dialog", 3);

  // Type bucket name into the Key input
  const keyInput = await page.$("input");
  if (keyInput) {
    await keyInput.click();
    await keyInput.type("demo-bucket", { delay: 60 });
  }
  await sleep(400);
  await capture("Typed bucket key", 3);

  // Select the local-minio checkbox
  const labels = await page.$$("[data-slot='label'], label");
  for (const lbl of labels) {
    const t = await page.evaluate((el) => el.textContent, lbl);
    if (t && t.includes("local-minio")) { await lbl.click(); break; }
  }
  await sleep(300);
  await capture("Source selected", 2);

  await clickButtonByText("Create");
  await sleep(1500);
  await capture("Credentials shown", 6);

  // Close dialog
  await clickButtonByText("Close");
  await sleep(500);

  // Reload to see bucket
  await page.goto(`${BASE}/buckets`, { waitUntil: "networkidle0" });
  await sleep(500);
  await capture("Buckets list with new bucket", 4);

  // ─── Scene 3: Upload a file ───
  console.log("\n--- Scene 3: Upload file ---");
  // Click the bucket card (pressable Card)
  const cards = await page.$$("[data-slot='base']");
  for (const card of cards) {
    const t = await page.evaluate((el) => el.textContent, card);
    if (t && t.includes("demo-bucket")) { await card.click(); break; }
  }
  await sleep(1000);
  await capture("Bucket detail (empty)", 3);

  // Upload via hidden file input
  const tmpFile = path.join(__dirname, "hello.txt");
  fs.writeFileSync(tmpFile, "Hello from SFree! This is a demo file.\n");
  const fileInput = await page.$('input[type="file"]');
  if (fileInput) {
    await fileInput.uploadFile(tmpFile);
    await sleep(1500);
  }
  await capture("File uploaded successfully", 5);

  // ─── Scene 4: Browse files ───
  console.log("\n--- Scene 4: Browse files ---");
  await capture("File list with uploaded file", 5);

  // ─── Scene 5: Download ───
  console.log("\n--- Scene 5: Download ---");
  // Find and click the download button (first icon button in the file row)
  const iconBtns = await page.$$("button");
  for (const btn of iconBtns) {
    const hasDownloadIcon = await page.evaluate((el) => {
      return el.querySelector("svg") !== null &&
        el.closest("tr") !== null &&
        !el.className.includes("danger");
    }, btn);
    if (hasDownloadIcon) { await btn.click(); break; }
  }
  await sleep(500);
  await capture("Download initiated", 4);

  // ─── Scene 6: End hold ───
  console.log("\n--- Scene 6: End ---");
  await capture("Final hold", 4);

  // ─── Encode GIF ───
  console.log(`\nEncoding ${frames.length} frames...`);
  const gif = GIFEncoder();

  for (let i = 0; i < frames.length; i++) {
    const png = PNG.sync.read(frames[i]);
    const palette = quantize(png.data, 256);
    const index = applyPalette(png.data, palette);
    gif.writeFrame(index, png.width, png.height, { palette, delay: FRAME_DELAY });
    if ((i + 1) % 10 === 0) console.log(`  ${i + 1}/${frames.length}`);
  }
  gif.finish();

  fs.mkdirSync(path.dirname(OUTPUT), { recursive: true });
  fs.writeFileSync(OUTPUT, Buffer.from(gif.bytes()));
  const sizeMB = (fs.statSync(OUTPUT).size / 1024 / 1024).toFixed(2);
  console.log(`\nGIF saved: ${OUTPUT} (${sizeMB} MB)`);

  // Cleanup
  fs.unlinkSync(tmpFile);
  await browser.close();
  return sizeMB;
}

// ─── Main ──────────────────────────────────────────────────────────────────────

async function main() {
  console.log("SFree Demo GIF Recorder\n");

  const mockServer = await startMockApi();
  const viteProc = await startVite();

  try {
    const sizeMB = await record();
    console.log(`\nDone! GIF is ${sizeMB} MB.`);
    if (parseFloat(sizeMB) > 10) {
      console.log("Warning: GIF exceeds 10 MB target. Consider optimizing with gifsicle:");
      console.log("  gifsicle -O3 --lossy=80 docs/assets/demo.gif -o docs/assets/demo.gif");
    }
  } finally {
    viteProc.kill();
    mockServer.close();
  }
}

main().catch((err) => {
  console.error("\nFailed:", err.message);
  process.exit(1);
});
