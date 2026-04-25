import {Button, Card, CardBody, CardHeader, Chip, CircularProgress, Spinner} from "@heroui/react";
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

function formatLatency(ms: number): string {
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(1)} s`;
}

function formatCheckedAt(iso: string): string {
  try {
    const d = new Date(iso);
    const now = Date.now();
    const diffSec = Math.floor((now - d.getTime()) / 1000);
    if (diffSec < 60) return "just now";
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)} min ago`;
    return d.toLocaleString();
  } catch {
    return iso;
  }
}

const PROVIDER_CAPABILITIES: Record<string, {quota: boolean; fileList: boolean; download: boolean; caveat: string}> = {
  gdrive: {
    quota: true,
    fileList: true,
    download: true,
    caveat: "Quota comes from native Google Drive metadata. Shared-drive files count against the drive owner's quota, not yours.",
  },
  s3: {
    quota: false,
    fileList: true,
    download: true,
    caveat: "S3-compatible providers do not expose provider-wide capacity via API. SFree checks bucket reachability only.",
  },
  telegram: {
    quota: false,
    fileList: false,
    download: false,
    caveat: "Telegram does not expose storage metadata. SFree checks bot/chat accessibility only.",
  },
};

function CapabilityDot({supported}: {supported: boolean}) {
  return (
    <span
      className={`inline-block w-2 h-2 rounded-full ${supported ? "bg-success" : "bg-default-300"}`}
      aria-hidden="true"
    />
  );
}

function HealthBanner({health}: {health: SourceHealth}) {
  const color = sourceHealthColor[health.status];
  return (
    <Card>
      <CardBody className="gap-3 py-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <span
              className={`inline-block w-3 h-3 rounded-full ${color === "success" ? "bg-success" : color === "warning" ? "bg-warning" : "bg-danger"}`}
              aria-hidden="true"
            />
            <span className="text-lg font-semibold capitalize">{health.status}</span>
            <Chip size="sm" variant="flat" color={color}>
              {health.reason_code}
            </Chip>
          </div>
          <div className="flex items-center gap-4 text-sm text-default-500">
            <span>Latency: {formatLatency(health.latency_ms)}</span>
            <span>Checked {formatCheckedAt(health.checked_at)}</span>
          </div>
        </div>
        {health.message && (
          <p className="text-sm text-default-600">{health.message}</p>
        )}
      </CardBody>
    </Card>
  );
}

function QuotaCard({health, quota}: {health: SourceHealth | null; quota: ReturnType<typeof getSourceQuotaState>}) {
  return (
    <Card className="flex-1">
      <CardHeader className="pb-0">
        <p className="text-xs uppercase tracking-wide text-default-400">Quota</p>
      </CardHeader>
      <CardBody className="items-center justify-center gap-4 py-5 text-center">
        {quota.kind === "available" ? (
          <>
            <CircularProgress
              aria-label={`Quota usage: ${quota.percent.toFixed(0)}%`}
              classNames={{
                svg: "w-28 h-28",
                indicator:
                  health?.status === "healthy"
                    ? "stroke-success"
                    : health?.status === "degraded"
                      ? "stroke-warning"
                      : "stroke-danger",
                track: "stroke-default-200",
                value: "text-2xl font-semibold",
              }}
              showValueLabel
              strokeWidth={4}
              value={quota.percent}
            />
            <div className="flex flex-col gap-0.5">
              <p className="text-sm text-default-600">
                {formatSize(quota.usedBytes)} / {formatSize(quota.totalBytes)}
              </p>
              <p className="text-xs text-default-400">{formatSize(quota.freeBytes)} free</p>
            </div>
          </>
        ) : (
          <div className="flex flex-col items-center gap-1 py-2">
            <p className="text-sm font-medium text-default-600">{quota.label}</p>
            <p className="text-xs text-default-400 max-w-[200px]">{quota.description}</p>
          </div>
        )}
      </CardBody>
    </Card>
  );
}

function StorageCard({info}: {info: SourceInfo}) {
  const fileCount = info.files.length;
  return (
    <Card className="flex-1">
      <CardHeader className="pb-0">
        <p className="text-xs uppercase tracking-wide text-default-400">Storage Used</p>
      </CardHeader>
      <CardBody className="items-center justify-center gap-4 py-5 text-center">
        <p className="text-3xl font-bold">{formatSize(info.storage_used)}</p>
        <p className="text-sm text-default-500">
          {fileCount} file{fileCount === 1 ? "" : "s"} stored
        </p>
      </CardBody>
    </Card>
  );
}

