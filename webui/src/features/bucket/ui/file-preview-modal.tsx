import {
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Button,
  Spinner,
  Chip,
} from "@heroui/react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { FileInfo } from "../../../shared/api/buckets";
import { fetchFileBlob, downloadFile } from "../../../shared/api/buckets";
import { showErrorToast } from "../../../shared/api/error";
import { DownloadIcon } from "../../../shared/icons";
import { formatSize } from "../../../shared/lib/format";

const IMAGE_EXTENSIONS = new Set(["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico"]);
const TEXT_EXTENSIONS = new Set(["txt", "json", "yaml", "yml", "md", "csv", "xml", "html", "css", "js", "ts", "tsx", "jsx", "go", "py", "sh", "toml", "ini", "cfg", "log", "env", "dockerfile"]);
const MAX_TEXT_PREVIEW_FILE_SIZE = 1_000_000;
const MAX_TEXT_PREVIEW_CONTENT_SIZE = 100_000;

function getExtension(name: string): string {
  const parts = name.toLowerCase().split(".");
  return parts.length > 1 ? parts[parts.length - 1] : "";
}

type Props = {
  isOpen: boolean;
  onClose: () => void;
  file: FileInfo | null;
  bucketId: string;
};

export function FilePreviewModal({ isOpen, onClose, file, bucketId }: Props) {
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [textContent, setTextContent] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const blobUrlRef = useRef<string | null>(null);

  const ext = file ? getExtension(file.name) : "";
  const isImage = IMAGE_EXTENSIONS.has(ext);
  const isText = TEXT_EXTENSIONS.has(ext);
  const textPreviewTooLarge = Boolean(file && isText && file.size > MAX_TEXT_PREVIEW_FILE_SIZE);

  async function handleDownload() {
    if (!file) return;
    try {
      await downloadFile(bucketId, file);
    } catch (err) {
      showErrorToast(err);
    }
  }

  const loadPreview = useCallback(async () => {
    if (!file) return;
    setIsLoading(true);
    setError(null);
    if (blobUrlRef.current) {
      URL.revokeObjectURL(blobUrlRef.current);
      blobUrlRef.current = null;
    }
    setBlobUrl(null);
    setTextContent(null);
    try {
      const blob = await fetchFileBlob(bucketId, file.id);
      if (isImage) {
        const url = URL.createObjectURL(blob);
        blobUrlRef.current = url;
        setBlobUrl(url);
      } else if (isText) {
        const text = await blob.text();
        setTextContent(
          text.length > MAX_TEXT_PREVIEW_CONTENT_SIZE
            ? text.slice(0, MAX_TEXT_PREVIEW_CONTENT_SIZE) + "\n\n--- truncated ---"
            : text,
        );
      }
    } catch {
      setError("Failed to load file preview");
    } finally {
      setIsLoading(false);
    }
  }, [file, bucketId, isImage, isText]);

  useEffect(() => {
    if (blobUrlRef.current) {
      URL.revokeObjectURL(blobUrlRef.current);
      blobUrlRef.current = null;
    }
    setBlobUrl(null);
    setTextContent(null);
    setError(null);
    setIsLoading(false);
    if (isOpen && file && (isImage || (isText && !textPreviewTooLarge))) {
      void loadPreview();
    }
    return () => {
      if (blobUrlRef.current) {
        URL.revokeObjectURL(blobUrlRef.current);
        blobUrlRef.current = null;
      }
    };
  }, [isOpen, file, isImage, isText, loadPreview, textPreviewTooLarge]);

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="3xl" scrollBehavior="inside">
      <ModalContent>
        <ModalHeader className="flex flex-col gap-1">
          <span className="truncate">{file?.name}</span>
        </ModalHeader>
        <ModalBody>
          {file && (
            <div className="flex flex-wrap gap-2 mb-4">
              <Chip size="sm" variant="flat">{formatSize(file.size)}</Chip>
              {ext && <Chip size="sm" variant="flat">.{ext}</Chip>}
              <Chip size="sm" variant="flat">
                {new Date(file.created_at).toLocaleString()}
              </Chip>
            </div>
          )}

          {isLoading && (
            <div className="flex justify-center py-12">
              <Spinner size="lg" />
            </div>
          )}

          {error && (
            <div className="text-center text-danger py-8">{error}</div>
          )}

          {!isLoading && !error && isImage && blobUrl && (
            <div className="flex justify-center">
              <img
                src={blobUrl}
                alt={file?.name}
                className="max-w-full max-h-[60vh] object-contain rounded"
              />
            </div>
          )}

          {!isLoading && !error && isText && textContent !== null && (
            <pre className="bg-default-100 rounded-lg p-4 overflow-auto max-h-[60vh] text-sm font-mono whitespace-pre-wrap break-words">
              {textContent}
            </pre>
          )}

          {!isLoading && !error && textPreviewTooLarge && file && (
            <div className="text-center py-12 text-default-500">
              <p className="text-lg mb-2">Preview unavailable for large text files</p>
              <p>
                Files larger than {formatSize(MAX_TEXT_PREVIEW_FILE_SIZE)} must be downloaded to inspect.
              </p>
            </div>
          )}

          {!isLoading && !error && !textPreviewTooLarge && !isImage && !isText && file && (
            <div className="text-center py-12 text-default-500">
              <p className="text-lg mb-2">Preview not available for this file type</p>
              <p>Click Download to save the file</p>
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <Button variant="light" onPress={onClose}>
            Close
          </Button>
          {file && (
            <Button
              color="primary"
              startContent={<DownloadIcon className="w-4 h-4" />}
              onPress={handleDownload}
            >
              Download
            </Button>
          )}
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
