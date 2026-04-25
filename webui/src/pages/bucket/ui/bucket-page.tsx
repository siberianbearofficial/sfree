import {
  Button,
  Checkbox,
  Chip,
  Input,
  Snippet,
  Spinner,
  useDisclosure,
} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useDeferredValue, useEffect, useMemo, useRef, useState} from "react";
import {Link, useNavigate, useParams} from "react-router-dom";
import {
  deleteFile,
  deleteFiles,
  downloadFile,
  downloadFiles,
  getBucket,
  listFiles,
  MAX_MULTI_FILE_DOWNLOAD_COUNT,
  uploadFile,
} from "../../../shared/api/buckets";
import type {BatchDeleteFilesResponse, Bucket, FileInfo} from "../../../shared/api/buckets";
import {DownloadIcon, ShareIcon} from "../../../shared/icons";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {FilePreviewModal} from "../../../features/bucket/ui/file-preview-modal";
import {ShareBucketDialog} from "../../../features/bucket/ui/share-bucket-dialog";
import {ShareFileDialog} from "../../../features/bucket/ui/share-file-dialog";
import {formatSize} from "../../../shared/lib/format";
import {ApiError, showErrorToast} from "../../../shared/api/error";

const ROLE_COLOR: Record<string, "default" | "primary" | "secondary" | "success" | "warning" | "danger"> = {
  owner: "primary",
  editor: "secondary",
  viewer: "default",
};

type SortField = "name" | "size" | "created_at";

