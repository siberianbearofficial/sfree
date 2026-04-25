import {Button, Checkbox, CheckboxGroup, Chip, Divider, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet, Spinner} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {createBucket, preflightBucket} from "../../../shared/api/buckets";
import type {BucketPreflight, BucketPreflightSource} from "../../../shared/api/buckets";
import {listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {ApiError, showErrorToast} from "../../../shared/api/error";
import {SourceTypeChip} from "../../../entities/source";
import {sourceHealthColor} from "../../../entities/source/lib/capacity";
import {formatSize} from "../../../shared/lib/format";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateBucketDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [key, setKey] = useState("");
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [preflight, setPreflight] = useState<BucketPreflight | null>(null);
  const [riskAcknowledged, setRiskAcknowledged] = useState(false);
  const [creds, setCreds] = useState<{accessKey: string; accessSecret: string} | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSourcesLoading, setIsSourcesLoading] = useState(false);
  const [isPreflightLoading, setIsPreflightLoading] = useState(false);
  const [sourceLoadError, setSourceLoadError] = useState<string | null>(null);
  const [preflightError, setPreflightError] = useState<string | null>(null);
  const sourceLoadRequestId = useRef(0);
  const preflightRequestId = useRef(0);

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
    } catch (err) {
      if (sourceLoadRequestId.current !== requestId) return;
      setSources([]);
      setSelectedSourceIds([]);
      setSourceLoadError(err instanceof Error ? err.message : "Failed to load sources");
      showErrorToast(err);
    } finally {
      if (sourceLoadRequestId.current === requestId) {
        setIsSourcesLoading(false);
      }
    }
  }, []);

  const loadPreflight = useCallback(async (sourceIds: string[]) => {
    const requestId = preflightRequestId.current + 1;
    preflightRequestId.current = requestId;
    setIsPreflightLoading(true);
    setPreflightError(null);
    try {
      const nextPreflight = await preflightBucket(sourceIds);
      if (preflightRequestId.current !== requestId) return;
      setPreflight(nextPreflight);
    } catch (err) {
      if (preflightRequestId.current !== requestId) return;
      setPreflight(null);
      setPreflightError(err instanceof Error ? err.message : "Failed to preflight bucket creation");
    } finally {
      if (preflightRequestId.current === requestId) {
        setIsPreflightLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (!isOpen || creds) return;
    void loadSources();
  }, [creds, isOpen, loadSources]);

  useEffect(() => {
    if (!isOpen || creds) return;
    setRiskAcknowledged(false);
    if (selectedSourceIds.length === 0) {
      preflightRequestId.current += 1;
      setIsPreflightLoading(false);
      setPreflight(null);
      setPreflightError(null);
      return;
    }
    void loadPreflight(selectedSourceIds);
  }, [creds, isOpen, loadPreflight, selectedSourceIds]);

  const hasSources = sources.length > 0;
  const helperText = useMemo(() => {
    if (isSourcesLoading) return "Loading sources...";
    if (sourceLoadError) return "Sources could not be loaded. Retry to try again.";
    if (!hasSources) return "Create at least one source before creating a bucket.";
    return "Choose which sources this bucket is allowed to use for uploads. SFree will preflight the selected sources before creation.";
  }, [hasSources, isSourcesLoading, sourceLoadError]);

  const selectedSourceNames = useMemo(
    () => sources.filter((s) => selectedSourceIds.includes(s.id)).map((s) => s.name),
    [sources, selectedSourceIds],
  );

  const canCreate = key.trim() !== ""
    && selectedSourceIds.length > 0
    && !isSourcesLoading
    && !isPreflightLoading
    && preflight !== null
    && preflightError === null
    && (preflight.decision === "ready" || (preflight.decision === "confirm_required" && riskAcknowledged));

  const preflightState = useMemo(() => {
    if (selectedSourceIds.length === 0) return null;
    if (isPreflightLoading) {
      return {
        className: "border-default-200 bg-default-50 text-default-600",
        message: "Checking the selected sources before bucket creation.",
      };
    }
    if (preflightError) {
      return {
        className: "border-danger-200 bg-danger-50 text-danger-700",
        message: preflightError,
      };
    }
    if (!preflight) return null;
    if (preflight.decision === "blocked") {
      return {
        className: "border-danger-200 bg-danger-50 text-danger-700",
        message: preflight.message,
      };
    }
    if (preflight.decision === "confirm_required") {
      return {
        className: "border-warning-200 bg-warning-50 text-warning-700",
        message: preflight.message,
      };
    }
    return {
      className: "border-success-200 bg-success-50 text-success-700",
      message: preflight.message,
    };
  }, [isPreflightLoading, preflight, preflightError, selectedSourceIds.length]);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          sourceLoadRequestId.current += 1;
          preflightRequestId.current += 1;
          setKey("");
          setSources([]);
          setSelectedSourceIds([]);
          setPreflight(null);
          setRiskAcknowledged(false);
          setCreds(null);
          setIsSourcesLoading(false);
          setIsPreflightLoading(false);
          setSourceLoadError(null);
          setPreflightError(null);
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
                              <div className="flex flex-wrap items-center gap-2 py-1">
                                <span>{source.name}</span>
                                <SourceTypeChip type={source.type} />
                              </div>
                            </Checkbox>
                          ))}
                        </CheckboxGroup>
                        {preflightState ? (
                          <div className={`rounded-lg border p-3 ${preflightState.className}`}>
                            <p className="text-sm">{preflightState.message}</p>
                          </div>
                        ) : null}
                        {preflightError && selectedSourceIds.length > 0 ? (
                          <Button size="sm" variant="flat" onPress={() => void loadPreflight(selectedSourceIds)}>
                            Retry preflight
                          </Button>
                        ) : null}
                        {preflight ? (
                          <PreflightDetails preflight={preflight} />
                        ) : null}
                        {preflight?.decision === "confirm_required" ? (
                          <Checkbox isSelected={riskAcknowledged} onValueChange={setRiskAcknowledged}>
                            I understand this bucket starts on degraded or near-capacity sources.
                          </Checkbox>
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
                  isDisabled={!canCreate}
                  isLoading={isLoading}
                  onPress={async () => {
                    setIsLoading(true);
                    try {
                      const trimmedKey = key.trim();
                      const res = await createBucket(trimmedKey, selectedSourceIds, riskAcknowledged);
                      setCreds({accessKey: res.access_key, accessSecret: res.access_secret});
                      addToast({title: "Bucket created", description: `${trimmedKey} is ready`, color: "success", timeout: 4000});
                      onCreated();
                    } catch (err) {
                      if (err instanceof ApiError && err.status === 409) {
                        void loadPreflight(selectedSourceIds);
                      }
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

function PreflightDetails({preflight}: {preflight: BucketPreflight}) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-default-200 bg-default-50 p-3">
      <p className="text-sm font-medium">Selected source preflight</p>
      <div className="flex flex-wrap gap-2 text-xs text-default-500">
        <span>{preflight.healthy_source_count} healthy</span>
        <span>{preflight.degraded_source_count} degraded</span>
        <span>{preflight.near_capacity_source_count} near capacity</span>
        <span>{preflight.unhealthy_source_count} unhealthy</span>
      </div>
      <div className="flex flex-col gap-2">
        {preflight.sources.map((source) => (
          <div key={source.source_id} className="rounded-md border border-default-200 bg-background px-3 py-2">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium text-sm">{source.source_name}</span>
              <SourceTypeChip type={source.source_type} />
              <Chip size="sm" variant="flat" color={sourceHealthColor[source.status]}>
                {source.status}
              </Chip>
              {source.blocks_creation ? <Chip size="sm" variant="flat" color="danger">blocked</Chip> : null}
              {!source.blocks_creation && source.requires_confirmation ? <Chip size="sm" variant="flat" color="warning">confirm</Chip> : null}
            </div>
            <p className="mt-1 text-xs text-default-500">{sourceDetail(source)}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function sourceDetail(source: BucketPreflightSource): string {
  const parts = [source.message];
  if (source.quota_total_bytes !== null && source.quota_free_bytes !== null) {
    parts.push(`${formatSize(Math.max(0, source.quota_free_bytes))} free of ${formatSize(source.quota_total_bytes)}`);
  } else if (source.source_type === "s3") {
    parts.push("S3-compatible sources are checked for reachability here, not provider-wide capacity limits.");
  } else if (source.source_type === "telegram") {
    parts.push("Telegram sources do not expose native quota metadata to SFree.");
  }
  return parts.join(" ");
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
