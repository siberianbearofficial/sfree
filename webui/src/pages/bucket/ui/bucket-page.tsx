import {
  Button,
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
  downloadFile,
  getBucket,
  listFiles,
  uploadFile,
} from "../../../shared/api/buckets";
import type {Bucket, FileInfo} from "../../../shared/api/buckets";
import {DownloadIcon, ShareIcon} from "../../../shared/icons";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {FilePreviewModal} from "../../../features/bucket/ui/file-preview-modal";
import {ShareBucketDialog} from "../../../features/bucket/ui/share-bucket-dialog";
import {ShareFileDialog} from "../../../features/bucket/ui/share-file-dialog";
import {formatSize} from "../../../shared/lib/format";
import {ApiError, showErrorToast} from "../../../shared/api/error";

/* ------------------------------------------------------------------ */
/*  Role helpers                                                       */
/* ------------------------------------------------------------------ */

function canManageBucket(bucket: Bucket) {
  return bucket.role === "owner";
}

function canWriteFiles(bucket: Bucket) {
  return bucket.role === "owner" || bucket.role === "editor";
}

const ROLE_COLOR: Record<string, "primary" | "secondary" | "default"> = {
  owner: "primary",
  editor: "secondary",
  viewer: "default",
};

/* ------------------------------------------------------------------ */
/*  Credentials panel                                                  */
/* ------------------------------------------------------------------ */

