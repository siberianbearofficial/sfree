import {Button, Card, CardBody, CardHeader, useDisclosure} from "@heroui/react";
import {useNavigate} from "react-router-dom";
import {useEffect, useState} from "react";
import {CreateSourceDialog} from "../../../features/source";
import {deleteSource, listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {DeleteIcon} from "@heroui/shared-icons";
import {ConfirmDialog} from "../../../shared/ui";

export function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
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
      await deleteSource(deleteId);
      await load();
    } finally {
      setIsDeleting(false);
      confirm.onClose();
    }
  }

  async function load() {
    setIsLoading(true);
    try {
      setSources(await listSources());
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
      {isLoading && sources.length === 0 ? (
        <p>Loading...</p>
      ) : sources.length === 0 ? (
        <p>No sources yet</p>
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
