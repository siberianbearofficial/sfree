import {Button, Card, CardBody, CardHeader, Chip, Spinner, Tooltip, useDisclosure} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useNavigate} from "react-router-dom";
import {useCallback, useEffect, useState} from "react";
import {CreateSourceDialog} from "../../../features/source";
import {deleteSource, getSourceHealth, listSources} from "../../../shared/api/sources";
import type {Source, SourceHealth} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {sourceHealthColor} from "../../../entities/source/lib/capacity";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {showErrorToast} from "../../../shared/api/error";

type HealthState =
  | {state: "checking"}
  | {state: "ready"; health: SourceHealth}
  | {state: "error"; message: string};

function RefreshIcon(props: {className?: string}) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...props}>
      <path d="M20 6v5h-5" />
      <path d="M4 18v-5h5" />
      <path d="M18.5 9A7 7 0 0 0 6.2 6.2L4 8.3" />
      <path d="M5.5 15a7 7 0 0 0 12.3 2.8L20 15.7" />
    </svg>
  );
}

export function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
  const [healthBySource, setHealthBySource] = useState<Record<string, HealthState>>({});
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

  async function refreshHealth(sourceId: string) {
    setHealthBySource((current) => ({
      ...current,
      [sourceId]: {state: "checking"},
    }));
    try {
      const health = await getSourceHealth(sourceId);
      setHealthBySource((current) => ({
        ...current,
        [sourceId]: {state: "ready", health},
      }));
    } catch (err) {
      setHealthBySource((current) => ({
        ...current,
        [sourceId]: {
          state: "error",
          message: err instanceof Error ? err.message : "Health check failed",
        },
      }));
    }
  }

  function refreshAllHealth(nextSources: Source[]) {
    const checking = Object.fromEntries(nextSources.map((s) => [s.id, {state: "checking"} as HealthState]));
    setHealthBySource(checking);
    nextSources.forEach((source) => {
      void refreshHealth(source.id);
    });
  }

  const load = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const nextSources = await listSources();
      setSources(nextSources);
      refreshAllHealth(nextSources);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load sources");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  function renderHealth(source: Source) {
    const health = healthBySource[source.id];
    if (!health || health.state === "checking") {
      return <Chip size="sm" variant="flat" color="default">Checking</Chip>;
    }
    if (health.state === "error") {
      return (
        <Tooltip content={health.message}>
          <Chip size="sm" variant="flat" color="danger">Check failed</Chip>
        </Tooltip>
      );
    }
    return (
      <Tooltip content={health.health.message}>
        <Chip size="sm" variant="flat" color={sourceHealthColor[health.health.status]}>
          {health.health.status}
        </Chip>
      </Tooltip>
    );
  }

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-6">
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
          description="This is step 1: connect a Google Drive, Telegram, or S3-compatible source so SFree knows where your files live."
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
                <div className="flex items-center gap-1">
                  <Tooltip content="Refresh health">
                    <Button
                      isIconOnly
                      variant="light"
                      aria-label="Refresh health"
                      isLoading={healthBySource[s.id]?.state === "checking"}
                      onClick={(e) => {
                        e.stopPropagation();
                        void refreshHealth(s.id);
                      }}
                    >
                      <RefreshIcon className="w-4 h-4" />
                    </Button>
                  </Tooltip>
                  <Button
                    isIconOnly
                    variant="light"
                    color="danger"
                    aria-label="Delete source"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteId(s.id);
                      confirm.onOpen();
                    }}
                  >
                    <DeleteIcon className="w-4 h-4" />
                  </Button>
                </div>
              </CardHeader>
              <CardBody className="flex flex-col gap-3">
                <div className="flex justify-between items-center">
                  <SourceTypeChip type={s.type} />
                  {renderHealth(s)}
                </div>
                <div className="text-small text-default-500">
                  {new Date(s.created_at).toLocaleString()}
                </div>
              </CardBody>
            </Card>
          ))}
        </div>
      )}
      <CreateSourceDialog isOpen={create.isOpen} onOpenChange={create.onOpenChange} onCreated={load} onNavigateToSource={(id) => navigate(`/sources/${id}`)} />
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
