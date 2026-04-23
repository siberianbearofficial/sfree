/**
 * Flow 3: Bucket creation and file upload.
 *
 * Covers:
 * - Buckets page renders with empty state
 * - "Add Bucket" dialog loads available sources
 * - Selecting a source and submitting creates a bucket and shows credentials
 * - Upload File button is present on the bucket detail page
 */

import { test, expect, type Route } from "@playwright/test";
import { API_GLOB, injectAuth, mockGet, mockPost } from "./helpers";

const MOCK_SOURCE = {
  id: "src-1",
  name: "My Drive",
  type: "gdrive",
  key: "k",
  created_at: "2024-01-15T10:00:00Z",
};

const MOCK_BUCKET = {
  id: "bkt-1",
  key: "my-bucket",
  access_key: "AK123",
  created_at: "2024-01-15T11:00:00Z",
  role: "owner",
  shared: false,
};

const MOCK_VIEWER_BUCKET = {
  ...MOCK_BUCKET,
  role: "viewer",
  shared: true,
};

const MOCK_BUCKET_CREDS = {
  key: "my-bucket",
  access_key: "AK123",
  access_secret: "SK456",
  created_at: "2024-01-15T11:00:00Z",
};

const MOCK_GRANT = {
  id: "grant-1",
  bucket_id: "bkt-1",
  user_id: "user-2",
  username: "shared-user",
  role: "viewer",
  granted_by: "user-1",
  created_at: "2024-01-15T12:00:00Z",
};

