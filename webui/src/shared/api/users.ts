import {throwIfNotOk} from "./error";

const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

export async function createUser(username: string): Promise<{id: string; created_at: string; password: string}> {
  const res = await fetch(`${API_BASE}/users`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify({username}),
  });
  await throwIfNotOk(res, "Failed to create user");
  return res.json();
}