function CapabilitiesCard({sourceType}: {sourceType: string}) {
  const caps = PROVIDER_CAPABILITIES[sourceType];
  if (!caps) return null;

  const rows: {label: string; supported: boolean}[] = [
    {label: "Native quota reporting", supported: caps.quota},
    {label: "File listing", supported: caps.fileList},
    {label: "Direct file download", supported: caps.download},
  ];

  return (
    <Card>
      <CardHeader className="pb-0">
        <p className="text-xs uppercase tracking-wide text-default-400">Provider Capabilities</p>
      </CardHeader>
      <CardBody className="gap-3 py-5">
        <ul className="flex flex-col gap-2">
          {rows.map((r) => (
            <li key={r.label} className="flex items-center gap-2 text-sm">
              <CapabilityDot supported={r.supported} />
              <span className={r.supported ? "text-default-700" : "text-default-400"}>{r.label}</span>
            </li>
          ))}
        </ul>
        <p className="text-xs text-default-400 border-t border-default-100 pt-3">{caps.caveat}</p>
      </CardBody>
    </Card>
  );
}

function FilesSection({
  files,
  onDownload,
}: {
  files: SourceFile[];
  onDownload: (f: SourceFile) => void;
}) {
  const [expanded, setExpanded] = useState(false);

  if (files.length === 0) {
    return (
      <Card>
        <CardHeader>
          <p className="text-xs uppercase tracking-wide text-default-400">Source Files</p>
        </CardHeader>
        <CardBody>
          <EmptyState
            title="No files in this source"
            description="Files will appear here once synced from the connected service."
          />
        </CardBody>
      </Card>
    );
  }

  return (
    <Card>
      <CardBody className="p-0">
        <button
          type="button"
          className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-default-100 transition-colors rounded-t-large"
          onClick={() => setExpanded(!expanded)}
          aria-expanded={expanded}
        >
          <span className="text-xs uppercase tracking-wide text-default-400">
            Source Files ({files.length})
          </span>
          <span
            className={`text-default-400 transition-transform ${expanded ? "rotate-180" : ""}`}
            aria-hidden="true"
          >
            &#9662;
          </span>
        </button>
        {expanded && (
          <div className="overflow-x-auto px-4 pb-4">
            <table className="w-full text-left text-sm">
              <thead>
                <tr>
                  <th scope="col" className="pb-2 font-medium text-default-500">Name</th>
                  <th scope="col" className="pb-2 font-medium text-default-500 whitespace-nowrap">Size</th>
                  <th scope="col" className="pb-2"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {files.map((f) => (
                  <tr key={f.id} className="border-t border-default-100">
                    <td className="py-2 break-all text-default-700">{f.name}</td>
                    <td className="py-2 whitespace-nowrap text-default-500">{formatSize(f.size)}</td>
                    <td className="py-2">
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        aria-label={`Download ${f.name}`}
                        onPress={() => onDownload(f)}
                      >
                        <DownloadIcon className="w-4 h-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardBody>
    </Card>
  );
}

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

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-5">
      <Link to="/sources" className="text-sm text-default-500 hover:text-primary transition-colors">
        &larr; Sources
      </Link>

      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
        <h1 className="text-2xl font-bold">{info.name}</h1>
        <SourceTypeChip type={info.type} />
      </div>

      {/* Health banner — primary surface */}
      {health ? (
        <HealthBanner health={health} />
      ) : (
        <Card>
          <CardBody className="py-4">
            <p className="text-sm text-default-500">
              Health data unavailable. SFree could not run a provider probe for this source.
            </p>
          </CardBody>
        </Card>
      )}

      {/* Quota + Storage — side by side */}
      <div className="grid gap-4 sm:grid-cols-2">
        <QuotaCard health={health} quota={quota} />
        <StorageCard info={info} />
      </div>

      {/* Provider capabilities */}
      <CapabilitiesCard sourceType={info.type} />

      {/* Files — visually secondary, collapsed by default */}
      <FilesSection files={info.files} onDownload={handleDownload} />
    </div>
  );
}
