import { Button, useDisclosure } from "@heroui/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  deleteFile,
  downloadFile,
  listBuckets,
  listFiles,
  uploadFile,
} from "../../../shared/api/buckets";
import type { Bucket, FileInfo } from "../../../shared/api/buckets";
import { DownloadIcon } from "../../../shared/icons";
import { DeleteIcon } from "@heroui/shared-icons";
import { ConfirmDialog } from "../../../shared/ui";
import { ArrowLeftIcon } from "@heroui/shared-icons";
import { FilePreviewModal } from "../../../features/bucket/ui/file-preview-modal";
import { formatSize } from "../../../shared/lib/format";

export function BucketPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [bucket, setBucket] = useState<Bucket | null>(null);
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const fileInput = useRef<HTMLInputElement>(null);
  const confirm = useDisclosure();
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileInfo | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setIsLoading(true);
    try {
      const [buckets, fs] = await Promise.all([listBuckets(), listFiles(id)]);
      setBucket(buckets.find((b) => b.id === id) || null);
      setFiles(fs);
    } finally {
      setIsLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleUpload(file: File) {
    if (!id) return;
    await uploadFile(id, file);
    await load();
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
      await load();
    } finally {
      setIsDeleting(false);
      confirm.onClose();
    }
  }

  if (isLoading) return <div className="p-8">Loading...</div>;
  if (!bucket) return <div className="p-8">Bucket not found</div>;

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
      <div className="flex justify-end">
        <input
          type="file"
          ref={fileInput}
          className="hidden"
          onChange={onFileChange}
        />
        <Button color="primary" onPress={() => fileInput.current?.click()}>
          Upload File
        </Button>
      </div>
      <div
        className="border-2 border-dashed rounded p-4"
        onDragOver={(e) => e.preventDefault()}
        onDrop={onDrop}
      >
        {files.length === 0 ? (
          <p>No files</p>
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
                      <Button
                        isIconOnly
                        variant="light"
                        onPress={() => downloadFile(id!, f)}
                      >
                        <DownloadIcon className="w-5 h-5" />
                      </Button>
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
    </div>
  );
}
