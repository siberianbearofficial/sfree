import {Button, Card, CardBody, CardHeader, useDisclosure} from "@heroui/react";
import {useEffect, useState} from "react";
import {Link} from "react-router-dom";
import {CreateBucketDialog} from "../../../features/bucket";
import {deleteBucket, listBuckets} from "../../../shared/api/buckets";
import type {Bucket} from "../../../shared/api/buckets";
import {DeleteIcon} from "@heroui/shared-icons";

export function BucketsPage() {
  const [buckets, setBuckets] = useState<Bucket[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const create = useDisclosure();

  async function handleDelete(id: string) {
    if (!window.confirm("Delete bucket?")) return;
    await deleteBucket(id);
    await load();
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
            <Link key={b.id} to={`/buckets/${b.id}`}>
              <Card>
                <CardHeader className="flex justify-between items-center font-bold">
                  {b.key}
                  <Button
                    isIconOnly
                    variant="light"
                    color="danger"
                    onPress={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      handleDelete(b.id);
                    }}
                  >
                    <DeleteIcon className="w-4 h-4" />
                  </Button>
                </CardHeader>
                <CardBody>{new Date(b.created_at).toLocaleString()}</CardBody>
              </Card>
            </Link>
          ))}
        </div>
      )}
      <CreateBucketDialog isOpen={create.isOpen} onOpenChange={create.onOpenChange} onCreated={load} />
    </div>
  );
}
