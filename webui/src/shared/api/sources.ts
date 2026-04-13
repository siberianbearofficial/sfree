import {throwIfNotOk} from "./error";

const API_BASE = import.meta.env.VITE_API_BASE || "/api/v1";

export type Source = {id: string; name: string; type: string; created_at: string};

export type SourceFile = {id: string; name: string; size: number};

export type SourceInfo = {
  id: string;
  name: string;
  type: string;
  files: SourceFile[];
  storage_total: number;
  storage_used: number;
  storage_free: number;
};

import { getAuthHeader, getCredentialsOption } from "../lib/auth";

function authHeader(): Record<string, string> {
  return getAuthHeader();
}

function credentials(): RequestCredentials | undefined {
  return getCredentialsOption();
}

export async function listSources(): Promise<Source[]> {
  const res = await fetch(`${API_BASE}/sources`, {
    headers: authHeader(),
    credentials: credentials(),
  });
  await throwIfNotOk(res, "Failed to list sources");
  return res.json() as Promise<Source[]>;
}

export async function createGDriveSource(name: string, key: string): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/gdrive`, {
    method: "POST",
    headers: {"Content-Type": "application/json", ...authHeader()},
    credentials: credentials(),
    body: JSON.stringify({name, key}),
  });
  await throwIfNotOk(res, "Failed to create source");
  return res.json();
}

export async function createTelegramSource(name: string, token: string, chatId: string): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/telegram`, {
    method: "POST",
    headers: {"Content-Type": "application/json", ...authHeader()},
    credentials: credentials(),
    body: JSON.stringify({name, token, chat_id: chatId}),
  });
  await throwIfNotOk(res, "Failed to create source");
  return res.json();
}

export type CreateS3SourceParams = {
  name: string;
  endpoint: string;
  bucket: string;
  access_key_id: string;
  secret_access_key: string;
  region?: string;
  path_style?: boolean;
};

export async function createS3Source(params: CreateS3SourceParams): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/s3`, {
    method: "POST",
    headers: {"Content-Type": "application/json", ...authHeader()},
    credentials: credentials(),
    body: JSON.stringify(params),
  });
  await throwIfNotOk(res, "Failed to create source");
  return res.json();
}

export async function getSourceInfo(id: string): Promise<SourceInfo> {
  const res = await fetch(`${API_BASE}/sources/${id}/info`, {
    headers: authHeader(),
    credentials: credentials(),
  });
  await throwIfNotOk(res, "Failed to get source info");
  return res.json();
}

export async function deleteSource(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/sources/${id}`, {
    method: "DELETE",
    headers: authHeader(),
    credentials: credentials(),
  });
  await throwIfNotOk(res, "Failed to delete source");
}

export async function downloadFile(sourceId: string, file: SourceFile): Promise<void> {
  const res = await fetch(`${API_BASE}/sources/${sourceId}/files/${file.id}/download`, {
    headers: authHeader(),
    credentials: credentials(),
  });
  await throwIfNotOk(res, "Failed to download file");
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = file.name;
  a.click();
  window.URL.revokeObjectURL(url);
}
