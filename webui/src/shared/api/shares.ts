import {apiFetch, apiJson} from "./client";

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
  return apiJson<ShareLinkInfo>(
    `/buckets/${bucketId}/files/${fileId}/share`,
    "Failed to create share link",
    {
      method: "POST",
      json: expiresIn ? {expires_in: expiresIn} : {},
    },
  );
}

export async function listShareLinks(
  bucketId: string,
  fileId: string,
): Promise<ShareLinkInfo[]> {
  return apiJson<ShareLinkInfo[]>(
    `/buckets/${bucketId}/files/${fileId}/shares`,
    "Failed to list share links",
  );
}

export async function deleteShareLink(id: string): Promise<void> {
  await apiFetch(`/shares/${id}`, "Failed to delete share link", {
    method: "DELETE",
  });
}
