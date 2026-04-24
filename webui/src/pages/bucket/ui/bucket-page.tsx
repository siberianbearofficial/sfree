import {
  Button,
  Input,
  Spinner,
  useDisclosure,
} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useDeferredValue, useEffect, useRef, useState} from "react";
import {useNavigate, useParams} from "react-router-dom";
import {
  deleteFile,
  downloadFile,
  getBucket,
  listFiles,
  uploadFile,
} from "../../../shared/api/buckets";
import type {Bucket, FileInfo} from "../../../shared/api/buckets";
import {DownloadIcon, ShareIcon} from "../../../shared/icons";
import {DeleteIcon, ArrowLeftIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {FilePreviewModal} from "../../../features/bucket/ui/file-preview-modal";
import {ShareBucketDialog} from "../../../features/bucket/ui/share-bucket-dialog";
import {ShareFileDialog} from "../../../features/bucket/ui/share-file-dialog";
import {formatSize} from "../../../shared/lib/format";
import {ApiError, showErrorToast} from "../../../shared/api/error";

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
  }, [id]);

  useEffect(() => {
    if (!id) return;
    void load(hasLoadedRef.current ? "refresh" : "initial");
  }, [id, load]);

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

  if (isLoading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[200px]">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <Button isIconOnly variant="light" onPress={() => navigate(-1)}>
          <ArrowLeftIcon className="w-5 h-5" />
        </Button>
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
      <div className="p-8">
        <Button isIconOnly variant="light" onPress={() => navigate("/buckets")}>
          <ArrowLeftIcon className="w-5 h-5" />
        </Button>
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
  const emptyTitle = activeSearchQuery ? "No matching files" : "No files yet";
  const emptyDescription = activeSearchQuery
    ? `No files in this bucket match "${activeSearchQuery}".`
    : canWrite
      ? "Drag and drop a file here, or use the Upload button."
      : "Files shared in this bucket will appear here.";

  return (
    <div className="p-8 flex flex-col gap-6">
      <Button
        isIconOnly
        aria-label="Back"
        variant="light"
        onPress={() => navigate(-1)}
      >
        <ArrowLeftIcon className="w-5 h-5" />
      </Button>
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold">{bucket.key}</h1>
        <p>ID: {bucket.id}</p>
        <p>Access Key: {bucket.access_key}</p>
        <p>Created: {new Date(bucket.created_at).toLocaleString()}</p>
      </div>
      <div className="flex justify-end gap-2">
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
        {files.length === 0 ? (
          <EmptyState
            title={emptyTitle}
            description={emptyDescription}
            ctaLabel={!activeSearchQuery && canWrite ? "Upload File" : undefined}
            onCtaPress={!activeSearchQuery && canWrite ? () => fileInput.current?.click() : undefined}
          />
        ) : (
          <table className="w-full text-left">
            <thead>
              <tr>
                <th className="pb-2">Name</th>
                <th className="pb-2">Size</th>
                <th className="pb-2">Created</th>
                <th className="pb-2"></th>
              </tr>
            </thead>
            <tbody>
              {files.map((f) => (
                <tr key={f.id} className="border-t">
                  <td className="py-2">
                    <button
                      type="button"
                      className="text-left hover:text-primary transition-colors cursor-pointer"
                      onClick={() => setPreviewFile(f)}
                    >
                      {f.name}
                    </button>
                  </td>
                  <td className="py-2">{formatSize(f.size)}</td>
                  <td className="py-2">
                    {new Date(f.created_at).toLocaleString()}
                  </td>
                  <td className="py-2">
                    <div className="flex gap-2">
                      {canWrite && (
                        <Button
                          isIconOnly
                          aria-label={`Share ${f.name}`}
                          variant="light"
                          onPress={() => openShareModal(f)}
                        >
                          <ShareIcon className="w-5 h-5" />
                        </Button>
                      )}
                      <Button
                        isIconOnly
                        aria-label={`Download ${f.name}`}
                        variant="light"
                        onPress={() => handleDownload(f)}
                      >
                        <DownloadIcon className="w-5 h-5" />
                      </Button>
                      {canWrite && (
                        <Button
                          isIconOnly
                          aria-label={`Delete ${f.name}`}
                          variant="light"
                          color="danger"
                          onPress={() => {
                            setDeleteId(f.id);
                            confirm.onOpen();
                          }}
                        >
                          <DeleteIcon className="w-5 h-5" />
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <ConfirmDialog
        isOpen={confirm.isOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null);
          confirm.onOpenChange();
        }}
        title="Delete file?"
        message="Are you sure you want to delete this file?"
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

function canManageBucket(bucket: Bucket) {
  return bucket.role === "owner";
}

function canWriteFiles(bucket: Bucket) {
  return bucket.role === "owner" || bucket.role === "editor";
}
