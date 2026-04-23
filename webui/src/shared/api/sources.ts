import {apiDownload, apiFetch, apiJson} from "./client";

export type Source = {id: string; name: string; type: string; created_at: string};

export type SourceFile = {id: string; name: string; size: number};

export type SourceHealthStatus = "healthy" | "degraded" | "unhealthy";

export type SourceHealth = {
  id: string;
  type: string;
  status: SourceHealthStatus;
  checked_at: string;
  latency_ms: number;
  reason_code: string;
  message: string;
  quota_total_bytes: number | null;
  quota_used_bytes: number | null;
  quota_free_bytes: number | null;
};

export type SourceInfo = {
  id: string;
  name: string;
  type: string;
  files: SourceFile[];
  storage_total: number;
  storage_used: number;
  storage_free: number;
};

export async function listSources(): Promise<Source[]> {
  return apiJson<Source[]>("/sources", "Failed to list sources");
}

export async function createGDriveSource(
  name: string,
  key: string,
): Promise<Source> {
  return apiJson<Source>("/sources/gdrive", "Failed to create source", {
    method: "POST",
    json: {name, key},
  });
}

export async function createTelegramSource(
  name: string,
  token: string,
  chatId: string,
): Promise<Source> {
  return apiJson<Source>("/sources/telegram", "Failed to create source", {
    method: "POST",
    json: {name, token, chat_id: chatId},
  });
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

export async function createS3Source(
  params: CreateS3SourceParams,
): Promise<Source> {
  return apiJson<Source>("/sources/s3", "Failed to create source", {
    method: "POST",
    json: params,
  });
}

export async function getSourceInfo(id: string): Promise<SourceInfo> {
  return apiJson<SourceInfo>(
    `/sources/${id}/info`,
    "Failed to get source info",
  );
}

export async function getSourceHealth(id: string): Promise<SourceHealth> {
  return apiJson<SourceHealth>(
    `/sources/${id}/health`,
    "Failed to check source health",
  );
}

export async function deleteSource(id: string): Promise<void> {
  await apiFetch(`/sources/${id}`, "Failed to delete source", {
    method: "DELETE",
  });
}

export async function downloadFile(
  sourceId: string,
  file: SourceFile,
): Promise<void> {
  const params = new URLSearchParams({file_id: file.id});
  await apiDownload(
    `/sources/${sourceId}/download?${params.toString()}`,
    file.name,
    "Failed to download file",
  );
}
