import {getAuthHeader, getCredentialsOption} from "../lib/auth";
import {throwIfNotOk} from "./error";

const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

type ApiFetchOptions = Omit<RequestInit, "body" | "headers"> & {
  auth?: boolean;
  body?: BodyInit | null;
  json?: unknown;
  headers?: HeadersInit;
};

export function apiUrl(path: string): string {
  return `${API_BASE}${path}`;
}

export async function apiFetch(
  path: string,
  fallback: string,
  options: ApiFetchOptions = {},
): Promise<Response> {
  const {auth = true, json, headers, ...init} = options;
  const requestHeaders = new Headers(headers);

  if (json !== undefined && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }

  if (auth) {
    for (const [key, value] of Object.entries(getAuthHeader())) {
      requestHeaders.set(key, value);
    }
  }

  const res = await fetch(apiUrl(path), {
    ...init,
    headers: requestHeaders,
    credentials: auth ? getCredentialsOption() : init.credentials,
    body: json !== undefined ? JSON.stringify(json) : init.body,
  });

  await throwIfNotOk(res, fallback);
  return res;
}

export async function apiJson<T>(
  path: string,
  fallback: string,
  options?: ApiFetchOptions,
): Promise<T> {
  const res = await apiFetch(path, fallback, options);
  return res.json() as Promise<T>;
}

export async function apiDownload(
  path: string,
  filename: string,
  fallback: string,
): Promise<void> {
  const res = await apiFetch(path, fallback);
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  window.URL.revokeObjectURL(url);
}
