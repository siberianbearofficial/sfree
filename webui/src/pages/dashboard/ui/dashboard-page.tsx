import {Card, CardBody, CardHeader, CircularProgress, useDisclosure} from "@heroui/react";
import {useEffect, useRef, useState} from "react";
import {useNavigate} from "react-router-dom";
import {listSources, getSourceInfo} from "../../../shared/api/sources";
import {listBuckets} from "../../../shared/api/buckets";
import {SourceTypeChip} from "../../../entities/source";
import {OnboardingHero} from "../../../features/onboarding";
import {CreateSourceDialog} from "../../../features/source";
import {CreateBucketDialog} from "../../../features/bucket";
import type {Source, SourceInfo} from "../../../shared/api/sources";
import type {Bucket} from "../../../shared/api/buckets";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

type SourceWithInfo = Source & {info: SourceInfo | null};

export function DashboardPage() {
  const navigate = useNavigate();
  const [sources, setSources] = useState<SourceWithInfo[]>([]);
  const [buckets, setBuckets] = useState<Bucket[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const createSource = useDisclosure();
  const createBucket = useDisclosure();
  const hasLoadedOnce = useRef(false);

  async function load() {
    if (!hasLoadedOnce.current) setIsLoading(true);
    try {
      const [srcList, bucketList] = await Promise.all([
        listSources(),
        listBuckets(),
      ]);
      setBuckets(bucketList);

      const withInfo = await Promise.all(
        srcList.map(async (s) => {
          try {
            const info = await getSourceInfo(s.id);
            return {...s, info};
          } catch {
            return {...s, info: null};
          }
        }),
      );
      setSources(withInfo);
    } catch {
      // keep empty state
    } finally {
      setIsLoading(false);
      hasLoadedOnce.current = true;
    }
  }

  useEffect(() => {
    load();
  }, []);

  const totalFiles = sources.reduce(
    (sum, s) => sum + (s.info?.files.length ?? 0),
    0,
  );
  const totalStorageUsed = sources.reduce(
    (sum, s) => sum + (s.info?.storage_used ?? 0),
    0,
  );

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
            <p className="text-4xl font-bold">{formatBytes(totalStorageUsed)}</p>
            <p className="text-sm text-default-500 mt-1">Storage Used</p>
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
              const percent =
                s.info && s.info.storage_total
                  ? (s.info.storage_used / s.info.storage_total) * 100
                  : 0;
              const used = s.info?.storage_used ?? 0;
              const total = s.info?.storage_total ?? 0;

              return (
                <Card
                  key={s.id}
                  isPressable
                  onPress={() => navigate(`/sources/${s.id}`)}
                >
                  <CardHeader className="flex justify-between items-center">
                    <div className="flex flex-col gap-1">
                      <span className="font-bold">{s.name}</span>
                      <SourceTypeChip type={s.type} />
                    </div>
                    {total > 0 && (
                      <CircularProgress
                        classNames={{
                          svg: "w-16 h-16",
                          indicator: "stroke-primary",
                          track: "stroke-default-200",
                          value: "text-xs font-semibold",
                        }}
                        showValueLabel
                        strokeWidth={4}
                        value={percent}
                      />
                    )}
                  </CardHeader>
                  <CardBody className="pt-0">
                    <div className="flex justify-between text-sm text-default-500">
                      <span>{fileCount} {fileCount === 1 ? "file" : "files"}</span>
                      {total > 0 && (
                        <span>
                          {formatBytes(used)} / {formatBytes(total)}
                        </span>
                      )}
                    </div>
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
