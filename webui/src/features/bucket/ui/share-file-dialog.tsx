import {
  Button,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
} from "@heroui/react";
import {useEffect, useState} from "react";
import type {FileInfo} from "../../../shared/api/buckets";
import {
  createShareLink,
  deleteShareLink,
  listShareLinks,
} from "../../../shared/api/shares";
import type {ShareLinkInfo} from "../../../shared/api/shares";

type Props = {
  bucketId: string;
  file: FileInfo | null;
  onClose: () => void;
};

export function ShareFileDialog({bucketId, file, onClose}: Props) {
  const [shareLinks, setShareLinks] = useState<ShareLinkInfo[]>([]);
  const [isCreatingShare, setIsCreatingShare] = useState(false);
  const [copiedToken, setCopiedToken] = useState<string | null>(null);
  const isOpen = file !== null;

  useEffect(() => {
    if (!file) return;

    listShareLinks(bucketId, file.id)
      .then(setShareLinks)
      .catch(() => setShareLinks([]));
  }, [bucketId, file]);

  function handleClose() {
    setShareLinks([]);
    setCopiedToken(null);
    onClose();
  }

  async function handleCreateShare() {
    if (!file) return;

    setIsCreatingShare(true);
    try {
      const link = await createShareLink(bucketId, file.id);
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
      return;
    }
  }

  function copyShareUrl(link: ShareLinkInfo) {
    const url = `${window.location.origin}${link.url}`;
    navigator.clipboard.writeText(url);
    setCopiedToken(link.token);
    setTimeout(() => setCopiedToken(null), 2000);
  }

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) handleClose();
      }}
      size="lg"
      scrollBehavior="inside"
    >
      <ModalContent>
        {(closeModal) => (
          <>
            <ModalHeader>Share: {file?.name}</ModalHeader>
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
                      className="flex flex-col sm:flex-row sm:items-center gap-2 border rounded p-2 text-sm"
                    >
                      <code className="flex-1 min-w-0 break-all text-xs sm:text-sm">
                        {window.location.origin}{link.url}
                      </code>
                      <div className="flex items-center gap-2 shrink-0">
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
                      </div>
                      {link.expires_at && (
                        <span className="text-xs text-default-400">
                          expires {new Date(link.expires_at).toLocaleString()}
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              <Button variant="light" onPress={closeModal}>
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
  );
}
