import {Button, Card, CardBody, CardHeader, Chip, Spinner, useDisclosure} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useEffect, useState} from "react";
import {useNavigate} from "react-router-dom";
import {CreateBucketDialog} from "../../../features/bucket";
import {deleteBucket, listBuckets} from "../../../shared/api/buckets";
import type {Bucket} from "../../../shared/api/buckets";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog, EmptyState} from "../../../shared/ui";
import {showErrorToast} from "../../../shared/api/error";

export function BucketsPage() {
  const [buckets, setBuckets] = useState<Bucket[]>([]);
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
      await deleteBucket(deleteId);
      addToast({title: "Bucket deleted", color: "success", timeout: 4000});
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
      setBuckets(await listBuckets());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load buckets");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-6">
      <div className="flex justify-between items-center">
        <h1 className="text-3xl font-bold">Buckets</h1>
        <Button color="primary" onPress={create.onOpen}>
          Add Bucket
        </Button>
      </div>
      {isLoading ? (
        <div className="flex items-center justify-center min-h-[200px]">
          <Spinner size="lg" label="Loading buckets..." />
        </div>
      ) : error ? (
        <EmptyState
          title="Failed to load buckets"
          description={error}
          ctaLabel="Retry"
          onCtaPress={load}
          variant="danger"
        />
      ) : buckets.length === 0 ? (
        <EmptyState
          title="No buckets yet"
          description="Step 2: create a bucket to get S3-compatible access to your files. Almost there!"
          ctaLabel="Create Bucket"
          onCtaPress={create.onOpen}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {buckets.map((b) => (
            <Card
              key={b.id}
              isPressable
              onPress={() => navigate(`/buckets/${b.id}`)}
            >
              <CardHeader className="flex justify-between items-center font-bold">
                <span className="flex items-center gap-2">
                  {b.key}
                  {b.shared && (
                    <Chip size="sm" variant="flat" color="secondary">
                      {b.role}
                    </Chip>
                  )}
                </span>
                {b.role === "owner" && (
                  <Button
                    isIconOnly
                    variant="light"
                    color="danger"
                    aria-label={`Delete bucket ${b.key}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteId(b.id);
                      confirm.onOpen();
                    }}
                  >
                    <DeleteIcon className="w-4 h-4" />
                  </Button>
                )}
              </CardHeader>
              <CardBody>{new Date(b.created_at).toLocaleString()}</CardBody>
            </Card>
          ))}
        </div>
      )}
      <CreateBucketDialog isOpen={create.isOpen} onOpenChange={create.onOpenChange} onCreated={load} />
      <ConfirmDialog
        isOpen={confirm.isOpen}
        onOpenChange={(open) => {
          if (!open) setDeleteId(null);
          confirm.onOpenChange();
        }}
        title="Delete bucket?"
        message="Are you sure you want to delete this bucket?"
        onConfirm={handleDelete}
        confirmLabel="Delete"
        isConfirmLoading={isDeleting}
      />
    </div>
  );
}
