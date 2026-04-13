const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

export type Bucket = {
  id: string;
  key: string;
  access_key: string;
  created_at: string;
};

export type FileInfo = {
  id: string;
  name: string;
  created_at: string;
  size: number;
};

import { getAuthHeader } from "../lib/auth";

function authHeader(): Record<string, string> {
  return getAuthHeader();
}

export async function listBuckets(): Promise<Bucket[]> {
  const res = await fetch(`${API_BASE}/buckets`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to list buckets");
  return res.json();
}

export async function createBucket(key: string, sourceIds: string[]): Promise<{
  key: string;
  access_key: string;
  access_secret: string;
  created_at: string;
}> {
  const res = await fetch(`${API_BASE}/buckets`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeader(),
    },
    body: JSON.stringify({key, source_ids: sourceIds}),
  });
  if (!res.ok) throw new Error("failed to create bucket");
  return res.json();
}

export async function listFiles(bucketId: string): Promise<FileInfo[]> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/files`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to list files");
  return res.json();
}

export async function uploadFile(
  bucketId: string,
  file: File,
): Promise<FileInfo> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/upload`, {
    method: "POST",
    headers: authHeader(),
    body: form,
  });
  if (!res.ok) throw new Error("failed to upload file");
  return res.json();
}

export async function deleteBucket(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/buckets/${id}`, {
    method: "DELETE",
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to delete bucket");
}

export async function downloadFile(
  bucketId: string,
  file: FileInfo,
): Promise<void> {
  const res = await fetch(
    `${API_BASE}/buckets/${bucketId}/files/${file.id}/download`,
    {
      headers: authHeader(),
    },
  );
  if (!res.ok) throw new Error("failed to download file");
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = file.name;
  a.click();
  window.URL.revokeObjectURL(url);
}

export async function fetchFileBlob(
  bucketId: string,
  fileId: string,
): Promise<Blob> {
  const res = await fetch(
    `${API_BASE}/buckets/${bucketId}/files/${fileId}/download`,
    {
      headers: authHeader(),
    },
  );
  if (!res.ok) throw new Error("failed to fetch file");
  return res.blob();
}

export async function deleteFile(
  bucketId: string,
  fileId: string,
): Promise<void> {
  const res = await fetch(`${API_BASE}/buckets/${bucketId}/files/${fileId}`, {
    method: "DELETE",
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to delete file");
}
