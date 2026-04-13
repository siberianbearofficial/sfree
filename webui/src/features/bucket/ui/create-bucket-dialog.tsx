import {Button, Checkbox, CheckboxGroup, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useEffect, useMemo, useState} from "react";
import {createBucket} from "../../../shared/api/buckets";
import {listSources} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateBucketDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [key, setKey] = useState("");
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedSourceIds, setSelectedSourceIds] = useState<string[]>([]);
  const [creds, setCreds] = useState<{accessKey: string; accessSecret: string} | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSourcesLoading, setIsSourcesLoading] = useState(false);

  useEffect(() => {
    if (!isOpen || creds) return;

    let isActive = true;

    async function loadSources() {
      setIsSourcesLoading(true);
      try {
        const nextSources = await listSources();
        if (!isActive) return;
        setSources(nextSources);
      } finally {
        if (isActive) {
          setIsSourcesLoading(false);
        }
      }
    }

    loadSources();

    return () => {
      isActive = false;
    };
  }, [creds, isOpen]);

  const hasSources = sources.length > 0;
  const canCreate = key.trim() !== "" && selectedSourceIds.length > 0;
  const helperText = useMemo(() => {
    if (isSourcesLoading) return "Loading sources...";
    if (!hasSources) return "Create at least one source before creating a bucket.";
    return "Choose which sources this bucket is allowed to use for uploads.";
  }, [hasSources, isSourcesLoading]);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          setKey("");
          setSources([]);
          setSelectedSourceIds([]);
          setCreds(null);
          setIsSourcesLoading(false);
        }
        onOpenChange(open);
      }}
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Create Bucket</ModalHeader>
            <ModalBody>
              {creds ? (
                <>
                  <Snippet hideSymbol>{creds.accessKey}</Snippet>
                  <Snippet hideSymbol>{creds.accessSecret}</Snippet>
                  <p className="text-sm text-default-500">
                    Make sure to copy these credentials now. You won't be able to see them again.
                  </p>
                </>
              ) : (
                <>
                  <Input label="Key" value={key} onChange={(e) => setKey(e.target.value)} />
                  <div className="flex flex-col gap-2">
                    <p className="text-sm font-medium">Allowed sources</p>
                    <p className="text-sm text-default-500">{helperText}</p>
                    {hasSources ? (
                      <CheckboxGroup
                        value={selectedSourceIds}
                        onValueChange={(values) => setSelectedSourceIds(values as string[])}
                      >
                        {sources.map((source) => (
                          <Checkbox key={source.id} value={source.id}>
                            {source.name}
                          </Checkbox>
                        ))}
                      </CheckboxGroup>
                    ) : null}
                  </div>
                </>
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
