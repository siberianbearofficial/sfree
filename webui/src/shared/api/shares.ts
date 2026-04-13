const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

import { getAuthHeader } from "../lib/auth";

function authHeader(): Record<string, string> {
  return getAuthHeader();
}

export type ShareLinkInfo = {
  id: string;
  file_id: string;
  file_name: string;
  token: string;
  url: string;
  expires_at: string | null;
  created_at: string;
};

export async function createShareLink(
  bucketId: string,
  fileId: string,
  expiresIn?: number,
): Promise<ShareLinkInfo> {
  const res = await fetch(
    `${API_BASE}/buckets/${bucketId}/files/${fileId}/share`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...authHeader(),
      },
      body: JSON.stringify(expiresIn ? { expires_in: expiresIn } : {}),
    },
  );
  if (!res.ok) throw new Error("failed to create share link");
  return res.json();
}

export async function listShareLinks(
  bucketId: string,
  fileId: string,
): Promise<ShareLinkInfo[]> {
  const res = await fetch(
    `${API_BASE}/buckets/${bucketId}/files/${fileId}/shares`,
    {
      headers: authHeader(),
    },
  );
  if (!res.ok) throw new Error("failed to list share links");
  return res.json();
}

export async function deleteShareLink(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/shares/${id}`, {
    method: "DELETE",
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to delete share link");
}
