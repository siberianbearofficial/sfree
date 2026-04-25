import {apiFetch, apiJson} from "./client";

export type CurrentUser = {
  id: string;
  username: string;
  avatar_url?: string;
  github_id?: number;
};

function basicAuthorization(username: string, password: string): string {
  return `Basic ${btoa(`${username}:${password}`)}`;
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return apiJson<CurrentUser>("/auth/me", "Failed to load session");
}

export async function createSession(username: string, password: string): Promise<void> {
  await apiFetch("/auth/session", "Failed to log in", {
    method: "POST",
    headers: {
      Authorization: basicAuthorization(username, password),
    },
  });
}

export async function deleteSession(): Promise<void> {
  await apiFetch("/auth/session", "Failed to log out", {
    method: "DELETE",
  });
}
