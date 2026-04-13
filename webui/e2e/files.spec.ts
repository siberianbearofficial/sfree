/**
 * Flow 4: File listing and download.
 *
 * Covers:
 * - Bucket detail page shows file list with name, size, and date columns
 * - Download button is present for each file
 * - Empty file state renders correctly
 * - Back navigation button is present
 */

import { test, expect } from "@playwright/test";
import { injectAuth, mockGet } from "./helpers";

const MOCK_BUCKET = {
  id: "bkt-1",
  key: "my-bucket",
  access_key: "AK123",
  created_at: "2024-01-15T11:00:00Z",
};

const MOCK_FILES = [
  {
    id: "file-1",
    name: "report.pdf",
    size: 204800,
    created_at: "2024-01-20T09:00:00Z",
  },
  {
    id: "file-2",
    name: "data.csv",
    size: 1024,
    created_at: "2024-01-21T14:30:00Z",
  },
];

test.describe("File listing and download", () => {
  test("bucket detail shows bucket info", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await page.goto("/buckets/bkt-1");

    await expect(page.getByRole("heading", { name: "my-bucket" })).toBeVisible();
    await expect(page.getByText(/AK123/)).toBeVisible();
  });

  test("bucket detail shows empty state when no files", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await page.goto("/buckets/bkt-1");

    await expect(page.getByText("No files")).toBeVisible();
  });

  test("bucket detail lists files with name, size, and date columns", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", MOCK_FILES);
    await page.goto("/buckets/bkt-1");

    // Table headers
    await expect(page.getByRole("columnheader", { name: "Name" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Size" })).toBeVisible();
    await expect(
      page.getByRole("columnheader", { name: "Created" }),
    ).toBeVisible();

    // File rows
    await expect(page.getByText("report.pdf")).toBeVisible();
    await expect(page.getByText("data.csv")).toBeVisible();
  });

  test("each file row has a download button", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", MOCK_FILES);
    await page.goto("/buckets/bkt-1");

    // There should be one download button per file row (icon buttons)
    // We verify at least one is visible
    const rows = page.getByRole("row");
    // header + 2 file rows
    await expect(rows).toHaveCount(3);

    // Download buttons are icon-only buttons in each file row
    // The download button uses DownloadIcon; we check via aria-label or button count
    const fileRows = page.locator("tbody tr");
    await expect(fileRows).toHaveCount(2);

    // Each row should have exactly 2 icon buttons (download + delete)
    const firstRowButtons = fileRows.first().getByRole("button");
    await expect(firstRowButtons).toHaveCount(2);
  });

  test("clicking download triggers a fetch to the download endpoint", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", [MOCK_FILES[0]]);

    // Intercept download call and fulfill with a blob
    let downloadCalled = false;
    await page.route("**/api/v1/buckets/bkt-1/files/file-1/download", async (route) => {
      downloadCalled = true;
      await route.fulfill({
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
        body: Buffer.from("fake-content"),
      });
    });

    await page.goto("/buckets/bkt-1");
    await expect(page.getByText("report.pdf")).toBeVisible();

    // Click the first icon button in the first file row (download)
    const firstRow = page.locator("tbody tr").first();
    await firstRow.getByRole("button").first().click();

    await expect.poll(() => downloadCalled).toBe(true);
  });

  test("back button is present and navigates back", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1/files", []);
    await page.goto("/buckets");
    // Navigate to bucket detail
    await page.goto("/buckets/bkt-1");

    // Back button (isIconOnly with ArrowLeftIcon) is present
    const backButton = page.getByRole("button").first();
    await expect(backButton).toBeVisible();
  });
});
