import {Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Snippet} from "@heroui/react";
import {useState} from "react";
import {createUser} from "../../../shared/api/users";
import {saveAuth} from "../../../shared/lib/auth";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void};

export function RegisterDialog({isOpen, onOpenChange}: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState<string | null>(null);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          setUsername("");
          setPassword(null);
        }
        onOpenChange(open);
      }}
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Sign Up</ModalHeader>
            <ModalBody>
              <Input label="Username" value={username} onChange={(e) => setUsername(e.target.value)} />
              {password && (
                <Snippet hideSymbol>{password}</Snippet>
              )}
            </ModalBody>
            <ModalFooter>
              {password ? (
                <Button color="primary" onPress={onClose}>
                  Close
                </Button>
              ) : (
                <Button
                  color="primary"
                  onPress={async () => {
                    const {password} = await createUser(username);
                    saveAuth(username, password);
                    setPassword(password);
                  }}
                >
                  Sign Up
                </Button>
              )}
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
