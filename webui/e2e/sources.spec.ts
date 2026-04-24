/**
 * Flow 2: Source creation flow.
 *
 * Covers:
 * - Sources page renders with empty state
 * - "Add Source" button opens the create-source dialog
 * - Filling the form and submitting calls the API and refreshes the list
 * - Newly created source appears in the list
 */

import { test, expect, type Page } from "@playwright/test";
import { injectAuth, mockGet, mockPost } from "./helpers";

const MOCK_SOURCE = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  key: "service-account-key",
  created_at: "2024-01-15T10:00:00Z",
};

test.describe("Source creation flow", () => {
  async function openCreateSourceDialog(
    page: Page,
  ) {
    await page.getByRole("button", { name: "Add Source" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();

    const providerPicker = dialog.getByText("Connect a Source");
    if (await providerPicker.isVisible().catch(() => false)) {
      await dialog.getByText("Google Drive", { exact: true }).first().click();
      await expect(
        dialog.getByLabel(/^(Name|Source Name)$/),
      ).toBeVisible();
    }

    return dialog;
  }

  test("sources page shows empty state when no sources exist", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await page.goto("/sources");
    await expect(
      page.getByRole("heading", { name: /^Sources$/, level: 1 }),
    ).toBeVisible();
    await expect(page.getByText("No sources yet")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add Source" }).first(),
    ).toBeVisible();
  });

  test("Add Source button opens the create source dialog", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", []);
    await page.goto("/sources");
    const dialog = await openCreateSourceDialog(page);
    await expect(
      dialog.getByText(/^(Create Source|Connect a Source|Connect Google Drive)$/),
    ).toBeVisible();
    await expect(dialog.getByLabel(/^(Name|Source Name)$/)).toBeVisible();
    await expect(dialog.getByLabel("Service Account Key (JSON)")).toBeVisible();
  });

  test("submitting the dialog creates a source and refreshes the list", async ({
    page,
  }) => {
    await injectAuth(page);

    // Before creation: empty list
    let listCallCount = 0;
    await page.route("**/api/v1/sources", async (route) => {
      if (route.request().method() === "GET") {
        // First call returns empty; subsequent calls return the new source
        const body = listCallCount === 0 ? [] : [MOCK_SOURCE];
        listCallCount++;
        await route.fulfill({ json: body });
      } else {
        await route.continue();
      }
    });
    await mockPost(page, "/sources/gdrive", MOCK_SOURCE);

    await page.goto("/sources");
    await expect(page.getByText("No sources yet")).toBeVisible();

    // Open dialog
    const dialog = await openCreateSourceDialog(page);

    // Fill form
    await dialog.getByLabel(/^(Name|Source Name)$/).fill("My Drive");
    await dialog
      .getByLabel("Service Account Key (JSON)")
      .fill("{\"type\":\"service_account\",\"project_id\":\"demo-project\"}");

    // Submit
    await dialog
      .getByRole("button", { name: "Create" })
      .or(dialog.getByRole("button", { name: "Connect Source" }))
      .click();

    await expect(page.getByRole("dialog")).not.toBeVisible();
    await expect(page.getByText("My Drive", { exact: true })).toBeVisible();
  });

  test("sources page shows existing sources", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/sources", [MOCK_SOURCE]);
    await page.goto("/sources");
    await expect(
      page.getByText("My Drive", { exact: true }),
    ).toBeVisible();
  });
});
