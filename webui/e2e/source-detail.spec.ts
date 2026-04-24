import {expect, test} from "@playwright/test";
import {injectAuth, mockGet} from "./helpers";

const GDRIVE_INFO = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  files: [{id: "file-1", name: "hello.txt", size: 12}],
  storage_total: 1024 * 1024,
  storage_used: 512 * 1024,
  storage_free: 512 * 1024,
};

const GDRIVE_HEALTH = {
  id: "src-1",
  type: "gdrive",
  status: "degraded",
  checked_at: "2024-01-15T10:10:00Z",
  latency_ms: 25,
  reason_code: "quota_low",
  message: "Google Drive quota is nearly exhausted.",
  quota_total_bytes: 1024 * 1024,
  quota_used_bytes: 980 * 1024,
  quota_free_bytes: 44 * 1024,
};

const S3_INFO = {
  id: "src-2",
  name: "Archive Bucket",
  type: "s3",
  files: [{id: "obj-1", name: "backup.tar", size: 256}],
  storage_total: 0,
  storage_used: 256,
  storage_free: 0,
};

const S3_HEALTH = {
  id: "src-2",
  type: "s3",
  status: "healthy",
  checked_at: "2024-01-15T10:35:00Z",
  latency_ms: 11,
  reason_code: "ok",
  message: "S3 bucket metadata is reachable.",
  quota_total_bytes: null,
  quota_used_bytes: null,
  quota_free_bytes: null,
};

test.describe("Source detail", () => {
  test("shows provider-native quota when available", async ({page}) => {
    await injectAuth(page);
    await mockGet(page, "/sources/src-1/info", GDRIVE_INFO);
    await mockGet(page, "/sources/src-1/health", GDRIVE_HEALTH);

    await page.goto("/sources/src-1");

    await expect(page.getByRole("heading", {name: "My Drive"})).toBeVisible();
    await expect(page.getByText("Native quota", {exact: true})).toBeVisible();
    await expect(page.getByText("Stored In Source", {exact: true})).toBeVisible();
    await expect(page.getByText("Google Drive quota is nearly exhausted.")).toBeVisible();
  });

  test("shows explicit unavailable state when quota is unsupported", async ({page}) => {
    await injectAuth(page);
    await mockGet(page, "/sources/src-2/info", S3_INFO);
    await mockGet(page, "/sources/src-2/health", S3_HEALTH);

    await page.goto("/sources/src-2");

    await expect(page.getByRole("heading", {name: "Archive Bucket"})).toBeVisible();
    await expect(page.getByText("Quota unavailable", {exact: true})).toBeVisible();
    await expect(
      page.getByText("S3-compatible sources are checked for reachability here, not provider-wide capacity limits."),
    ).toBeVisible();
  });
});
