import {Button, Checkbox, CheckboxGroup, Divider, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet, Spinner} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {createBucket} from "../../../shared/api/buckets";
import {listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";
import {SourceTypeChip} from "../../../entities/source";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateBucketDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [key, setKey] = useState("");
  const [sources, setSources] = useState<Source[]>([]);
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

  useEffect(() => {
    if (!isOpen || creds) return;
    void loadSources();
  }, [creds, isOpen, loadSources]);

  const hasSources = sources.length > 0;
  const canCreate = key.trim() !== "" && selectedSourceIds.length > 0;
  const helperText = useMemo(() => {
    if (isSourcesLoading) return "Loading sources...";
    if (sourceLoadError) return "Sources could not be loaded. Retry to try again.";
    if (!hasSources) return "Create at least one source before creating a bucket.";
    return "Choose which sources this bucket is allowed to use for uploads.";
  }, [hasSources, isSourcesLoading, sourceLoadError]);

  const selectedSourceNames = useMemo(
    () => sources.filter((s) => selectedSourceIds.includes(s.id)).map((s) => s.name),
    [sources, selectedSourceIds],
  );

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          sourceLoadRequestId.current += 1;
          setKey("");
          setSources([]);
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
                      <CheckboxGroup
                        value={selectedSourceIds}
                        onValueChange={(values) => setSelectedSourceIds(values as string[])}
                        classNames={{wrapper: "gap-3"}}
                      >
                        {sources.map((source) => (
                          <Checkbox key={source.id} value={source.id} classNames={{base: "max-w-full"}}>
                            <div className="flex items-center gap-2">
                              <span>{source.name}</span>
                              <SourceTypeChip type={source.type} />
                            </div>
                          </Checkbox>
                        ))}
                      </CheckboxGroup>
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
                      const res = await createBucket(key, selectedSourceIds);
                      setCreds({accessKey: res.access_key, accessSecret: res.access_secret});
                      addToast({title: "Bucket created", description: `${key} is ready`, color: "success", timeout: 4000});
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
