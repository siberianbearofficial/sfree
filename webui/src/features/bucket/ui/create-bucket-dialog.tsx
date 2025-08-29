import {Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet} from "@heroui/react";
import {useState} from "react";
import {createBucket} from "../../../shared/api/buckets";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateBucketDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [key, setKey] = useState("");
  const [creds, setCreds] = useState<{accessKey: string; accessSecret: string} | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          setKey("");
          setCreds(null);
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
                <Input label="Key" value={key} onChange={(e) => setKey(e.target.value)} />
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
                  isLoading={isLoading}
                  onPress={async () => {
                    setIsLoading(true);
                    try {
                      const res = await createBucket(key);
                      setCreds({accessKey: res.access_key, accessSecret: res.access_secret});
                      onCreated();
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
