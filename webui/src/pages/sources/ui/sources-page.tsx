import {Button, Card, CardBody, CardHeader, useDisclosure} from "@heroui/react";
import {Link} from "react-router-dom";
import {useEffect, useState} from "react";
import {CreateSourceDialog} from "../../../features/source";
import {deleteSource, listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {DeleteIcon} from "@heroui/shared-icons";

export function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const create = useDisclosure();

  async function handleDelete(id: string) {
    if (!window.confirm("Delete source?")) return;
    await deleteSource(id);
    await load();
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
            <Link key={s.id} to={`/sources/${s.id}`}>
              <Card>
                <CardHeader className="flex justify-between items-center font-bold">
                  {s.name}
                  <Button
                    isIconOnly
                    variant="light"
                    color="danger"
                    onPress={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      handleDelete(s.id);
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
            </Link>
          ))}
        </div>
      )}
      <CreateSourceDialog isOpen={create.isOpen} onOpenChange={create.onOpenChange} onCreated={load} />
    </div>
  );
}
