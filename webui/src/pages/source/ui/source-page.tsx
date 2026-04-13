import {Button, CircularProgress, Spinner} from "@heroui/react";
import {useEffect, useState} from "react";
import {useNavigate, useParams} from "react-router-dom";
import {downloadFile, getSourceInfo} from "../../../shared/api/sources";
import type {SourceInfo} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {DownloadIcon} from "../../../shared/icons";
import {ArrowLeftIcon} from "@heroui/shared-icons";
import {EmptyState} from "../../../shared/ui";

export function SourcePage() {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const [info, setInfo] = useState<SourceInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  function load() {
    if (!id) return;
    setIsLoading(true);
    setError(null);
    getSourceInfo(id)
      .then(setInfo)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load source"))
      .finally(() => setIsLoading(false));
  }

  useEffect(() => {
    load();
  }, [id]);

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
          title="Failed to load source"
          description={error}
          ctaLabel="Retry"
          onCtaPress={load}
          variant="danger"
        />
      </div>
    );
  }

  if (!info) {
    return (
      <div className="p-8">
        <Button isIconOnly variant="light" onPress={() => navigate("/sources")}>
          <ArrowLeftIcon className="w-5 h-5" />
        </Button>
        <EmptyState
          title="Source not found"
          description="This source may have been deleted."
          ctaLabel="Back to Sources"
          onCtaPress={() => navigate("/sources")}
        />
      </div>
    );
  }

  const percent = info.storage_total
    ? (info.storage_used / info.storage_total) * 100
    : 0;

  return (
    <div className="p-8 flex flex-col gap-6">
      <Button isIconOnly variant="light" onPress={() => navigate(-1)}>
        <ArrowLeftIcon className="w-5 h-5" />
      </Button>
      <h1 className="text-3xl font-bold">{info.name}</h1>
      <SourceTypeChip type={info.type} />
      <div className="flex justify-center">
        <CircularProgress
          classNames={{
            svg: "w-36 h-36 drop-shadow-md",
            indicator: "stroke-white",
            track: "stroke-white/10",
            value: "text-3xl font-semibold text-white",
          }}
          showValueLabel
          strokeWidth={4}
          value={percent}
        />
      </div>
      <div className="border-2 border-dashed rounded p-4">
        {info.files.length === 0 ? (
          <EmptyState
            title="No files in this source"
            description="Files will appear here once synced from the connected service."
          />
        ) : (
          <table className="w-full text-left">
            <thead>
              <tr>
                <th className="pb-2">Name</th>
                <th className="pb-2">Size</th>
                <th className="pb-2"></th>
              </tr>
            </thead>
            <tbody>
              {info.files.map((f) => (
                <tr key={f.id} className="border-t">
                  <td className="py-2">{f.name}</td>
                  <td className="py-2">{f.size}</td>
                  <td className="py-2">
                    <Button
                      isIconOnly
                      variant="light"
                      onPress={() => downloadFile(id!, f)}
                    >
                      <DownloadIcon className="w-5 h-5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
