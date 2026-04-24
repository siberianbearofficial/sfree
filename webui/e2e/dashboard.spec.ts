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

const MOCK_SOURCE = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  created_at: "2024-01-15T10:00:00Z",
};

const MOCK_SOURCE_INFO = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  files: [{id: "file-1", name: "hello.txt", size: 12}],
  storage_total: 1024,
  storage_used: 12,
  storage_free: 1012,
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
      summaryCards.getByText("Storage Used", { exact: true }),
    ).toBeVisible();
  });

  test("returning user dashboard hides onboarding and shows sources section", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", [MOCK_SOURCE]);
    await mockGet(page, "/sources/src-1/info", MOCK_SOURCE_INFO);
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
  });

  test("unauthenticated user at / sees landing page", async ({ page }) => {
    // No injectAuth — localStorage is empty
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
      page.getByRole("button", { name: "Log In" }),
    ).toBeVisible();
  });
});
