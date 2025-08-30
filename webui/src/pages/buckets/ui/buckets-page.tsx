import {Button, Card, CardBody, CardHeader, useDisclosure} from "@heroui/react";
import {useEffect, useState} from "react";
import {useNavigate} from "react-router-dom";
import {CreateBucketDialog} from "../../../features/bucket";
import {deleteBucket, listBuckets} from "../../../shared/api/buckets";
import type {Bucket} from "../../../shared/api/buckets";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog} from "../../../shared/ui";

export function BucketsPage() {
  const [buckets, setBuckets] = useState<Bucket[]>([]);
  const [isLoading, setIsLoading] = useState(false);
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
      await load();
    } finally {
      setIsDeleting(false);
      confirm.onClose();
    }
  }

  async function load() {
    setIsLoading(true);
    try {
      setBuckets(await listBuckets());
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
        <h1 className="text-3xl font-bold">Buckets</h1>
        <Button color="primary" onPress={create.onOpen}>
          Add Bucket
        </Button>
      </div>
      {isLoading && buckets.length === 0 ? (
        <p>Loading...</p>
      ) : buckets.length === 0 ? (
        <p>No buckets yet</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {buckets.map((b) => (
            <Card
              key={b.id}
              isPressable
              onPress={() => navigate(`/buckets/${b.id}`)}
            >
              <CardHeader className="flex justify-between items-center font-bold">
                {b.key}
                <Button
                  isIconOnly
                  variant="light"
                  color="danger"
                  onClick={(e) => {
                    e.stopPropagation();
                    setDeleteId(b.id);
                    confirm.onOpen();
                  }}
                >
                  <DeleteIcon className="w-4 h-4" />
                </Button>
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
