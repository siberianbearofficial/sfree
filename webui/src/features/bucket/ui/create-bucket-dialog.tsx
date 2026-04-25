import {Button, Checkbox, CheckboxGroup, Chip, Divider, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet, Spinner} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {createBucket} from "../../../shared/api/buckets";
import {getSourceHealth, listSources} from "../../../shared/api/sources";
import type {Source, SourceHealth} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";
import {SourceTypeChip} from "../../../entities/source";
import {getSourceQuotaState, sourceHealthColor} from "../../../entities/source/lib/capacity";
import {formatSize} from "../../../shared/lib/format";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

type HealthState =
  | {state: "checking"}
  | {state: "ready"; health: SourceHealth}
  | {state: "error"; message: string};

export function CreateBucketDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [key, setKey] = useState("");
  const [sources, setSources] = useState<Source[]>([]);
  const [healthBySource, setHealthBySource] = useState<Record<string, HealthState>>({});
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [creds, setCreds] = useState<{accessKey: string; accessSecret: string} | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSourcesLoading, setIsSourcesLoading] = useState(false);
  const [sourceLoadError, setSourceLoadError] = useState<string | null>(null);
  const sourceLoadRequestId = useRef(0);

  const loadSources = useCallback(async () => {
    const requestId = sourceLoadRequestId.current + 1;
    sourceLoadRequestId.current = requestId;
    setIsSourcesLoading(true);
    setSourceLoadError(null);
    try {
      const nextSources = await listSources();
      if (sourceLoadRequestId.current !== requestId) return;
      const nextSourceIds = new Set(nextSources.map((source) => source.id));
      setSources(nextSources);
      setSelectedSourceIds((current) => current.filter((id) => nextSourceIds.has(id)));
      const checking = Object.fromEntries(nextSources.map((source) => [source.id, {state: "checking"} as HealthState]));
      setHealthBySource(checking);
      const healthEntries = await Promise.all(
        nextSources.map(async (source) => {
          try {
            const health = await getSourceHealth(source.id);
            return [source.id, {state: "ready", health} as HealthState] as const;
          } catch (err) {
            return [
              source.id,
              {
                state: "error",
                message: err instanceof Error ? err.message : "Health check failed",
              } as HealthState,
            ] as const;
          }
        }),
      );
      if (sourceLoadRequestId.current !== requestId) return;
      setHealthBySource(Object.fromEntries(healthEntries));
    } catch (err) {
      if (sourceLoadRequestId.current !== requestId) return;
      setSources([]);
      setHealthBySource({});
      setSelectedSourceIds([]);
      setSourceLoadError(err instanceof Error ? err.message : "Failed to load sources");
      showErrorToast(err);
    } finally {
      if (sourceLoadRequestId.current === requestId) {
        setIsSourcesLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (!isOpen || creds) return;
    void loadSources();
  }, [creds, isOpen, loadSources]);

  const hasSources = sources.length > 0;
  const helperText = useMemo(() => {
    if (isSourcesLoading) return "Loading sources...";
    if (sourceLoadError) return "Sources could not be loaded. Retry to try again.";
    if (!hasSources) return "Create at least one source before creating a bucket.";
    return "Choose which sources this bucket is allowed to use for uploads. Each source shows its latest health signal before you create the bucket.";
  }, [hasSources, isSourcesLoading, sourceLoadError]);

  const selectedSourceNames = useMemo(
    () => sources.filter((s) => selectedSourceIds.includes(s.id)).map((s) => s.name),
    [sources, selectedSourceIds],
  );

  const selectedHealthSummary = useMemo(() => {
    const states = selectedSourceIds.map((id) => healthBySource[id]).filter(Boolean);
    const checkingCount = states.filter((state) => state.state === "checking").length;
    const errorCount = states.filter((state) => state.state === "error").length;
    const readyStates = states.filter((state): state is Extract<HealthState, {state: "ready"}> => state.state === "ready");
    const unhealthyCount = readyStates.filter((state) => state.health.status === "unhealthy").length;
    const nearCapacityCount = readyStates.filter((state) => state.health.reason_code === "quota_low").length;
    const degradedCount = readyStates.filter((state) => state.health.status === "degraded" && state.health.reason_code !== "quota_low").length;
    const healthyCount = readyStates.filter((state) => state.health.status === "healthy").length;
    return {
      checkingCount,
      errorCount,
      unhealthyCount,
      nearCapacityCount,
      degradedCount,
      healthyCount,
    };
  }, [healthBySource, selectedSourceIds]);

  const canCreate = key.trim() !== "" && selectedSourceIds.length > 0 && selectedHealthSummary.checkingCount === 0;

  const selectionStatus = useMemo(() => {
    if (selectedSourceIds.length === 0) return null;
    if (selectedHealthSummary.checkingCount > 0) {
      return {
        tone: "default" as const,
        className: "border-default-200 bg-default-50 text-default-600",
        message:
          selectedHealthSummary.checkingCount === 1
            ? "Checking the selected source before bucket creation."
            : `Checking ${selectedHealthSummary.checkingCount} selected sources before bucket creation.`,
      };
    }
    if (selectedHealthSummary.unhealthyCount > 0) {
      const parts = [
        selectedHealthSummary.unhealthyCount === 1
          ? "1 selected source is unhealthy"
          : `${selectedHealthSummary.unhealthyCount} selected sources are unhealthy`,
      ];
      if (selectedHealthSummary.nearCapacityCount > 0) {
        parts.push(
          selectedHealthSummary.nearCapacityCount === 1
            ? "1 source is near capacity"
            : `${selectedHealthSummary.nearCapacityCount} sources are near capacity`,
        );
      }
      if (selectedHealthSummary.degradedCount > 0) {
        parts.push(
          selectedHealthSummary.degradedCount === 1
            ? "1 source is degraded"
            : `${selectedHealthSummary.degradedCount} sources are degraded`,
        );
      }
      return {
        tone: "danger" as const,
        className: "border-danger-200 bg-danger-50 text-danger-700",
        message: `${parts.join(", ")}. SFree can create the bucket anyway, but uploads or later reads may fail while those providers remain impaired.`,
      };
    }
    if (selectedHealthSummary.nearCapacityCount > 0 || selectedHealthSummary.degradedCount > 0 || selectedHealthSummary.errorCount > 0) {
      const parts: string[] = [];
      if (selectedHealthSummary.nearCapacityCount > 0) {
        parts.push(
          selectedHealthSummary.nearCapacityCount === 1
            ? "1 selected source is near capacity"
            : `${selectedHealthSummary.nearCapacityCount} selected sources are near capacity`,
        );
      }
      if (selectedHealthSummary.degradedCount > 0) {
        parts.push(
          selectedHealthSummary.degradedCount === 1
            ? "1 selected source is degraded"
            : `${selectedHealthSummary.degradedCount} selected sources are degraded`,
        );
      }
      if (selectedHealthSummary.errorCount > 0) {
        parts.push(
          selectedHealthSummary.errorCount === 1
            ? "1 selected source could not be checked"
            : `${selectedHealthSummary.errorCount} selected sources could not be checked`,
        );
      }
      return {
        tone: "warning" as const,
        className: "border-warning-200 bg-warning-50 text-warning-700",
        message: `${parts.join(", ")}. Bucket creation is still allowed, but the current source signal is not fully healthy.`,
      };
    }
    return {
      tone: "success" as const,
      className: "border-success-200 bg-success-50 text-success-700",
      message: "Selected sources currently report healthy reachability. SFree still does not replicate chunks automatically across all sources.",
    };
  }, [selectedHealthSummary, selectedSourceIds.length]);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          sourceLoadRequestId.current += 1;
          setKey("");
          setSources([]);
          setHealthBySource({});
          setSelectedSourceIds([]);
          setCreds(null);
          setIsSourcesLoading(false);
          setSourceLoadError(null);
        }
        onOpenChange(open);
      }}
      size="lg"
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Create Bucket</ModalHeader>
            <ModalBody>
              {creds ? (
                <CredentialsView accessKey={creds.accessKey} accessSecret={creds.accessSecret} />
              ) : (
                <div className="flex flex-col gap-5">
                  <p className="text-sm text-default-500">
                    A bucket gives you S3-compatible access to your files. Name it, pick the sources it can draw from, and you&apos;ll get credentials to connect any S3 client.
                  </p>

                  <div className="flex flex-col gap-2">
                    <p className="text-sm font-medium">Bucket name</p>
                    <Input label="Key" value={key} onChange={(e) => setKey(e.target.value)} />
                  </div>

                  <Divider />

                  <div className="flex flex-col gap-2">
                    <p className="text-sm font-medium">Allowed sources</p>
                    <p className={`text-sm ${sourceLoadError ? "text-danger" : "text-default-500"}`}>{helperText}</p>
                    {sourceLoadError ? (
                      <div className="flex flex-col items-start gap-2">
                        <p className="text-sm text-danger">{sourceLoadError}</p>
                        <Button size="sm" variant="flat" color="danger" isLoading={isSourcesLoading} onPress={() => void loadSources()}>
                          Retry
                        </Button>
                      </div>
                    ) : null}
                    {isSourcesLoading && !sourceLoadError ? (
                      <div className="flex items-center justify-center py-4">
                        <Spinner size="sm" />
                      </div>
                    ) : null}
                    {hasSources ? (
                      <>
                        <CheckboxGroup
                          value={selectedSourceIds}
                          onValueChange={(values) => setSelectedSourceIds(values as string[])}
                          classNames={{wrapper: "gap-3"}}
                        >
                          {sources.map((source) => (
                            <Checkbox key={source.id} value={source.id} classNames={{base: "max-w-full"}}>
                              <div className="flex flex-col gap-1 py-1">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span>{source.name}</span>
                                  <SourceTypeChip type={source.type} />
                                  <SourceHealthChip state={healthBySource[source.id]} />
                                </div>
                                <SourceHealthDetail state={healthBySource[source.id]} />
                              </div>
                            </Checkbox>
                          ))}
                        </CheckboxGroup>
                        {selectionStatus ? (
                          <div className={`rounded-lg border p-3 ${selectionStatus.className}`}>
                            <p className="text-sm">{selectionStatus.message}</p>
                          </div>
                        ) : null}
                      </>
                    ) : null}
                  </div>

                  {canCreate ? (
                    <>
                      <Divider />
                      <div className="flex flex-col gap-2 rounded-lg bg-default-50 p-3">
                        <p className="text-sm font-medium">Review</p>
                        <div className="text-sm text-default-600 flex flex-col gap-1">
                          <p>Bucket: <span className="font-mono font-medium">{key.trim()}</span></p>
                          <p>Sources: {selectedSourceNames.join(", ")}</p>
                        </div>
                      </div>
                    </>
                  ) : null}
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              {creds ? (
                <Button color="primary" onPress={onClose}>
                  Close
                </Button>
              ) : (
                <Button
                  color="primary"
                  isDisabled={!canCreate || isSourcesLoading}
                  isLoading={isLoading}
                  onPress={async () => {
                    setIsLoading(true);
                    try {
                      const trimmedKey = key.trim();
                      const res = await createBucket(trimmedKey, selectedSourceIds);
                      setCreds({accessKey: res.access_key, accessSecret: res.access_secret});
                      addToast({title: "Bucket created", description: `${trimmedKey} is ready`, color: "success", timeout: 4000});
                      onCreated();
                    } catch (err) {
                      showErrorToast(err);
                    } finally {
                      setIsLoading(false);
                    }
                  }}
                >
                  Create
                </Button>
              )}
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}

function SourceHealthChip({state}: {state: HealthState | undefined}) {
  if (!state || state.state === "checking") {
    return <Chip size="sm" variant="flat" color="default">Checking</Chip>;
  }
  if (state.state === "error") {
    return <Chip size="sm" variant="flat" color="danger">Check failed</Chip>;
  }
  return (
    <Chip size="sm" variant="flat" color={sourceHealthColor[state.health.status]}>
      {state.health.status}
    </Chip>
  );
}

function SourceHealthDetail({state}: {state: HealthState | undefined}) {
  if (!state || state.state === "checking") {
    return <p className="text-xs text-default-400">Checking source health and capacity signal...</p>;
  }
  if (state.state === "error") {
    return <p className="text-xs text-danger">{state.message}</p>;
  }

  const quota = getSourceQuotaState(state.health);
  const detailParts = [state.health.message];
  if (quota.kind === "available") {
    detailParts.push(`${formatSize(quota.freeBytes)} free of ${formatSize(quota.totalBytes)}`);
  } else if (state.health.type === "s3" || state.health.type === "telegram") {
    detailParts.push(quota.description);
  }

  return <p className="text-xs text-default-500">{detailParts.join(" ")}</p>;
}

function CredentialsView({accessKey, accessSecret}: {accessKey: string; accessSecret: string}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-lg border border-warning-200 bg-warning-50 p-3">
        <p className="text-sm font-medium text-warning-700">
          Make sure to copy these credentials now. You won&apos;t be able to see them again.
        </p>
      </div>
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-xs text-default-500 font-medium">Access Key</p>
          <Snippet hideSymbol variant="flat" size="sm">{accessKey}</Snippet>
        </div>
        <div className="flex flex-col gap-1">
          <p className="text-xs text-default-500 font-medium">Secret Key</p>
          <Snippet hideSymbol variant="flat" size="sm">{accessSecret}</Snippet>
        </div>
      </div>
      <p className="text-sm text-default-500">
        Use these credentials with any S3-compatible client to access your bucket.
      </p>
    </div>
  );
}
