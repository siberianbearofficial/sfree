/**
 * Flow 1: Dashboard loads correctly.
 *
 * Covers:
 * - Authenticated user at "/" redirects to "/dashboard"
 * - Dashboard heading and navigation cards render
 * - "Open" buttons navigate to /buckets and /sources
 * - Unauthenticated user at "/" sees the landing page
 */

import { test, expect, type Page } from "@playwright/test";
import { injectAuth, mockGet, API_GLOB } from "./helpers";

async function expectDashboardNav(
  page: Page,
  name: "Sources" | "Buckets",
) {
  const pattern =
    name === "Sources"
      ? /^(Manage Sources|Sources)$/
      : /^(Manage Buckets|Buckets)$/;
  await expect(
    page.locator("a, button").filter({ hasText: pattern }).first(),
  ).toBeVisible();
}

async function clickDashboardNav(
  page: Page,
  name: "Sources" | "Buckets",
) {
  const pattern =
    name === "Sources"
      ? /^(Manage Sources|Sources)$/
      : /^(Manage Buckets|Buckets)$/;
  await page.locator("a, button").filter({ hasText: pattern }).first().click();
}

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
    await expectDashboardNav(page, "Sources");
    await expectDashboardNav(page, "Buckets");
  });

  test("Manage Buckets button navigates to /buckets", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await page.goto("/dashboard");
    await clickDashboardNav(page, "Buckets");
    await expect(page).toHaveURL("/buckets");
    await expect(
      page.getByRole("heading", { name: /^Buckets$/, level: 1 }),
    ).toBeVisible();
  });

  test("Manage Sources button navigates to /sources", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await page.goto("/dashboard");
    await clickDashboardNav(page, "Sources");
    await expect(page).toHaveURL("/sources");
    await expect(
      page.getByRole("heading", { name: /^Sources$/, level: 1 }),
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
