import {apiFetch, apiJson} from "./client";

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
  return apiJson<BucketGrant>(
    `/buckets/${bucketId}/grants`,
    "Failed to grant access",
    {
      method: "POST",
      json: {username, role},
    },
  );
}

export async function listGrants(bucketId: string): Promise<BucketGrant[]> {
  return apiJson<BucketGrant[]>(
    `/buckets/${bucketId}/grants`,
    "Failed to list grants",
  );
}

export async function updateGrant(
  bucketId: string,
  grantId: string,
  role: "owner" | "editor" | "viewer",
): Promise<void> {
  await apiFetch(
    `/buckets/${bucketId}/grants/${grantId}`,
    "Failed to update grant",
    {
      method: "PATCH",
      json: {role},
    },
  );
}

export async function deleteGrant(
  bucketId: string,
  grantId: string,
): Promise<void> {
  await apiFetch(
    `/buckets/${bucketId}/grants/${grantId}`,
    "Failed to revoke access",
    {method: "DELETE"},
  );
}
