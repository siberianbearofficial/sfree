/**
 * Flow 1: Dashboard loads correctly.
 *
 * Covers:
 * - Authenticated user at "/" redirects to "/dashboard"
 * - Dashboard heading and navigation cards render
 * - "Open" buttons navigate to /buckets and /sources
 * - Unauthenticated user at "/" sees the landing page
 */

import { test, expect } from "@playwright/test";
import { injectAuth, mockGet, API_GLOB } from "./helpers";

test.describe("Dashboard", () => {
  test("authenticated user at / redirects to /dashboard", async ({ page }) => {
    await injectAuth(page);
    await page.goto("/");
    await expect(page).toHaveURL("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  test("dashboard shows Buckets and Sources cards", async ({ page }) => {
    await injectAuth(page);
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
    const summaryCards = page.locator(".grid.gap-4.grid-cols-2.lg\\:grid-cols-4");
    await expect(summaryCards.getByText("Sources", { exact: true })).toBeVisible();
    await expect(summaryCards.getByText("Buckets", { exact: true })).toBeVisible();
    await expect(
      summaryCards.getByText("Files", { exact: true }),
    ).toBeVisible();
    await expect(
      summaryCards.getByText("Storage Used", { exact: true }),
    ).toBeVisible();
    await expect(
      page
        .locator("a, button")
        .filter({ hasText: /^Manage Sources$/ }),
    ).toBeVisible();
    await expect(
      page
        .locator("a, button")
        .filter({ hasText: /^Manage Buckets$/ }),
    ).toBeVisible();
  });

  test("Manage Buckets button navigates to /buckets", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await page.goto("/dashboard");
    await page.locator("a, button").filter({ hasText: /^Manage Buckets$/ }).click();
    await expect(page).toHaveURL("/buckets");
    await expect(
      page.getByRole("heading", { name: "Buckets" }),
    ).toBeVisible();
  });

  test("Manage Sources button navigates to /sources", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await page.goto("/dashboard");
    await page.locator("a, button").filter({ hasText: /^Manage Sources$/ }).click();
    await expect(page).toHaveURL("/sources");
    await expect(
      page.getByRole("heading", { name: "Sources" }),
    ).toBeVisible();
  });

  test("unauthenticated user at / sees landing page", async ({ page }) => {
    // No injectAuth — localStorage is empty
    await page.route(`${API_GLOB}/**`, (route) => route.abort());
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: "Free Distributed Object Storage" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Sign Up" }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Log In" }),
    ).toBeVisible();
  });
});