function CredentialsPanel({bucket}: {bucket: Bucket}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="border border-divider rounded-lg">
      <button
        type="button"
        className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium text-foreground hover:bg-default-100 transition-colors rounded-lg cursor-pointer"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        <span>S3 Credentials</span>
        <svg
          className={`w-4 h-4 transition-transform ${open ? "rotate-180" : ""}`}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>

      {open && (
        <div className="px-4 pb-4 flex flex-col gap-3">
          <div>
            <label className="text-xs text-default-500 mb-1 block">
              Bucket ID
            </label>
            <Snippet size="sm" symbol="" className="w-full">
              {bucket.id}
            </Snippet>
          </div>
          <div>
            <label className="text-xs text-default-500 mb-1 block">
              Access Key
            </label>
            <Snippet size="sm" symbol="" className="w-full">
              {bucket.access_key}
            </Snippet>
          </div>
          <div>
            <label className="text-xs text-default-500 mb-1 block">
              Created
            </label>
            <p className="text-sm text-default-700">
              {new Date(bucket.created_at).toLocaleString()}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Drop zone                                                          */
/* ------------------------------------------------------------------ */

function DropZone({
  active,
  onDrop,
  children,
}: {
  active: boolean;
  onDrop: (files: FileList) => void;
  children: React.ReactNode;
}) {
  const [dragging, setDragging] = useState(false);
  const dragCounter = useRef(0);

  function handleDragEnter(e: React.DragEvent) {
    e.preventDefault();
    dragCounter.current++;
    setDragging(true);
  }
  function handleDragLeave(e: React.DragEvent) {
    e.preventDefault();
    dragCounter.current--;
    if (dragCounter.current <= 0) {
      dragCounter.current = 0;
      setDragging(false);
    }
  }
  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    dragCounter.current = 0;
    setDragging(false);
    if (active && e.dataTransfer.files.length > 0) {
      onDrop(e.dataTransfer.files);
    }
  }

  function suppressDrag(e: React.DragEvent) {
    e.preventDefault();
  }

  return (
    <div
      className={`relative rounded-lg transition-colors ${
        dragging && active ? "ring-2 ring-primary bg-primary/5" : ""
      }`}
      onDragEnter={active ? handleDragEnter : suppressDrag}
      onDragOver={suppressDrag}
      onDragLeave={active ? handleDragLeave : undefined}
      onDrop={active ? handleDrop : suppressDrag}
    >
      {dragging && active && (
        <div className="absolute inset-0 z-10 flex items-center justify-center bg-primary/5 rounded-lg pointer-events-none">
          <p className="text-primary font-medium">Drop files here</p>
        </div>
      )}
      {children}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Upload queue indicator                                             */
/* ------------------------------------------------------------------ */

function UploadQueue({count}: {count: number}) {
  if (count === 0) return null;
  return (
    <div className="flex items-center gap-2 px-3 py-2 bg-primary/10 rounded-lg text-sm text-primary">
      <Spinner size="sm" color="primary" />
      <span>
        Uploading {count} file{count > 1 ? "s" : ""}…
      </span>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Main page                                                          */
/* ------------------------------------------------------------------ */

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
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileInfo | null>(null);

  const shareBucket = useDisclosure();
  const [shareFile, setShareFile] = useState<FileInfo | null>(null);
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const activeSearchQuery = deferredSearchQuery.trim();

  const [uploadingCount, setUploadingCount] = useState(0);
  type SortColumn = "name" | "size" | "created_at";
  type SortDir = "ascending" | "descending";
  const [sortColumn, setSortColumn] = useState<SortColumn>("name");
  const [sortDirection, setSortDirection] = useState<SortDir>("ascending");

  /* ---- data loading ---- */

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
      if (requestID !== requestIDRef.current) return;
      setBucket(loadedBucket);
      setFiles(fs);
      hasLoadedRef.current = true;
    } catch (err) {
      if (requestID !== requestIDRef.current) return;
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
  }, [id]);

  useEffect(() => {
    if (!id) return;
    void load(hasLoadedRef.current ? "refresh" : "initial");
  }, [id, load]);

  /* ---- sorted files ---- */

  const sortedFiles = useMemo(() => {
    const sorted = [...files];
    sorted.sort((a, b) => {
      let cmp = 0;
      if (sortColumn === "name") {
        cmp = a.name.localeCompare(b.name);
      } else if (sortColumn === "size") {
        cmp = a.size - b.size;
      } else if (sortColumn === "created_at") {
        cmp =
          new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
      }
      return sortDirection === "descending" ? -cmp : cmp;
    });
    return sorted;
  }, [files, sortColumn, sortDirection]);

  function toggleSort(col: SortColumn) {
    if (sortColumn === col) {
      setSortDirection((d) => (d === "ascending" ? "descending" : "ascending"));
    } else {
      setSortColumn(col);
      setSortDirection("ascending");
    }
  }

  /* ---- uploads ---- */

  async function handleMultiUpload(fileList: FileList) {
    if (!id || !bucket || !canWriteFiles(bucket)) return;
    const filesToUpload = Array.from(fileList);
    setUploadingCount((c) => c + filesToUpload.length);
    const results = await Promise.allSettled(
      filesToUpload.map(async (file) => {
        try {
          await uploadFile(id, file);
          addToast({
            title: "File uploaded",
            description: `${file.name} added to bucket`,
            color: "success",
            timeout: 4000,
          });
        } finally {
          setUploadingCount((c) => c - 1);
        }
      }),
    );
    for (const r of results) {
      if (r.status === "rejected") showErrorToast(r.reason);
    }
    await load("refresh");
  }

  function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const fileList = e.target.files;
    if (fileList && fileList.length > 0) {
      handleMultiUpload(fileList);
      e.target.value = "";
    }
  }

  /* ---- delete ---- */

  async function handleDelete() {
    if (!id || !deleteId) return;
    setIsDeleting(true);
    try {
      await deleteFile(id, deleteId);
      addToast({title: "File deleted", color: "success", timeout: 4000});
      await load("refresh");
    } catch (err) {
      showErrorToast(err);
    } finally {
      setIsDeleting(false);
      confirm.onClose();
    }
  }

  /* ---- download ---- */

  async function handleDownload(file: FileInfo) {
    if (!id) return;
    try {
      await downloadFile(id, file);
    } catch (err) {
      showErrorToast(err);
    }
  }

  /* ---- loading / error / not-found states ---- */

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

      {/* Header row */}
      <div className="flex flex-col sm:flex-row sm:items-center gap-4">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <h1 className="text-2xl sm:text-3xl font-bold truncate">
            {bucket.key}
          </h1>
          <Chip size="sm" variant="flat" color={ROLE_COLOR[bucket.role]}>
            {bucket.role}
          </Chip>
          {bucket.shared && (
            <Chip size="sm" variant="flat" color="warning">
              shared
            </Chip>
          )}
        </div>
        <div className="flex gap-2 shrink-0">
          {canManage && (
            <Button
              variant="flat"
              size="sm"
              startContent={<ShareIcon className="w-4 h-4" />}
              onPress={shareBucket.onOpen}
            >
              <span className="hidden sm:inline">Share</span>
            </Button>
          )}
          {canWrite && (
            <>
              <input
                type="file"
                ref={fileInput}
                className="hidden"
                onChange={onFileChange}
                multiple
              />
              <Button
                color="primary"
                size="sm"
                onPress={() => fileInput.current?.click()}
              >
                Upload
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Credentials panel */}
      <CredentialsPanel bucket={bucket} />

      {/* Upload queue */}
      <UploadQueue count={uploadingCount} />

      {/* File workspace */}
      <DropZone active={canWrite} onDrop={handleMultiUpload}>
        {/* Search bar */}
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

        {files.length === 0 ? (
          <div className="border-2 border-dashed border-default-300 rounded-lg p-8">
            <EmptyState
              title={emptyTitle}
              description={emptyDescription}
              ctaLabel={canWrite ? "Upload File" : undefined}
              onCtaPress={canWrite ? () => fileInput.current?.click() : undefined}
            />
          </div>
        ) : (
          <div className="border border-divider rounded-lg overflow-hidden">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b border-divider bg-default-50">
                  <th className="px-4 py-3 text-sm font-medium">
                    <button type="button" className="cursor-pointer select-none" onClick={() => toggleSort("name")}>
                      Name{sortColumn === "name" && <span aria-hidden="true"> {sortDirection === "ascending" ? "↑" : "↓"}</span>}
                    </button>
                  </th>
                  <th className="px-4 py-3 text-sm font-medium hidden sm:table-cell">
                    <button type="button" className="cursor-pointer select-none" onClick={() => toggleSort("size")}>
                      Size{sortColumn === "size" && <span aria-hidden="true"> {sortDirection === "ascending" ? "↑" : "↓"}</span>}
                    </button>
                  </th>
                  <th className="px-4 py-3 text-sm font-medium hidden md:table-cell">
                    <button type="button" className="cursor-pointer select-none" onClick={() => toggleSort("created_at")}>
                      Created{sortColumn === "created_at" && <span aria-hidden="true"> {sortDirection === "ascending" ? "↑" : "↓"}</span>}
                    </button>
                  </th>
                  <th className="px-4 py-3 text-sm font-medium text-right w-[1%]">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {sortedFiles.map((file) => (
                  <tr key={file.id} className="border-t border-divider hover:bg-default-50 transition-colors">
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        className="text-left hover:text-primary transition-colors cursor-pointer truncate max-w-[200px] sm:max-w-[300px] md:max-w-none"
                        onClick={() => setPreviewFile(file)}
                        title={file.name}
                      >
                        {file.name}
                      </button>
                      <span className="block sm:hidden text-xs text-default-400 mt-0.5">
                        {formatSize(file.size)}
                      </span>
                    </td>
                    <td className="px-4 py-3 hidden sm:table-cell text-default-500 text-sm">
                      {formatSize(file.size)}
                    </td>
                    <td className="px-4 py-3 hidden md:table-cell text-default-500 text-sm whitespace-nowrap">
                      {new Date(file.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex gap-1 justify-end">
                        {canWrite && (
                          <Button
                            isIconOnly
                            size="sm"
                            aria-label={`Share ${file.name}`}
                            variant="light"
                            onPress={() => setShareFile(file)}
                          >
                            <ShareIcon className="w-4 h-4" />
                          </Button>
                        )}
                        <Button
                          isIconOnly
                          size="sm"
                          aria-label={`Download ${file.name}`}
                          variant="light"
                          onPress={() => handleDownload(file)}
                        >
                          <DownloadIcon className="w-4 h-4" />
                        </Button>
                        {canWrite && (
                          <Button
                            isIconOnly
                            size="sm"
                            aria-label={`Delete ${file.name}`}
                            variant="light"
                            color="danger"
                            onPress={() => {
                              setDeleteId(file.id);
                              confirm.onOpen();
                            }}
                          >
                            <DeleteIcon className="w-4 h-4" />
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
      </DropZone>

      {/* File count summary */}
      {files.length > 0 && (
        <p className="text-xs text-default-400">
          {files.length} file{files.length !== 1 ? "s" : ""} &middot;{" "}
          {formatSize(files.reduce((acc, f) => acc + f.size, 0))} total
        </p>
      )}

      {/* Modals */}
      <ConfirmDialog
        isOpen={confirm.isOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null);
          confirm.onOpenChange();
        }}
        title="Delete file?"
        message="Are you sure you want to delete this file? This action cannot be undone."
        onConfirm={handleDelete}
        confirmLabel="Delete"
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
