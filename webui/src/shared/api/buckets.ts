import {apiDownload, apiFetch, apiJson} from "./client";

export type Bucket = {
  id: string;
  key: string;
  access_key: string;
  created_at: string;
  role: "owner" | "editor" | "viewer";
  shared: boolean;
};

export type FileInfo = {
  id: string;
  name: string;
  created_at: string;
  size: number;
};

export type BatchDeleteFileResult = {
  id: string;
  name: string;
};

export type BatchDeleteFileIssue = {
  id: string;
  name?: string;
  error: string;
};

export type BatchDeleteFilesResponse = {
  deleted: BatchDeleteFileResult[];
  failed: BatchDeleteFileIssue[];
  warnings: BatchDeleteFileIssue[];
};

export type BatchDownloadFileIssue = {
  file: FileInfo;
  error: unknown;
};

export type BatchDownloadFilesResult = {
  downloaded: FileInfo[];
  failed: BatchDownloadFileIssue[];
};

export const MAX_MULTI_FILE_DOWNLOAD_COUNT = 5;

type CreateBucketResponse = {
  key: string;
  access_key: string;
  access_secret: string;
  created_at: string;
};

export async function listBuckets(): Promise<Bucket[]> {
  return apiJson<Bucket[]>("/buckets", "Failed to list buckets");
}

export async function getBucket(id: string): Promise<Bucket> {
  return apiJson<Bucket>(`/buckets/${id}`, "Failed to load bucket");
}

export async function createBucket(
  key: string,
  sourceIds: string[],
): Promise<CreateBucketResponse> {
  return apiJson<CreateBucketResponse>("/buckets", "Failed to create bucket", {
    method: "POST",
    json: {key, source_ids: sourceIds},
  });
}

function bucketFilesPath(bucketId: string, query?: string): string {
  const params = new URLSearchParams();
  const trimmedQuery = query?.trim();
  if (trimmedQuery) {
    params.set("q", trimmedQuery);
  }
  const encodedQuery = params.toString();
  return encodedQuery
    ? `/buckets/${bucketId}/files?${encodedQuery}`
    : `/buckets/${bucketId}/files`;
}

export async function listFiles(
  bucketId: string,
  query?: string,
): Promise<FileInfo[]> {
  return apiJson<FileInfo[]>(
    bucketFilesPath(bucketId, query),
    "Failed to list files",
  );
}

export async function uploadFile(
  bucketId: string,
  file: File,
): Promise<FileInfo> {
  const form = new FormData();
  form.append("file", file);
  return apiJson<FileInfo>(
    `/buckets/${bucketId}/upload`,
    "Failed to upload file",
    {
      method: "POST",
      body: form,
    },
  );
}

export async function deleteBucket(id: string): Promise<void> {
  await apiFetch(`/buckets/${id}`, "Failed to delete bucket", {
    method: "DELETE",
  });
}

export async function downloadFile(
  bucketId: string,
  file: FileInfo,
): Promise<void> {
  await apiDownload(
    `/buckets/${bucketId}/files/${file.id}/download`,
    file.name,
    "Failed to download file",
  );
}

export async function downloadFiles(
  bucketId: string,
  files: FileInfo[],
): Promise<BatchDownloadFilesResult> {
  const boundedFiles = files.slice(0, MAX_MULTI_FILE_DOWNLOAD_COUNT);
  const result: BatchDownloadFilesResult = {
    downloaded: [],
    failed: [],
  };

  for (const file of boundedFiles) {
    try {
      await downloadFile(bucketId, file);
      result.downloaded.push(file);
    } catch (error) {
      result.failed.push({file, error});
    }
  }

  return result;
}

export async function fetchFileBlob(
  bucketId: string,
  fileId: string,
): Promise<Blob> {
  const res = await apiFetch(
    `/buckets/${bucketId}/files/${fileId}/download`,
    "Failed to fetch file",
  );
  return res.blob();
}

export async function deleteFile(
  bucketId: string,
  fileId: string,
): Promise<void> {
  await apiFetch(
    `/buckets/${bucketId}/files/${fileId}`,
    "Failed to delete file",
    {
      method: "DELETE",
    },
  );
}

export async function deleteFiles(
  bucketId: string,
  fileIds: string[],
): Promise<BatchDeleteFilesResponse> {
  const result = await apiJson<Partial<BatchDeleteFilesResponse>>(
    `/buckets/${bucketId}/files/batch-delete`,
    "Failed to delete files",
    {
      method: "POST",
      json: {file_ids: fileIds},
    },
  );
  return {
    deleted: result.deleted ?? [],
    failed: result.failed ?? [],
    warnings: result.warnings ?? [],
  };
}
