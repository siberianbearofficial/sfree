/**
 * Flow 1: Dashboard loads correctly.
 *
 * Covers:
 * - Authenticated user at "/" redirects to "/dashboard"
 * - Fresh accounts see the onboarding hero on the dashboard
 * - Returning users with sources and buckets see the normal dashboard view
 * - Unauthenticated user at "/" sees the landing page
 */

import { test, expect } from "@playwright/test";
import { injectAuth, mockGet, API_GLOB } from "./helpers";

const MOCK_GDRIVE_SOURCE = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  created_at: "2024-01-15T10:00:00Z",
};

const MOCK_GDRIVE_INFO = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  files: [{id: "file-1", name: "hello.txt", size: 12}],
  storage_total: 1024 * 1024,
  storage_used: 512 * 1024,
  storage_free: 512 * 1024,
};

const MOCK_GDRIVE_HEALTH = {
  id: "src-1",
  type: "gdrive",
  status: "healthy",
  checked_at: "2024-01-15T10:10:00Z",
  latency_ms: 25,
  reason_code: "ok",
  message: "Google Drive metadata is reachable.",
  quota_total_bytes: 1024 * 1024,
  quota_used_bytes: 512 * 1024,
  quota_free_bytes: 512 * 1024,
};

const MOCK_S3_SOURCE = {
  id: "src-2",
  name: "Archive Bucket",
  type: "s3",
  created_at: "2024-01-15T10:30:00Z",
};

const MOCK_S3_INFO = {
  id: "src-2",
  name: "Archive Bucket",
  type: "s3",
  files: [{id: "obj-1", name: "backup.tar", size: 256}],
  storage_total: 0,
  storage_used: 256,
  storage_free: 0,
};

const MOCK_S3_HEALTH = {
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

const MOCK_BUCKET = {
  id: "bkt-1",
  key: "first-bucket",
  access_key: "AK123",
  created_at: "2024-01-15T11:00:00Z",
  role: "owner",
  shared: false,
};

test.describe("Dashboard", () => {
  test("authenticated user at / redirects to /dashboard", async ({ page }) => {
    await injectAuth(page);
    await page.goto("/");
    await expect(page).toHaveURL("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  test("fresh account dashboard shows onboarding hero", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await mockGet(page, "/buckets", []);
    await page.goto("/dashboard");

    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Get started with SFree" })).toBeVisible();
    await expect(page.getByText("Three steps to your first upload.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Add Source" })).toBeVisible();

    const summaryCards = page.locator(".grid.gap-4.grid-cols-2.lg\\:grid-cols-4");
    await expect(summaryCards.getByText("Sources", { exact: true })).toBeVisible();
    await expect(summaryCards.getByText("Buckets", { exact: true })).toBeVisible();
    await expect(
      summaryCards.getByText("Files", { exact: true }),
    ).toBeVisible();
    await expect(
      summaryCards.getByText("Sources Reporting Quota", { exact: true }),
    ).toBeVisible();
  });

  test("returning user dashboard hides onboarding and shows honest quota states", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", [MOCK_GDRIVE_SOURCE, MOCK_S3_SOURCE]);
    await mockGet(page, "/sources/src-1/info", MOCK_GDRIVE_INFO);
    await mockGet(page, "/sources/src-1/health", MOCK_GDRIVE_HEALTH);
    await mockGet(page, "/sources/src-2/info", MOCK_S3_INFO);
    await mockGet(page, "/sources/src-2/health", MOCK_S3_HEALTH);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await page.goto("/dashboard");

    await expect(
      page.getByRole("heading", { name: "Dashboard" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Get started with SFree" }),
    ).not.toBeVisible();
    await expect(
      page.getByRole("heading", { name: /^Sources$/, level: 2 }),
    ).toBeVisible();
    await expect(
      page.getByText("My Drive", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Archive Bucket", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Sources Reporting Quota", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("1/2", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Quota unavailable", { exact: true }),
    ).toBeVisible();
  });

  test("unauthenticated user at / sees landing page", async ({ page }) => {
    // No injectAuth — session lookup fails and the landing page stays visible
    await page.route(`${API_GLOB}/**`, (route) => route.abort());
    await page.goto("/");
    await expect(
      page.getByRole("heading", {
        name: /^(Free Distributed Object Storage|Unify your free storage into one bucket)$/,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /^(Sign Up|Get Started Free)$/ }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Log In" }).first(),
    ).toBeVisible();
  });
});
