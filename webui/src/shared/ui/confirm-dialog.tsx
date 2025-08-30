import {Button, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader} from "@heroui/react";

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  message?: string;
  onConfirm: () => void;
  confirmLabel?: string;
  isConfirmLoading?: boolean;
};

export function ConfirmDialog({
  isOpen,
  onOpenChange,
  title,
  message,
  onConfirm,
  confirmLabel = "Confirm",
  isConfirmLoading,
}: Props) {
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>{title}</ModalHeader>
            {message && <ModalBody>{message}</ModalBody>}
            <ModalFooter>
              <Button variant="light" onPress={onClose}>
                Cancel
              </Button>
              <Button color="danger" isLoading={isConfirmLoading} onPress={onConfirm}>
                {confirmLabel}
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
