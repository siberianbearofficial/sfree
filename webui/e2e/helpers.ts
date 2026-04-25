import type { Page } from "@playwright/test";

export const API_GLOB = "**/api/v1";

export async function injectAuth(
  page: Page,
  username = "testuser",
) {
  await page.route(`${API_GLOB}/auth/me`, async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      json: {
        id: "user-1",
        username,
      },
    });
  });
}

/**
 * Mock a GET endpoint. Glob pattern: API_GLOB + path, e.g. "/api/v1/sources".
 */
export async function mockGet(
  page: Page,
  path: string,
  body: unknown,
  status = 200,
) {
  await page.route(`${API_GLOB}${path}`, async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({ status, json: body });
  });
}

/**
 * Mock a POST endpoint.
 */
export async function mockPost(
  page: Page,
  path: string,
  body: unknown,
  status = 200,
) {
  await page.route(`${API_GLOB}${path}`, async (route) => {
    if (route.request().method() !== "POST") {
      await route.continue();
      return;
    }
    await route.fulfill({ status, json: body });
  });
}
