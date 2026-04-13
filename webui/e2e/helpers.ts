import type { Page } from "@playwright/test";

export const API_GLOB = "**/api/v1";

/**
 * Inject Basic auth credentials into localStorage before the page loads.
 * Must be called before page.goto().
 */
export async function injectAuth(
  page: Page,
  username = "testuser",
  password = "testpass",
) {
  await page.addInitScript(
    ({ u, p }) => localStorage.setItem("auth", btoa(`${u}:${p}`)),
    { u: username, p: password },
  );
}

/**
 * Mock a GET endpoint. Glob pattern: API_GLOB + path, e.g. "**/api/v1/sources".
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
