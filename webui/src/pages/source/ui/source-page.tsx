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
    <div className="p-8 flex flex-col gap-6 max-w-5xl mx-auto w-full">
      <div className="flex items-center gap-3">
        <Button isIconOnly variant="light" size="sm" onPress={() => navigate(-1)}>
          <ArrowLeftIcon className="w-5 h-5" />
        </Button>
        <h1 className="text-2xl font-semibold">{info.name}</h1>
      </div>
      <SourceTypeChip type={info.type} />
      <div className="flex justify-center">
        <CircularProgress
          classNames={{
            svg: "w-36 h-36 drop-shadow-md",
            indicator: "stroke-primary",
            track: "stroke-default-200",
            value: "text-3xl font-semibold text-foreground",
          }}
          showValueLabel
          strokeWidth={4}
          value={percent}
        />
      </div>
      <div className="border-2 border-dashed border-default-300 rounded-lg p-6">
        {info.files.length === 0 ? (
          <EmptyState
            title="No files in this source"
            description="Files will appear here once synced from the connected service."
          />
        ) : (
          <table className="w-full text-left text-sm">
            <thead>
              <tr>
                <th className="pb-3 text-default-500 font-medium">Name</th>
                <th className="pb-3 text-default-500 font-medium">Size</th>
                <th className="pb-3"></th>
              </tr>
            </thead>
            <tbody>
              {info.files.map((f) => (
                <tr key={f.id} className="border-t border-default-200 hover:bg-default-50 transition-colors">
                  <td className="py-3">{f.name}</td>
                  <td className="py-3">{f.size}</td>
                  <td className="py-3">
                    <div className="flex justify-end">
                      <Button
                        isIconOnly
                        variant="light"
                        size="sm"
                        onPress={() => downloadFile(id!, f)}
                      >
                        <DownloadIcon className="w-4 h-4" />
                      </Button>
                    </div>
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
