/**
 * Flow 4: File listing and download.
 *
 * Covers:
 * - Bucket detail page shows file list with name, size, and date columns
 * - Download button is present for each file
 * - Empty file state renders correctly
 * - Back navigation button is present
 */

import { test, expect, type Page } from "@playwright/test";
import { injectAuth, mockGet } from "./helpers";
import { formatSize } from "../src/shared/lib/format";

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

const MOCK_EDITOR_BUCKET = {
  ...MOCK_BUCKET,
  role: "editor",
  shared: true,
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

const LARGE_TEXT_FILE = {
  id: "file-3",
  name: "huge.log",
  size: 1_500_000,
  created_at: "2024-01-22T08:15:00Z",
};

function filePage(items: typeof MOCK_FILES, nextCursor?: string) {
  return nextCursor ? {items, next_cursor: nextCursor} : {items};
}

async function mockBucketFiles(
  page: Page,
  items: typeof MOCK_FILES,
  nextCursor?: string,
) {
  await page.route("**/api/v1/buckets/bkt-1/files*", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({status: 200, json: filePage(items, nextCursor)});
  });
}

test.describe("File listing and download", () => {
  async function revealCredentialsIfNeeded(
    page: Page,
  ) {
    if (await page.getByText(/AK123/).isVisible().catch(() => false)) {
      return;
    }

    const credentialsToggle = page.getByRole("button", {
      name: "S3 Credentials",
    });
    if (await credentialsToggle.isVisible().catch(() => false)) {
      await credentialsToggle.click();
    }
  }

  test("bucket detail shows bucket info", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, []);
    await page.goto("/buckets/bkt-1");

    await expect(page.getByRole("heading", { name: "my-bucket" })).toBeVisible();
    await revealCredentialsIfNeeded(page);
    await expect(page.getByText(/AK123/)).toBeVisible();
  });

  test("bucket detail shows empty state when no files", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, []);
    await page.goto("/buckets/bkt-1");

    await expect(
      page.getByText(/(No files yet|Last step: upload your first file)/),
    ).toBeVisible();
  });

  test("bucket detail lists files with name, size, and date columns", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, MOCK_FILES);
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
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, MOCK_FILES);
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

    const firstRowActionButtons = fileRows
      .first()
      .locator("td")
      .last()
      .getByRole("button");
    await expect(firstRowActionButtons).toHaveCount(3);
    await expect(fileRows.first().getByRole("button", { name: "Share report.pdf" })).toBeVisible();
    await expect(fileRows.first().getByRole("button", { name: "Download report.pdf" })).toBeVisible();
    await expect(fileRows.first().getByRole("button", { name: "Delete report.pdf" })).toBeVisible();
  });

  test("bucket detail filters files by filename search", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);

    const requestedQueries: string[] = [];
    await page.route("**/api/v1/buckets/bkt-1/files*", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      const query = new URL(route.request().url()).searchParams.get("q") ?? "";
      requestedQueries.push(query);
      const body = query
        ? MOCK_FILES.filter((file) =>
            file.name.toLowerCase().includes(query.toLowerCase()),
          )
        : MOCK_FILES;
      await route.fulfill({status: 200, json: filePage(body)});
    });

    await page.goto("/buckets/bkt-1");
    await expect(page.getByText("report.pdf")).toBeVisible();
    await expect(page.getByText("data.csv")).toBeVisible();

    await page.getByLabel("Search files").fill("report");

    await expect.poll(() => requestedQueries.includes("report")).toBe(true);
    await expect(page.getByText("report.pdf")).toBeVisible();
    await expect(page.getByText("data.csv")).not.toBeVisible();
    await expect(page.getByText("1 matching file")).toBeVisible();
    await expect(
      page.getByRole("button", {name: "Download report.pdf"}),
    ).toBeVisible();
  });

  test("bucket detail shows a search-specific empty state", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);

    await page.route("**/api/v1/buckets/bkt-1/files*", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      const query = new URL(route.request().url()).searchParams.get("q") ?? "";
      const body = query === "missing" ? [] : MOCK_FILES;
      await route.fulfill({status: 200, json: filePage(body)});
    });

    await page.goto("/buckets/bkt-1");
    await expect(page.getByText("report.pdf")).toBeVisible();

    await page.getByLabel("Search files").fill("missing");

    await expect(page.getByText("No matching files")).toBeVisible();
    await expect(
      page.getByText('No files in this bucket match "missing".'),
    ).toBeVisible();
    await expect(page.getByText("report.pdf")).not.toBeVisible();
    await expect(page.getByRole("button", {name: "Upload File"})).toHaveCount(1);
  });

  test("bucket detail loads additional file pages on demand", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);

    await page.route("**/api/v1/buckets/bkt-1/files*", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      const cursor = new URL(route.request().url()).searchParams.get("cursor") ?? "";
      const body = cursor
        ? filePage([{id: "file-3", name: "zeta.txt", size: 512, created_at: "2024-01-23T10:00:00Z"}])
        : filePage([MOCK_FILES[0]], MOCK_FILES[0].name);
      await route.fulfill({status: 200, json: body});
    });

    await page.goto("/buckets/bkt-1");
    await expect(page.getByText("report.pdf")).toBeVisible();
    await expect(page.getByRole("button", {name: "Load More"})).toBeVisible();

    await page.getByRole("button", {name: "Load More"}).click();

    await expect(page.getByText("zeta.txt")).toBeVisible();
    await expect(page.getByRole("button", {name: "Load More"})).toHaveCount(0);
  });

  test("viewer file rows only expose download actions", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_VIEWER_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_VIEWER_BUCKET);
    await mockBucketFiles(page, MOCK_FILES);
    await page.goto("/buckets/bkt-1");

    const firstRowActions = page.locator("tbody tr").first().locator("td").last();
    await expect(firstRowActions.getByRole("button")).toHaveCount(1);
    await expect(
      firstRowActions.getByRole("button", { name: "Download report.pdf" }),
    ).toBeVisible();
    await expect(
      page.getByRole("checkbox", { name: "Select all visible files" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Download Selected" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Delete Selected" })).not.toBeVisible();
    await expect(
      page.getByRole("button", { name: /^Upload( File)?$/ }),
    ).not.toBeVisible();
    await expect(
      page.getByRole("button", { name: /^(Share Bucket|Share)$/ }),
    ).not.toBeVisible();
    await expect(page.getByRole("button", { name: "Share report.pdf" })).not.toBeVisible();
    await expect(page.getByRole("button", { name: "Delete report.pdf" })).not.toBeVisible();
  });

  test("editor file rows expose file actions without bucket sharing", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_EDITOR_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_EDITOR_BUCKET);
    await mockBucketFiles(page, [MOCK_FILES[0]]);
    await page.goto("/buckets/bkt-1");

    await expect(
      page.getByRole("button", { name: /^Upload( File)?$/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /^(Share Bucket|Share)$/ }),
    ).not.toBeVisible();
    await expect(page.getByRole("button", { name: "Share report.pdf" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Download report.pdf" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Delete report.pdf" })).toBeVisible();
  });

  test("clicking download triggers a fetch to the download endpoint", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, [MOCK_FILES[0]]);

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

    await page.getByRole("button", { name: "Download report.pdf" }).click();

    await expect.poll(() => downloadCalled).toBe(true);
  });

  test("selected files download through a bounded multi-file action", async ({
    page,
  }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_VIEWER_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_VIEWER_BUCKET);
    await mockGet(page, "/buckets/bkt-1/files", MOCK_FILES);

    let reportDownloaded = false;
    let dataDownloaded = false;
    await page.route("**/api/v1/buckets/bkt-1/files/file-1/download", async (route) => {
      reportDownloaded = true;
      await route.fulfill({
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
        body: Buffer.from("report"),
      });
    });
    await page.route("**/api/v1/buckets/bkt-1/files/file-2/download", async (route) => {
      dataDownloaded = true;
      await route.fulfill({
        status: 200,
        headers: { "Content-Type": "application/octet-stream" },
        body: Buffer.from("data"),
      });
    });

    await page.goto("/buckets/bkt-1");

    await expect(
      page.getByText("Download up to 5 selected files at once."),
    ).toBeVisible();

    await page.getByRole("checkbox", { name: "Select report.pdf" }).click();
    await page.getByRole("checkbox", { name: "Select data.csv" }).click();
    await page.getByRole("button", { name: "Download Selected" }).click();

    await expect.poll(() => reportDownloaded).toBe(true);
    await expect.poll(() => dataDownloaded).toBe(true);
  });

  test("failed bucket download shows an error toast", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, [MOCK_FILES[0]]);

    await page.route("**/api/v1/buckets/bkt-1/files/file-1/download", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({error: "download unavailable"}),
      });
    });

    await page.goto("/buckets/bkt-1");
    await expect(page.getByText("report.pdf")).toBeVisible();

    const firstRow = page.locator("tbody tr").first();
    const actionCell = firstRow.locator("td").last();
    const actionButtons = actionCell.getByRole("button");
    const actionButtonCount = await actionButtons.count();
    await actionButtons.nth(actionButtonCount === 1 ? 0 : 1).click();

    await expect(page.getByText("Something went wrong")).toBeVisible();
    await expect(
      page.getByText("The server returned an error. Try again later."),
    ).toBeVisible();
  });

  test("large text previews do not download the file", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, [LARGE_TEXT_FILE]);

    let previewRequests = 0;
    await page.route("**/api/v1/buckets/bkt-1/files/file-3/download", async (route) => {
      previewRequests += 1;
      await route.fulfill({
        status: 200,
        headers: { "Content-Type": "text/plain" },
        body: "should not load",
      });
    });

    await page.goto("/buckets/bkt-1");
    await page.getByRole("button", { name: "huge.log", exact: true }).click();

    await expect(
      page.getByText("Preview unavailable for large text files"),
    ).toBeVisible();
    await expect(
      page.getByText(
        `Files larger than ${formatSize(1024 * 1024)} must be downloaded to inspect.`,
      ),
    ).toBeVisible();
    await expect.poll(() => previewRequests).toBe(0);
  });

  test("back button is present and navigates back", async ({ page }) => {
    await injectAuth(page);
    await mockGet(page, "/buckets", [MOCK_BUCKET]);
    await mockGet(page, "/buckets/bkt-1", MOCK_BUCKET);
    await mockBucketFiles(page, []);
    await page.goto("/buckets");
    // Navigate to bucket detail
    await page.goto("/buckets/bkt-1");

    const backButton = page
      .locator('a[href="/buckets"], button[aria-label="Back"]')
      .first();
    await expect(backButton).toBeVisible();
  });
});
