import {Card, CardBody, CardHeader, Chip, CircularProgress, useDisclosure} from "@heroui/react";
import {useCallback, useEffect, useRef, useState} from "react";
import {useNavigate} from "react-router-dom";
import {listSources, getSourceHealth, getSourceInfo} from "../../../shared/api/sources";
import {listBuckets} from "../../../shared/api/buckets";
import {SourceTypeChip} from "../../../entities/source";
import {getSourceQuotaState, sourceHealthColor} from "../../../entities/source/lib/capacity";
import {OnboardingHero} from "../../../features/onboarding";
import {CreateSourceDialog} from "../../../features/source";
import {CreateBucketDialog} from "../../../features/bucket";
import type {Source, SourceHealth, SourceInfo} from "../../../shared/api/sources";
import type {Bucket} from "../../../shared/api/buckets";
import {formatSize} from "../../../shared/lib/format";

type SourceWithDetails = Source & {
  info: SourceInfo | null;
  health: SourceHealth | null;
};

export function DashboardPage() {
  const navigate = useNavigate();
  const [sources, setSources] = useState<SourceWithDetails[]>([]);
  const [buckets, setBuckets] = useState<Bucket[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const createSource = useDisclosure();
  const createBucket = useDisclosure();
  const hasLoadedOnce = useRef(false);

  const load = useCallback(async () => {
    if (!hasLoadedOnce.current) setIsLoading(true);
    try {
      const [srcList, bucketList] = await Promise.all([
        listSources(),
        listBuckets(),
      ]);
      setBuckets(bucketList);

      const withDetails = await Promise.all(
        srcList.map(async (s) => {
          const [infoResult, healthResult] = await Promise.allSettled([
            getSourceInfo(s.id),
            getSourceHealth(s.id),
          ]);
          return {
            ...s,
            info: infoResult.status === "fulfilled" ? infoResult.value : null,
            health: healthResult.status === "fulfilled" ? healthResult.value : null,
          };
        }),
      );
      setSources(withDetails);
    } catch {
      // keep empty state
    } finally {
      setIsLoading(false);
      hasLoadedOnce.current = true;
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const totalFiles = sources.reduce(
    (sum, s) => sum + (s.info?.files.length ?? 0),
    0,
  );
  const quotaReportingSources = sources.filter(
    (s) => getSourceQuotaState(s.health).kind === "available",
  ).length;
  const sourcesNeedingAttention = sources.filter(
    (s) => s.health && s.health.status !== "healthy",
  ).length;

  const hasSources = sources.length > 0;
  const hasBuckets = buckets.length > 0;
  const showOnboarding = !hasSources || !hasBuckets;

  if (isLoading) {
    return (
      <div className="p-6 sm:p-8 flex flex-col gap-8">
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-default-500">Loading...</p>
      </div>
    );
  }

  return (
    <div className="p-6 sm:p-8 flex flex-col gap-8">
      <h1 className="text-3xl font-bold">Dashboard</h1>

      {showOnboarding && (
        <OnboardingHero
          hasSources={hasSources}
          hasBuckets={hasBuckets}
          onAddSource={createSource.onOpen}
          onAddBucket={createBucket.onOpen}
          onGoToBucket={() => {
            if (buckets.length > 0) navigate(`/buckets/${buckets[0].id}`);
          }}
        />
      )}

      {/* Summary stat cards */}
      <div className="grid gap-4 grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardBody className="text-center py-6">
            <p className="text-4xl font-bold">{sources.length}</p>
            <p className="text-sm text-default-500 mt-1">Sources</p>
          </CardBody>
        </Card>
        <Card>
          <CardBody className="text-center py-6">
            <p className="text-4xl font-bold">{buckets.length}</p>
            <p className="text-sm text-default-500 mt-1">Buckets</p>
          </CardBody>
        </Card>
        <Card>
          <CardBody className="text-center py-6">
            <p className="text-4xl font-bold">{totalFiles}</p>
            <p className="text-sm text-default-500 mt-1">Files</p>
          </CardBody>
        </Card>
        <Card>
          <CardBody className="text-center py-6">
            <p className="text-4xl font-bold">
              {sources.length === 0 ? "0" : `${quotaReportingSources}/${sources.length}`}
            </p>
            <p className="text-sm text-default-500 mt-1">Sources Reporting Quota</p>
            <p className="text-xs text-default-400 mt-2">
              {sourcesNeedingAttention === 0
                ? "No active provider warnings"
                : `${sourcesNeedingAttention} source${sourcesNeedingAttention === 1 ? "" : "s"} need attention`}
            </p>
          </CardBody>
        </Card>
      </div>

      {/* Sources breakdown */}
      {hasSources && (
        <div>
          <h2 className="text-xl font-semibold mb-4">Sources</h2>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {sources.map((s) => {
              const fileCount = s.info?.files.length ?? 0;
              const used = s.info?.storage_used ?? 0;
              const quota = getSourceQuotaState(s.health);

              return (
                <Card
                  key={s.id}
                  isPressable
                  onPress={() => navigate(`/sources/${s.id}`)}
                >
                  <CardHeader className="flex justify-between items-start gap-3">
                    <div className="flex flex-col gap-1">
                      <span className="font-bold">{s.name}</span>
                      <SourceTypeChip type={s.type} />
                    </div>
                    {s.health ? (
                      <Chip
                        size="sm"
                        variant="flat"
                        color={sourceHealthColor[s.health.status]}
                      >
                        {s.health.status}
                      </Chip>
                    ) : (
                      <Chip size="sm" variant="flat" color="default">
                        health unavailable
                      </Chip>
                    )}
                  </CardHeader>
                  <CardBody className="pt-0 flex flex-col gap-3">
                    <div className="flex justify-between text-sm text-default-500">
                      <span>{fileCount} {fileCount === 1 ? "file" : "files"}</span>
                      <span>{formatSize(used)} stored</span>
                    </div>
                    {quota.kind === "available" ? (
                      <div className="flex items-center gap-4">
                        <CircularProgress
                          classNames={{
                            svg: "w-14 h-14",
                            indicator:
                              s.health?.status === "healthy"
                                ? "stroke-success"
                                : s.health?.status === "degraded"
                                  ? "stroke-warning"
                                  : "stroke-danger",
                            track: "stroke-default-200",
                            value: "text-xs font-semibold",
                          }}
                          showValueLabel
                          strokeWidth={4}
                          value={quota.percent}
                        />
                        <div className="flex flex-col">
                          <span className="text-sm font-medium">Quota</span>
                          <span className="text-sm text-default-500">
                            {formatSize(quota.usedBytes)} / {formatSize(quota.totalBytes)}
                          </span>
                          <span className="text-xs text-default-400">
                            {formatSize(quota.freeBytes)} free
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-lg border border-default-200 bg-default-50 px-3 py-2">
                        <p className="text-sm font-medium">{quota.label}</p>
                        <p className="text-xs text-default-500">{quota.description}</p>
                      </div>
                    )}
                    {s.health && s.health.status !== "healthy" && (
                      <p className="text-xs text-default-500">{s.health.message}</p>
                    )}
                  </CardBody>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      <CreateSourceDialog isOpen={createSource.isOpen} onOpenChange={createSource.onOpenChange} onCreated={load} />
      <CreateBucketDialog isOpen={createBucket.isOpen} onOpenChange={createBucket.onOpenChange} onCreated={load} />
    </div>
  );
}
