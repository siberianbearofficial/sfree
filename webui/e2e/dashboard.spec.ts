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
    await expect(
      page.getByText("Buckets", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByText("Sources", { exact: true }).first(),
    ).toBeVisible();
    await expect(page.getByText("Manage storage buckets")).toBeVisible();
    await expect(page.getByText("Configure data sources")).toBeVisible();
  });

  test("Manage Buckets button navigates to /buckets", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await page.goto("/dashboard");
    await page.getByText("Manage Buckets", { exact: true }).click();
    await expect(page).toHaveURL("/buckets");
    await expect(
      page.getByRole("heading", { name: "Buckets" }),
    ).toBeVisible();
  });

  test("Manage Sources button navigates to /sources", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await page.goto("/dashboard");
    await page.getByText("Manage Sources", { exact: true }).click();
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
