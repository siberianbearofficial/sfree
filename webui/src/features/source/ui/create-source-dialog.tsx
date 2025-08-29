import {Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Textarea} from "@heroui/react";
import {useState} from "react";
import {createSource} from "../../../shared/api/sources";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateSourceDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          setName("");
          setKey("");
        }
        onOpenChange(open);
      }}
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Create Source</ModalHeader>
            <ModalBody>
              <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} />
              <Textarea label="Key" value={key} onChange={(e) => setKey(e.target.value)} />
            </ModalBody>
            <ModalFooter>
              <Button
                color="primary"
                isLoading={isLoading}
                onPress={async () => {
                  setIsLoading(true);
                  try {
                    await createSource(name, key);
                    onCreated();
                    onClose();
                  } finally {
                    setIsLoading(false);
                  }
                }}
              >
                Create
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
