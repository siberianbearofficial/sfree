import {Button, Card, CardBody, Chip, CircularProgress, Spinner} from "@heroui/react";
import {useCallback, useEffect, useState} from "react";
import {Link, useNavigate, useParams} from "react-router-dom";
import {downloadFile, getSourceHealth, getSourceInfo} from "../../../shared/api/sources";
import type {SourceFile, SourceHealth, SourceInfo} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";
import {SourceTypeChip} from "../../../entities/source";
import {getSourceQuotaState, sourceHealthColor} from "../../../entities/source/lib/capacity";
import {DownloadIcon} from "../../../shared/icons";
import {EmptyState} from "../../../shared/ui";
import {formatSize} from "../../../shared/lib/format";

export function SourcePage() {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const [info, setInfo] = useState<SourceInfo | null>(null);
  const [health, setHealth] = useState<SourceHealth | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setIsLoading(true);
    setError(null);
    const [infoResult, healthResult] = await Promise.allSettled([
      getSourceInfo(id),
      getSourceHealth(id),
    ]);

    if (infoResult.status === "rejected") {
      setInfo(null);
      setHealth(null);
      setError(infoResult.reason instanceof Error ? infoResult.reason.message : "Failed to load source");
      setIsLoading(false);
      return;
    }

    setInfo(infoResult.value);
    setHealth(healthResult.status === "fulfilled" ? healthResult.value : null);
    setIsLoading(false);
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  async function handleDownload(file: SourceFile) {
    if (!id) return;
    try {
      await downloadFile(id, file);
    } catch (err) {
      showErrorToast(err);
    }
  }

  if (isLoading) {
    return (
      <div className="p-6 sm:p-8 flex items-center justify-center min-h-[200px]">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 sm:p-8">
        <Link to="/sources" className="text-sm text-default-500 hover:text-primary transition-colors">
          &larr; Sources
        </Link>
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
      <div className="p-6 sm:p-8">
        <Link to="/sources" className="text-sm text-default-500 hover:text-primary transition-colors">
          &larr; Sources
        </Link>
        <EmptyState
          title="Source not found"
          description="This source may have been deleted."
          ctaLabel="Back to Sources"
          onCtaPress={() => navigate("/sources")}
        />
      </div>
    );
  }

  const quota = getSourceQuotaState(health);
  const fileCount = info.files.length;

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-6">
      <Link to="/sources" className="text-sm text-default-500 hover:text-primary transition-colors">
        &larr; Sources
      </Link>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-bold">{info.name}</h1>
          <SourceTypeChip type={info.type} />
        </div>
        {health ? (
          <Chip size="sm" variant="flat" color={sourceHealthColor[health.status]}>
            {health.status}
          </Chip>
        ) : (
          <Chip size="sm" variant="flat" color="default">
            health unavailable
          </Chip>
        )}
      </div>
      <div className="grid gap-4 lg:grid-cols-[220px,minmax(0,1fr)]">
        <Card>
          <CardBody className="items-center justify-center gap-4 py-6 text-center">
            {quota.kind === "available" ? (
              <>
                <CircularProgress
                  aria-label={`Quota usage: ${quota.percent}%`}
                  classNames={{
                    svg: "w-24 h-24 sm:w-36 sm:h-36",
                    indicator:
                      health?.status === "healthy"
                        ? "stroke-success"
                        : health?.status === "degraded"
                          ? "stroke-warning"
                          : "stroke-danger",
                    track: "stroke-default-200",
                    value: "text-3xl font-semibold",
                  }}
                  showValueLabel
                  strokeWidth={4}
                  value={quota.percent}
                />
                <div className="flex flex-col gap-1">
                  <p className="text-sm font-medium">Native quota</p>
                  <p className="text-sm text-default-500">
                    {formatSize(quota.usedBytes)} of {formatSize(quota.totalBytes)}
                  </p>
                  <p className="text-xs text-default-400">
                    {formatSize(quota.freeBytes)} free
                  </p>
                </div>
              </>
            ) : (
              <div className="flex h-full flex-col items-center justify-center gap-2">
                <p className="text-sm font-medium">{quota.label}</p>
                <p className="text-sm text-default-500">{quota.description}</p>
              </div>
            )}
          </CardBody>
        </Card>
        <Card>
          <CardBody className="gap-4 py-6">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="rounded-lg border border-default-200 px-4 py-3">
                <p className="text-xs uppercase tracking-wide text-default-400">Stored In Source</p>
                <p className="mt-2 text-2xl font-semibold">{formatSize(info.storage_used)}</p>
                <p className="mt-1 text-sm text-default-500">
                  {fileCount} listed file{fileCount === 1 ? "" : "s"}
                </p>
              </div>
              <div className="rounded-lg border border-default-200 px-4 py-3">
                <p className="text-xs uppercase tracking-wide text-default-400">Capacity Signal</p>
                <p className="mt-2 text-base font-semibold">
                  {quota.kind === "available" ? "Provider quota available" : quota.label}
                </p>
                <p className="mt-1 text-sm text-default-500">
                  {quota.kind === "available"
                    ? "This meter comes from native provider quota metadata."
                    : quota.description}
                </p>
              </div>
            </div>
            {health && (
              <div className="rounded-lg border border-default-200 bg-default-50 px-4 py-3">
                <p className="text-xs uppercase tracking-wide text-default-400">Health Probe</p>
                <p className="mt-2 text-sm text-default-700">{health.message}</p>
              </div>
            )}
          </CardBody>
        </Card>
      </div>
      <div className="border-2 border-dashed rounded p-4">
        {info.files.length === 0 ? (
          <EmptyState
            title="No files in this source"
            description="Files will appear here once synced from the connected service."
          />
        ) : (
          <div className="overflow-x-auto -mx-4 px-4">
            <table className="w-full text-left">
              <thead>
                <tr>
                  <th scope="col" className="pb-2">Name</th>
                  <th scope="col" className="pb-2 whitespace-nowrap">Size</th>
                  <th scope="col" className="pb-2"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {info.files.map((f) => (
                  <tr key={f.id} className="border-t">
                    <td className="py-2 break-all">{f.name}</td>
                    <td className="py-2 whitespace-nowrap">{formatSize(f.size)}</td>
                    <td className="py-2">
                      <Button
                        isIconOnly
                        variant="light"
                        aria-label={`Download ${f.name}`}
                        onPress={() => handleDownload(f)}
                      >
                        <DownloadIcon className="w-5 h-5" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
