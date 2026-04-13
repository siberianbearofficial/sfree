import {
  Button,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
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
import {
  createShareLink,
  deleteShareLink,
  listShareLinks,
} from "../../../shared/api/shares";
import type {ShareLinkInfo} from "../../../shared/api/shares";
import {DownloadIcon, ShareIcon} from "../../../shared/icons";
import {DeleteIcon, ArrowLeftIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {FilePreviewModal} from "../../../features/bucket/ui/file-preview-modal";
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

  const shareModal = useDisclosure();
  const [shareFile, setShareFile] = useState<FileInfo | null>(null);
  const [shareLinks, setShareLinks] = useState<ShareLinkInfo[]>([]);
  const [isCreatingShare, setIsCreatingShare] = useState(false);
  const [copiedToken, setCopiedToken] = useState<string | null>(null);

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

  async function openShareModal(file: FileInfo) {
    if (!id) return;
    setShareFile(file);
    shareModal.onOpen();
    try {
      const links = await listShareLinks(id, file.id);
      setShareLinks(links);
    } catch {
      setShareLinks([]);
    }
  }

  async function handleCreateShare() {
    if (!id || !shareFile) return;
    setIsCreatingShare(true);
    try {
      const link = await createShareLink(id, shareFile.id);
      setShareLinks((prev) => [...prev, link]);
    } finally {
      setIsCreatingShare(false);
    }
  }

  async function handleDeleteShare(shareId: string) {
    try {
      await deleteShareLink(shareId);
      setShareLinks((prev) => prev.filter((l) => l.id !== shareId));
    } catch {
      // ignore
    }
  }

  function copyShareUrl(link: ShareLinkInfo) {
    const url = `${window.location.origin}${link.url}`;
    navigator.clipboard.writeText(url);
    setCopiedToken(link.token);
    setTimeout(() => setCopiedToken(null), 2000);
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
                      <Button
                        isIconOnly
                        variant="light"
                        onPress={() => openShareModal(f)}
                      >
                        <ShareIcon className="w-5 h-5" />
                      </Button>
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
      <Modal
        isOpen={shareModal.isOpen}
        onOpenChange={(open) => {
          if (!open) {
            setShareFile(null);
            setShareLinks([]);
            setCopiedToken(null);
          }
          shareModal.onOpenChange();
        }}
        size="lg"
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader>Share: {shareFile?.name}</ModalHeader>
              <ModalBody>
                {shareLinks.length === 0 ? (
                  <p className="text-sm text-gray-500">
                    No share links yet. Create one to share this file publicly.
                  </p>
                ) : (
                  <div className="flex flex-col gap-2">
                    {shareLinks.map((link) => (
                      <div
                        key={link.id}
                        className="flex items-center gap-2 border rounded p-2 text-sm"
                      >
                        <code className="flex-1 truncate">
                          {window.location.origin}{link.url}
                        </code>
                        <Button
                          size="sm"
                          variant="flat"
                          onPress={() => copyShareUrl(link)}
                        >
                          {copiedToken === link.token ? "Copied!" : "Copy"}
                        </Button>
                        <Button
                          size="sm"
                          variant="flat"
                          color="danger"
                          onPress={() => handleDeleteShare(link.id)}
                        >
                          Revoke
                        </Button>
                        {link.expires_at && (
                          <span className="text-xs text-gray-400">
                            expires {new Date(link.expires_at).toLocaleString()}
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose}>
                  Close
                </Button>
                <Button
                  color="primary"
                  isLoading={isCreatingShare}
                  onPress={handleCreateShare}
                >
                  Create Share Link
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
}
