import {Button, Card, CardBody, CardHeader, Spinner, useDisclosure} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useNavigate} from "react-router-dom";
import {useEffect, useState} from "react";
import {CreateSourceDialog} from "../../../features/source";
import {deleteSource, listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {showErrorToast} from "../../../shared/api/error";

export function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();
  const create = useDisclosure();
  const confirm = useDisclosure();
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  async function handleDelete() {
    if (!deleteId) return;
    setIsDeleting(true);
    try {
      await deleteSource(deleteId);
      addToast({title: "Source deleted", color: "success", timeout: 4000});
      await load();
    } catch (err) {
      showErrorToast(err);
    } finally {
      setIsDeleting(false);
      confirm.onClose();
    }
  }

  async function load() {
    setIsLoading(true);
    setError(null);
    try {
      setSources(await listSources());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load sources");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="p-8 flex flex-col gap-6">
      <div className="flex justify-between items-center">
        <h1 className="text-3xl font-bold">Sources</h1>
        <Button color="primary" onPress={create.onOpen}>
          Add Source
        </Button>
      </div>
      {isLoading ? (
        <div className="flex items-center justify-center min-h-[200px]">
          <Spinner size="lg" label="Loading sources..." />
        </div>
      ) : error ? (
        <EmptyState
          title="Failed to load sources"
          description={error}
          ctaLabel="Retry"
          onCtaPress={load}
          variant="danger"
        />
      ) : sources.length === 0 ? (
        <EmptyState
          title="No sources yet"
          description="Connect a Google Drive, Telegram, or S3 source to get started."
          ctaLabel="Add Source"
          onCtaPress={create.onOpen}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {sources.map((s) => (
            <Card
              key={s.id}
              isPressable
              onPress={() => navigate(`/sources/${s.id}`)}
            >
              <CardHeader className="flex justify-between items-center font-bold">
                {s.name}
                <Button
                  isIconOnly
                  variant="light"
                  color="danger"
                  onClick={(e) => {
                    e.stopPropagation();
                    setDeleteId(s.id);
                    confirm.onOpen();
                  }}
                >
                  <DeleteIcon className="w-4 h-4" />
                </Button>
              </CardHeader>
              <CardBody className="flex justify-between items-center">
                <SourceTypeChip type={s.type} />
                {new Date(s.created_at).toLocaleString()}
              </CardBody>
            </Card>
          ))}
        </div>
      )}
      <CreateSourceDialog isOpen={create.isOpen} onOpenChange={create.onOpenChange} onCreated={load} />
      <ConfirmDialog
        isOpen={confirm.isOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null);
          confirm.onOpenChange();
        }}
        title="Delete source?"
        message="Are you sure you want to delete this source?"
        onConfirm={handleDelete}
        confirmLabel="Delete"
        isConfirmLoading={isDeleting}
      />
    </div>
  );
}
