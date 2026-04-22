import {
  Button,
  Spinner,
  useDisclosure,
} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useEffect, useRef, useState} from "react";
import {useNavigate, useParams} from "react-router-dom";
import {
  deleteFile,
  downloadFile,
  listBuckets,
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
import {showErrorToast} from "../../../shared/api/error";

export function BucketPage() {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const [bucket, setBucket] = useState<Bucket | null>(null);
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const confirm = useDisclosure();
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileInfo | null>(null);

  const shareBucket = useDisclosure();
  const [shareFile, setShareFile] = useState<FileInfo | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setIsLoading(true);
    setError(null);
    try {
      const [buckets, fs] = await Promise.all([listBuckets(), listFiles(id)]);
      setBucket(buckets.find((b) => b.id === id) || null);
      setFiles(fs);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load bucket");
    } finally {
      setIsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleUpload(file: File) {
    if (!id) return;
    try {
      await uploadFile(id, file);
      addToast({title: "File uploaded", description: `${file.name} added to bucket`, color: "success", timeout: 4000});
      await load();
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
    const f = e.dataTransfer.files?.[0];
    if (f) handleUpload(f);
  }

  async function handleDelete() {
    if (!id || !deleteId) return;
    setIsDeleting(true);
    try {
      await deleteFile(id, deleteId);
      addToast({title: "File deleted", color: "success", timeout: 4000});
      await load();
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

  return (
    <div className="p-8 flex flex-col gap-6">
      <Button isIconOnly variant="light" onPress={() => navigate(-1)}>
        <ArrowLeftIcon className="w-5 h-5" />
      </Button>
      <div className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold">{bucket.key}</h1>
        <p>ID: {bucket.id}</p>
        <p>Access Key: {bucket.access_key}</p>
        <p>Created: {new Date(bucket.created_at).toLocaleString()}</p>
      </div>
      <div className="flex justify-end gap-2">
        {bucket.role === "owner" && (
          <Button variant="flat" onPress={shareBucket.onOpen}>
            Share Bucket
          </Button>
        )}
        {(bucket.role === "owner" || bucket.role === "editor") && (
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
        className="border-2 border-dashed rounded p-4"
        onDragOver={(e) => e.preventDefault()}
        onDrop={onDrop}
      >
        {files.length === 0 ? (
          <EmptyState
            title="No files yet"
            description="Drag and drop a file here, or use the Upload button."
            ctaLabel="Upload File"
            onCtaPress={() => fileInput.current?.click()}
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
                      {(bucket.role === "owner" || bucket.role === "editor") && (
                        <Button
                          isIconOnly
                          variant="light"
                          onPress={() => openShareModal(f)}
                        >
                          <ShareIcon className="w-5 h-5" />
                        </Button>
                      )}
                      <Button
                        isIconOnly
                        variant="light"
                        onPress={() => handleDownload(f)}
                      >
                        <DownloadIcon className="w-5 h-5" />
                      </Button>
                      {(bucket.role === "owner" || bucket.role === "editor") && (
                        <Button
                          isIconOnly
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