export function BucketPage() {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const [bucket, setBucket] = useState<Bucket | null>(null);
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshingFiles, setIsRefreshingFiles] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const hasLoadedRef = useRef(false);
  const requestIDRef = useRef(0);
  const confirm = useDisclosure();
  const [pendingDeleteFiles, setPendingDeleteFiles] = useState<FileInfo[]>([]);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isDownloadingSelected, setIsDownloadingSelected] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileInfo | null>(null);
  const [selectedFileIds, setSelectedFileIds] = useState<string[]>([]);

  const shareBucket = useDisclosure();
  const [shareFile, setShareFile] = useState<FileInfo | null>(null);
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const activeSearchQuery = deferredSearchQuery.trim();

  const [sortBy, setSortBy] = useState<SortField | null>(null);
  const [sortAsc, setSortAsc] = useState(true);

  function toggleSort(field: SortField) {
    if (sortBy === field) {
      setSortAsc((prev) => !prev);
    } else {
      setSortBy(field);
      setSortAsc(true);
    }
  }

  const sortedFiles = useMemo(() => {
    if (!sortBy) return files;
    const field = sortBy;
    return [...files].sort((a, b) => {
      let cmp: number;
      switch (field) {
        case "name":
          cmp = a.name.localeCompare(b.name);
          break;
        case "size":
          cmp = a.size - b.size;
          break;
        case "created_at":
          cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
          break;
      }
      return sortAsc ? cmp : -cmp;
    });
  }, [files, sortBy, sortAsc]);

  const load = useCallback(async (mode: "initial" | "refresh" = "initial") => {
    if (!id) return;
    const requestID = requestIDRef.current + 1;
    requestIDRef.current = requestID;
    if (mode === "initial") {
      setIsLoading(true);
    } else {
      setIsRefreshingFiles(true);
    }
    setError(null);
    try {
      const [loadedBucket, fs] = await Promise.all([
        getBucket(id),
        listFiles(id, activeSearchQuery),
      ]);
      if (requestID !== requestIDRef.current) {
        return;
      }
      setBucket(loadedBucket);
      setFiles(fs);
      hasLoadedRef.current = true;
    } catch (err) {
      if (requestID !== requestIDRef.current) {
        return;
      }
      if (err instanceof ApiError && err.status === 404) {
        setBucket(null);
        setFiles([]);
        return;
      }
      setError(err instanceof Error ? err.message : "Failed to load bucket");
    } finally {
      if (requestID === requestIDRef.current) {
        if (mode === "initial") {
          setIsLoading(false);
        } else {
          setIsRefreshingFiles(false);
        }
      }
    }
  }, [activeSearchQuery, id]);

  useEffect(() => {
    hasLoadedRef.current = false;
    setSearchQuery("");
    setSelectedFileIds([]);
    setPendingDeleteFiles([]);
  }, [id]);

  useEffect(() => {
    if (!id) return;
    void load(hasLoadedRef.current ? "refresh" : "initial");
  }, [id, load]);

  useEffect(() => {
    const visibleFileIDs = new Set(files.map((file) => file.id));
    setSelectedFileIds((prev) => prev.filter((fileID) => visibleFileIDs.has(fileID)));
  }, [files]);

  async function handleUpload(file: File) {
    if (!id || !bucket || !canWriteFiles(bucket)) return;
    try {
      await uploadFile(id, file);
      addToast({title: "File uploaded", description: `${file.name} added to bucket`, color: "success", timeout: 4000});
      await load("refresh");
    } catch (err) {
      showErrorToast(err);
    }
  }

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) handleUpload(f);
  }

  function onDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    if (!bucket || !canWriteFiles(bucket)) return;
    const f = e.dataTransfer.files?.[0];
    if (f) handleUpload(f);
  }

  async function handleDelete() {
    if (!id || pendingDeleteFiles.length === 0) return;
    setIsDeleting(true);
    try {
      if (pendingDeleteFiles.length === 1) {
        await deleteFile(id, pendingDeleteFiles[0].id);
        addToast({title: "File deleted", color: "success", timeout: 4000});
        setSelectedFileIds((prev) => prev.filter((fileID) => fileID !== pendingDeleteFiles[0].id));
      } else {
        const result = await deleteFiles(id, pendingDeleteFiles.map((file) => file.id));
        setSelectedFileIds((prev) => {
          const deleted = new Set(result.deleted.map((file) => file.id));
          return prev.filter((fileID) => !deleted.has(fileID));
        });
        showBatchDeleteToast(result, pendingDeleteFiles.length);
      }
      await load("refresh");
    } catch (err) {
      showErrorToast(err);
    } finally {
      setIsDeleting(false);
      setPendingDeleteFiles([]);
      confirm.onClose();
    }
  }

  function openShareModal(file: FileInfo) {
    setShareFile(file);
  }

  async function handleDownload(file: FileInfo) {
    if (!id) return;
    try {
      await downloadFile(id, file);
    } catch (err) {
      showErrorToast(err);
    }
  }

  const selectedFileIDSet = useMemo(() => new Set(selectedFileIds), [selectedFileIds]);
  const selectedFiles = useMemo(
    () => sortedFiles.filter((file) => selectedFileIDSet.has(file.id)),
    [selectedFileIDSet, sortedFiles],
  );
  const selectedCount = selectedFiles.length;
  const allVisibleSelected = sortedFiles.length > 0 && selectedCount === sortedFiles.length;
  const someVisibleSelected = selectedCount > 0 && !allVisibleSelected;
  const canSelectFiles = sortedFiles.length > 0;
  const canDeleteSelected = bucket !== null && canWriteFiles(bucket);
  const selectedDownloadLimitExceeded = selectedCount > MAX_MULTI_FILE_DOWNLOAD_COUNT;

  function setFileSelected(fileID: string, selected: boolean) {
    setSelectedFileIds((prev) => {
      if (selected) {
        return prev.includes(fileID) ? prev : [...prev, fileID];
      }
      return prev.filter((currentID) => currentID !== fileID);
    });
  }

  function setAllVisibleSelected(selected: boolean) {
    if (selected) {
      setSelectedFileIds(sortedFiles.map((file) => file.id));
      return;
    }
    setSelectedFileIds([]);
  }

  function openDeleteDialog(filesToDelete: FileInfo[]) {
    if (filesToDelete.length === 0) return;
    setPendingDeleteFiles(filesToDelete);
    confirm.onOpen();
  }

  async function handleDownloadSelected() {
    if (!id || selectedCount === 0) return;
    if (selectedDownloadLimitExceeded) {
      addToast({
        title: "Select fewer files",
        description: `Download up to ${MAX_MULTI_FILE_DOWNLOAD_COUNT} files at once.`,
        color: "warning",
        timeout: 5000,
      });
      return;
    }

    setIsDownloadingSelected(true);
    try {
      const result = await downloadFiles(id, selectedFiles);
      if (result.failed.length === 0) {
        addToast({
          title: result.downloaded.length === 1 ? "Download started" : "Downloads started",
          description: "Your browser may ask to allow multiple downloads.",
          color: "success",
          timeout: 4000,
        });
        return;
      }

      addToast({
        title: result.downloaded.length > 0 ? "Some downloads failed" : "Downloads failed",
        description:
          result.downloaded.length > 0
            ? `${result.downloaded.length} started, ${result.failed.length} failed.`
            : `Unable to start ${result.failed.length} download${result.failed.length === 1 ? "" : "s"}.`,
        color: "warning",
        timeout: 5000,
      });
    } finally {
      setIsDownloadingSelected(false);
    }
  }

  if (isLoading) {
    return (
      <div className="p-6 sm:p-8 flex items-center justify-center min-h-[200px]">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 sm:p-8">
        <Link to="/buckets" className="text-sm text-default-500 hover:text-primary transition-colors">
          &larr; Buckets
        </Link>
        <EmptyState
          title="Failed to load bucket"
          description={error}
          ctaLabel="Retry"
          onCtaPress={load}
          variant="danger"
        />
      </div>
    );
  }

  if (!bucket) {
    return (
      <div className="p-6 sm:p-8">
        <Link to="/buckets" className="text-sm text-default-500 hover:text-primary transition-colors">
          &larr; Buckets
        </Link>
        <EmptyState
          title="Bucket not found"
          description="This bucket may have been deleted."
          ctaLabel="Back to Buckets"
          onCtaPress={() => navigate("/buckets")}
        />
      </div>
    );
  }

  const canManage = canManageBucket(bucket);
  const canWrite = canWriteFiles(bucket);
  const emptyTitle = activeSearchQuery
    ? "No matching files"
    : canWrite
      ? "Last step: upload your first file"
      : "No files yet";
  const emptyDescription = activeSearchQuery
    ? `No files in this bucket match "${activeSearchQuery}".`
    : canWrite
      ? "Drop a file here or click Upload to finish setting up your SFree account."
      : "Files shared in this bucket will appear here.";

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-6">
      <Link to="/buckets" className="text-sm text-default-500 hover:text-primary transition-colors">
        &larr; Buckets
      </Link>
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-3xl font-bold break-all">{bucket.key}</h1>
          <Chip size="sm" variant="flat" color={ROLE_COLOR[bucket.role] ?? "default"}>
            {bucket.role}
          </Chip>
        </div>
        <CredentialsPanel bucket={bucket} />
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        {canManage && (
          <Button variant="flat" onPress={shareBucket.onOpen}>
            Share Bucket
          </Button>
        )}
        {canWrite && (
          <>
            <input
              type="file"
              ref={fileInput}
              className="hidden"
              onChange={onFileChange}
            />
            <Button color="primary" onPress={() => fileInput.current?.click()}>
              Upload File
            </Button>
          </>
        )}
      </div>
      <div
        className={
          canWrite ? "border-2 border-dashed rounded p-4" : "border rounded p-4"
        }
        onDragOver={(e) => e.preventDefault()}
        onDrop={onDrop}
      >
        <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
          <Input
            aria-label="Search files"
            className="w-full md:max-w-md"
            isClearable
            label="Search files"
            placeholder="Filter by filename"
            type="search"
            value={searchQuery}
            onClear={() => setSearchQuery("")}
            onValueChange={setSearchQuery}
            endContent={isRefreshingFiles ? <Spinner size="sm" /> : null}
          />
          {activeSearchQuery ? (
            <p className="text-sm text-default-500">
              {files.length === 1 ? "1 matching file" : `${files.length} matching files`}
            </p>
          ) : null}
        </div>
        {canSelectFiles ? (
          <div className="mb-4 flex flex-col gap-3 rounded-lg border border-divider bg-content2/60 p-3 md:flex-row md:items-center md:justify-between">
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium">
                {selectedCount === 0
                  ? canDeleteSelected
                    ? "Select files to download or delete in one step"
                    : "Select files to download together"
                  : selectedCount === 1
                    ? "1 file selected"
                    : `${selectedCount} files selected`}
              </p>
              <p className="text-xs text-default-500">
                {selectedDownloadLimitExceeded
                  ? `Download up to ${MAX_MULTI_FILE_DOWNLOAD_COUNT} files at once. Clear some selections to continue.`
                  : `Download up to ${MAX_MULTI_FILE_DOWNLOAD_COUNT} selected files at once. Your browser may ask to allow multiple downloads.`}
              </p>
            </div>
            <div className="flex gap-2">
              <Button variant="flat" onPress={() => setSelectedFileIds([])} isDisabled={selectedCount === 0}>
                Clear
              </Button>
              <Button
                color="primary"
                onPress={handleDownloadSelected}
                isDisabled={selectedCount === 0 || selectedDownloadLimitExceeded}
                isLoading={isDownloadingSelected}
              >
                Download Selected
              </Button>
              {canDeleteSelected ? (
                <Button color="danger" onPress={() => openDeleteDialog(selectedFiles)} isDisabled={selectedCount === 0}>
                  Delete Selected
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}
        {sortedFiles.length === 0 ? (
          <EmptyState
            title={emptyTitle}
            description={emptyDescription}
            ctaLabel={!activeSearchQuery && canWrite ? "Upload File" : undefined}
            onCtaPress={!activeSearchQuery && canWrite ? () => fileInput.current?.click() : undefined}
          />
        ) : (
          <div className="overflow-x-auto -mx-4 px-4">
            <table className="w-full text-left">
              <thead>
                <tr>
                  {canSelectFiles ? (
                    <th className="w-12 pb-2">
                      <Checkbox
                        aria-label={allVisibleSelected ? "Unselect all visible files" : "Select all visible files"}
                        isSelected={allVisibleSelected}
                        isIndeterminate={someVisibleSelected}
                        onValueChange={setAllVisibleSelected}
                      />
                    </th>
                  ) : null}
                  <th className="pb-2 cursor-pointer select-none" onClick={() => toggleSort("name")}>
                    Name<SortArrow field="name" sortBy={sortBy} sortAsc={sortAsc} />
                  </th>
                  <th className="pb-2 cursor-pointer select-none whitespace-nowrap" onClick={() => toggleSort("size")}>
                    Size<SortArrow field="size" sortBy={sortBy} sortAsc={sortAsc} />
                  </th>
                  <th
                    className="hidden pb-2 cursor-pointer select-none whitespace-nowrap sm:table-cell"
                    onClick={() => toggleSort("created_at")}
                  >
                    Created<SortArrow field="created_at" sortBy={sortBy} sortAsc={sortAsc} />
                  </th>
                  <th className="pb-2"></th>
                </tr>
              </thead>
              <tbody>
                {sortedFiles.map((f) => (
                  <tr key={f.id} className="border-t">
                    {canSelectFiles ? (
                      <td className="py-2 pr-3 align-middle">
                        <Checkbox
                          aria-label={`Select ${f.name}`}
                          isSelected={selectedFileIDSet.has(f.id)}
                          onValueChange={(selected) => setFileSelected(f.id, selected)}
                        />
                      </td>
                    ) : null}
                    <td className="py-2">
                      <button
                        type="button"
                        className="cursor-pointer break-all text-left transition-colors hover:text-primary"
                        onClick={() => setPreviewFile(f)}
                      >
                        {f.name}
                      </button>
                    </td>
                    <td className="py-2 whitespace-nowrap">{formatSize(f.size)}</td>
                    <td className="hidden py-2 whitespace-nowrap sm:table-cell">
                      {new Date(f.created_at).toLocaleString()}
                    </td>
                    <td className="py-2">
                      <div className="flex gap-1 sm:gap-2">
                        {canWrite && (
                          <Button
                            isIconOnly
                            size="sm"
                            aria-label={`Share ${f.name}`}
                            variant="light"
                            onPress={() => openShareModal(f)}
                          >
                            <ShareIcon className="h-4 w-4 sm:h-5 sm:w-5" />
                          </Button>
                        )}
                        <Button
                          isIconOnly
                          size="sm"
                          aria-label={`Download ${f.name}`}
                          variant="light"
                          onPress={() => handleDownload(f)}
                        >
                          <DownloadIcon className="h-4 w-4 sm:h-5 sm:w-5" />
                        </Button>
                        {canWrite && (
                          <Button
                            isIconOnly
                            size="sm"
                            aria-label={`Delete ${f.name}`}
                            variant="light"
                            color="danger"
                            onPress={() => openDeleteDialog([f])}
                          >
                            <DeleteIcon className="h-4 w-4 sm:h-5 sm:w-5" />
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <ConfirmDialog
        isOpen={confirm.isOpen}
        onOpenChange={(open) => {
          if (!open) setPendingDeleteFiles([]);
          confirm.onOpenChange();
        }}
        title={pendingDeleteFiles.length > 1 ? "Delete selected files?" : "Delete file?"}
        message={
          pendingDeleteFiles.length > 1
            ? `Are you sure you want to delete ${pendingDeleteFiles.length} files? This cannot be undone.`
            : "Are you sure you want to delete this file?"
        }
        onConfirm={handleDelete}
        confirmLabel={pendingDeleteFiles.length > 1 ? "Delete Selected" : "Delete"}
        isConfirmLoading={isDeleting}
      />
      <FilePreviewModal
        isOpen={previewFile !== null}
        onClose={() => setPreviewFile(null)}
        file={previewFile}
        bucketId={id!}
      />
      <ShareFileDialog
        bucketId={id!}
        file={shareFile}
        onClose={() => setShareFile(null)}
      />
      <ShareBucketDialog
        isOpen={shareBucket.isOpen}
        onOpenChange={shareBucket.onOpenChange}
        bucketId={id!}
      />
    </div>
  );
}

function CredentialsPanel({bucket}: {bucket: Bucket}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="border border-divider rounded-lg">
      <button
        type="button"
        className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-default-100 transition-colors rounded-lg cursor-pointer"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span>S3 Credentials</span>
        <svg
          className={`w-4 h-4 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div className="px-4 pb-4 flex flex-col gap-3 overflow-hidden">
          <div className="min-w-0">
            <p className="text-xs text-default-500 mb-1">Bucket ID</p>
            <Snippet size="sm" variant="flat" symbol="" classNames={{base: "max-w-full", pre: "whitespace-pre-wrap break-all"}}>{bucket.id}</Snippet>
          </div>
          <div className="min-w-0">
            <p className="text-xs text-default-500 mb-1">Access Key</p>
            <Snippet size="sm" variant="flat" symbol="" classNames={{base: "max-w-full", pre: "whitespace-pre-wrap break-all"}}>{bucket.access_key}</Snippet>
          </div>
          <p className="text-xs text-default-500">
            Created {new Date(bucket.created_at).toLocaleString()}
          </p>
        </div>
      )}
    </div>
  );
}

function SortArrow({field, sortBy, sortAsc}: {field: SortField; sortBy: SortField | null; sortAsc: boolean}) {
  if (sortBy !== field) return null;
  return (
    <span aria-hidden="true" className="ml-1 text-default-400">
      {sortAsc ? "↑" : "↓"}
    </span>
  );
}

function canManageBucket(bucket: Bucket) {
  return bucket.role === "owner";
}

function canWriteFiles(bucket: Bucket) {
  return bucket.role === "owner" || bucket.role === "editor";
}

function showBatchDeleteToast(result: BatchDeleteFilesResponse, requestedCount: number) {
  const deletedCount = result.deleted.length;
  const failedCount = result.failed.length;
  const warningCount = result.warnings.length;

  if (deletedCount === requestedCount && failedCount === 0 && warningCount === 0) {
    addToast({
      title: deletedCount === 1 ? "File deleted" : `${deletedCount} files deleted`,
      color: "success",
      timeout: 4000,
    });
    return;
  }

  if (deletedCount > 0 && failedCount === 0) {
    addToast({
      title: deletedCount === 1 ? "File deleted" : `${deletedCount} files deleted`,
      description: warningCount === 1
        ? "Server cleanup reported a warning for 1 file."
        : `Server cleanup reported warnings for ${warningCount} files.`,
      color: "warning",
      timeout: 6000,
    });
    return;
  }

  if (deletedCount > 0) {
    addToast({
      title: `Deleted ${deletedCount} of ${requestedCount} files`,
      description: failedCount === 1
        ? "1 file could not be deleted."
        : `${failedCount} files could not be deleted.`,
      color: "warning",
      timeout: 6000,
    });
    return;
  }

  addToast({
    title: "No files deleted",
    description: failedCount === 1 ? result.failed[0].error : `${failedCount} files could not be deleted.`,
    color: "danger",
    timeout: 6000,
  });
}
