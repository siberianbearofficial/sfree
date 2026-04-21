import {apiDownload, apiFetch, apiJson} from "./client";

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

export async function deleteSource(id: string): Promise<void> {
  await apiFetch(`/sources/${id}`, "Failed to delete source", {
    method: "DELETE",
  });
}

export async function downloadFile(
  sourceId: string,
  file: SourceFile,
): Promise<void> {
  await apiDownload(
    `/sources/${sourceId}/files/${file.id}/download`,
    file.name,
    "Failed to download file",
  );
}
