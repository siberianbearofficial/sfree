/**
 * Flow 5: Error states render correctly.
 *
 * Covers:
 * - "Bucket not found" when navigating to a non-existent bucket ID
 * - Landing page auth dialogs open and close without crashing
 * - Register flow shows password after API call
 * - Login flow closes dialog and persists auth
 */

import { test, expect } from "@playwright/test";
import { injectAuth, mockGet, mockPost, API_GLOB } from "./helpers";

test.describe("Error states", () => {
  test("navigating to a non-existent bucket shows Bucket not found", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets/does-not-exist", { error: "not found" }, 404);
    await page.route("**/api/v1/buckets/does-not-exist/files*", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      await route.fulfill({status: 200, json: {items: []}});
    });
    await page.goto("/buckets/does-not-exist");

    await expect(page.getByText("Bucket not found")).toBeVisible();
  });

  test("Register dialog opens, creates user, and displays password", async ({
    page,
  }) => {
    // No auth — unauthenticated landing page
    await page.route(`${API_GLOB}/**`, (route) => route.abort());
    await mockPost(page, "/users", {
      id: "u-new",
      created_at: "2024-01-01T00:00:00Z",
      password: "generated-secret-pw",
    });

    await page.goto("/");
    await page.getByRole("button", { name: "Sign Up" }).first().click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(
      dialog
        .locator(":not(button)")
        .filter({ hasText: /^(Sign Up|Create a free SFree account)$/ })
        .first(),
    ).toBeVisible();

    await dialog.getByLabel("Username").fill("newuser");
    await dialog.getByRole("button", { name: "Create Account" }).click();

    await expect(dialog.getByText("generated-secret-pw")).toBeVisible();

    await page.route(`${API_GLOB}/auth/session`, (route) =>
      route.fulfill({status: 204, body: ""}),
    );
    await page.route(`${API_GLOB}/auth/me`, (route) =>
      route.fulfill({status: 200, json: {id: "u-new", username: "newuser"}}),
    );

    await dialog.getByRole("button", { name: "I saved my password" }).click();
    await expect(dialog).not.toBeVisible();
  });

  test("Login dialog opens and closes, storing credentials", async ({
    page,
  }) => {
    await page.route(`${API_GLOB}/**`, (route) => route.abort());

    await page.goto("/");
    await page.getByRole("button", { name: "Log In" }).first().click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    await dialog.getByLabel("Username").fill("alice");
    await dialog.getByLabel("Password").fill("secret");
    await dialog.getByRole("button", { name: "Log In" }).click();

    await expect(dialog).not.toBeVisible();

    const auth = await page.evaluate(() => localStorage.getItem("auth"));
    expect(auth).toBe(btoa("alice:secret"));
  });

  test("sources page renders without crashing when API returns error", async ({
    page,
  }) => {
    await injectAuth(page);
    // API fails — app catches the error and shows empty state
    await page.route("**/api/v1/sources", (route) =>
      route.fulfill({ status: 500, json: { error: "internal server error" } }),
    );

    await page.goto("/sources");

    // Page must not crash — heading and Add Source button should still be present
    await expect(
      page.getByRole("heading", { name: /^Sources$/, level: 1 }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add Source" }).first(),
    ).toBeVisible();
    // API errors should show the dedicated empty state title and retry action
    await expect(
      page.getByRole("heading", { name: "Failed to load sources" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
  });

  test("buckets page renders without crashing when API returns error", async ({
    page,
  }) => {
    await injectAuth(page);
    await page.route("**/api/v1/buckets", (route) =>
      route.fulfill({ status: 500, json: { error: "internal server error" } }),
    );

    await page.goto("/buckets");

    await expect(
      page.getByRole("heading", { name: /^Buckets$/, level: 1 }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /^(Add|Create) Bucket$/ }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Failed to load buckets" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Retry" })).toBeVisible();
  });
});
