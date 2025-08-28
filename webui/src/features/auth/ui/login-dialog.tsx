import {Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader} from "@heroui/react";
import {useState} from "react";
import {saveAuth} from "../../../shared/lib/auth";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void};

export function LoginDialog({isOpen, onOpenChange}: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Log In</ModalHeader>
            <ModalBody>
              <Input label="Username" value={username} onChange={(e) => setUsername(e.target.value)} />
              <Input label="Password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </ModalBody>
            <ModalFooter>
              <Button
                color="primary"
                isLoading={isLoading}
                onPress={async () => {
                  setIsLoading(true);
                  try {
                    saveAuth(username, password);
                    onClose();
                  } finally {
                    setIsLoading(false);
                  }
                }}
              >
                Log In
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
