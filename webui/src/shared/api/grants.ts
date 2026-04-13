const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

import { getAuthHeader } from "../lib/auth";
import { throwIfNotOk } from "./error";

function authHeader(): Record<string, string> {
  return getAuthHeader();
}

export type BucketGrant = {
  id: string;
  bucket_id: string;
  user_id: string;
  username: string;
  role: "owner" | "editor" | "viewer";
  granted_by: string;
  created_at: string;
};

export async function createGrant(
  bucketId: string,
  username: string,
  role: "owner" | "editor" | "viewer",
): Promise<BucketGrant> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/grants`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeader(),
    },
    body: JSON.stringify({ username, role }),
  });
  await throwIfNotOk(res, "Failed to grant access");
  return res.json();
}

export async function listGrants(bucketId: string): Promise<BucketGrant[]> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/grants`, {
    headers: authHeader(),
  });
  await throwIfNotOk(res, "Failed to list grants");
  return res.json();
}

export async function updateGrant(
  bucketId: string,
  grantId: string,
  role: "owner" | "editor" | "viewer",
): Promise<void> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/grants/${grantId}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
      ...authHeader(),
    },
    body: JSON.stringify({ role }),
  });
  await throwIfNotOk(res, "Failed to update grant");
}

export async function deleteGrant(
  bucketId: string,
  grantId: string,
): Promise<void> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/grants/${grantId}`, {
    method: "DELETE",
    headers: authHeader(),
  });
  await throwIfNotOk(res, "Failed to revoke access");
}
