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

function authHeader(): Record<string, string> {
  const auth = localStorage.getItem("auth");
  return auth ? {Authorization: `Basic ${auth}`} : {};
}

export async function listSources(): Promise<Source[]> {
  const res = await fetch(`${API_BASE}/sources`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to list sources");
  return res.json();
}

export async function createGDriveSource(name: string, key: string): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/gdrive`, {
    method: "POST",
    headers: {"Content-Type": "application/json", ...authHeader()},
    body: JSON.stringify({name, key}),
  });
  if (!res.ok) throw new Error("failed to create source");
  return res.json();
}

export async function createTelegramSource(name: string, token: string, chatId: string): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/telegram`, {
    method: "POST",
    headers: {"Content-Type": "application/json", ...authHeader()},
    body: JSON.stringify({name, token, chat_id: chatId}),
  });
  if (!res.ok) throw new Error("failed to create source");
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
    body: JSON.stringify(params),
  });
  if (!res.ok) throw new Error("failed to create source");
  return res.json();
}

export async function getSourceInfo(id: string): Promise<SourceInfo> {
  const res = await fetch(`${API_BASE}/sources/${id}/info`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to get source info");
  return res.json();
}

export async function deleteSource(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/sources/${id}`, {
    method: "DELETE",
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to delete source");
}

export async function downloadFile(sourceId: string, file: SourceFile): Promise<void> {
  const res = await fetch(`${API_BASE}/sources/${sourceId}/files/${file.id}/download`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to download file");
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = file.name;
  a.click();
  window.URL.revokeObjectURL(url);
}