test.describe("Bucket creation flow", () => {
  test("buckets page shows empty state when no buckets exist", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await page.goto("/buckets");
    await expect(
      page.getByRole("heading", { name: /^Buckets$/, level: 1 }),
    ).toBeVisible();
    await expect(page.getByText("No buckets yet")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add Bucket" }).first(),
    ).toBeVisible();
  });

  test("Add Bucket dialog loads available sources", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await mockGet(page, "/sources", [MOCK_SOURCE]);
    await page.goto("/buckets");
    const dialog = page.getByRole("dialog");

    await page.getByRole("button", { name: "Add Bucket" }).first().click();
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByText("Create Bucket"),
    ).toBeVisible();
    await expect(page.getByLabel("Key")).toBeVisible();

    // Source checkbox should appear after sources load
    await expect(dialog.getByText("My Drive", { exact: true })).toBeVisible();
  });

  test("Add Bucket dialog surfaces source loading failures and retries", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    let sourceRequests = 0;
    await page.route("**/api/v1/sources", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      sourceRequests += 1;
      if (sourceRequests === 1) {
        await route.fulfill({ status: 500, json: { error: "Source service unavailable" } });
        return;
      }
      await route.fulfill({ status: 200, json: [MOCK_SOURCE] });
    });

    await page.goto("/buckets");
    const dialog = page.getByRole("dialog");

    await page.getByRole("button", { name: "Add Bucket" }).first().click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("Sources could not be loaded. Retry to try again.")).toBeVisible();
    await expect(dialog.getByText("Source service unavailable")).toBeVisible();
    await expect(dialog.getByText("Create at least one source before creating a bucket.")).not.toBeVisible();

    await dialog.getByRole("button", { name: "Retry" }).click();
    await expect(dialog.getByText("My Drive", { exact: true })).toBeVisible();
  });

  test("creating a bucket shows S3 credentials", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", []);
    await mockGet(page, "/sources", [MOCK_SOURCE]);
    await mockPost(page, "/buckets", MOCK_BUCKET_CREDS);

    await page.goto("/buckets");
    const dialog = page.getByRole("dialog");
    await page.getByRole("button", { name: "Add Bucket" }).first().click();
    await expect(dialog).toBeVisible();

    // Wait for sources to load in dialog
    await expect(dialog.getByText("My Drive", { exact: true })).toBeVisible();

    // Fill key
    await page.getByLabel("Key").fill("my-bucket");

    // Select the source checkbox
    await page.getByLabel("My Drive").check();

    // Submit
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Create" })
      .click();

    // Credentials screen: access key and secret should be visible
    await expect(page.getByText("AK123")).toBeVisible();
    await expect(page.getByText("SK456")).toBeVisible();
    await expect(
      page.getByText(/copy these credentials/i),
    ).toBeVisible();

    // Close button dismisses the dialog
    await dialog
      .getByRole("button", { name: "Close" })
      .last()
      .click();
    await expect(page.getByRole("dialog")).not.toBeVisible();
  });

  test("owner bucket detail shows owner actions", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await page.goto("/buckets/bkt-1");

    await expect(page.getByRole("button", { name: "Upload File" })).toHaveCount(2);
    await expect(page.getByRole("button", { name: "Share Bucket" })).toBeVisible();
    await expect(page.getByText("Drag and drop a file here")).toBeVisible();
  });

  test("Share Bucket dialog shows grant loading and successful grants", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", []);

    let fulfillGrants: (() => Promise<void>) | undefined;
    await page.route(`${API_GLOB}/buckets/bkt-1/grants`, async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      fulfillGrants = () => route.fulfill({status: 200, json: [MOCK_GRANT]});
    });

    await page.goto("/buckets/bkt-1");
    const dialog = page.getByRole("dialog");

    await page.getByRole("button", { name: "Share Bucket" }).click();
    await expect(dialog.getByText("Loading people with access")).toBeVisible();

    expect(fulfillGrants).toBeDefined();
    await fulfillGrants!();
    await expect(dialog.getByText("People with access")).toBeVisible();
    await expect(dialog.getByText("shared-user")).toBeVisible();
  });

  test("Share Bucket dialog shows grant-list failure state", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await mockGet(
      page,
      "/buckets/bkt-1/grants",
      {error: "Grant store unavailable"},
      500,
    );

    await page.goto("/buckets/bkt-1");
    const dialog = page.getByRole("dialog");

    await page.getByRole("button", { name: "Share Bucket" }).click();
    await expect(dialog.getByText("Access list failed to load")).toBeVisible();
    await expect(dialog.getByText("Grant store unavailable")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Retry" })).toBeVisible();
    await expect(dialog.getByText("People with access")).not.toBeVisible();
  });

  test("Share Bucket dialog ignores stale grant-list failures", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", []);

    const grantRoutes: Route[] = [];
    await page.route(`${API_GLOB}/buckets/bkt-1/grants`, async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      grantRoutes.push(route);
    });

    await page.goto("/buckets/bkt-1");
    const dialog = page.getByRole("dialog");

    await page.getByRole("button", { name: "Share Bucket" }).click();
    await expect(dialog.getByText("Loading people with access")).toBeVisible();
    await expect.poll(() => grantRoutes.length).toBe(1);
    await dialog.getByRole("button", { name: "Close" }).last().click();
    await expect(dialog).not.toBeVisible();

    await page.getByRole("button", { name: "Share Bucket" }).click();
    await expect.poll(() => grantRoutes.length).toBe(2);
    await grantRoutes[1].fulfill({status: 200, json: [MOCK_GRANT]});
    await expect(dialog.getByText("shared-user")).toBeVisible();

    await grantRoutes[0].fulfill({
      status: 500,
      json: {error: "Stale grant failure"},
    });
    await expect(dialog.getByText("shared-user")).toBeVisible();
    await expect(dialog.getByText("Stale grant failure")).not.toBeVisible();
  });

  test("viewer bucket detail hides write actions", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_VIEWER_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_VIEWER_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await page.goto("/buckets/bkt-1");

    await expect(page.getByRole("button", { name: "Upload File" })).not.toBeVisible();
    await expect(page.getByRole("button", { name: "Share Bucket" })).not.toBeVisible();
    await expect(page.getByText("Drag and drop a file here")).not.toBeVisible();
    await expect(page.getByText("Files shared in this bucket will appear here.")).toBeVisible();
  });
});
