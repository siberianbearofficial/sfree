import {Button, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader} from "@heroui/react";
import {useState} from "react";
import {createUser} from "../../../shared/api/users";
import {saveAuth} from "../../../shared/lib/auth";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void};

export function RegisterDialog({isOpen, onOpenChange}: Props) {
  const [username, setUsername] = useState("");
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Sign Up</ModalHeader>
            <ModalBody>
              <Input label="Username" value={username} onChange={(e) => setUsername(e.target.value)} />
            </ModalBody>
            <ModalFooter>
              <Button color="primary" onPress={async () => {const {password} = await createUser(username); saveAuth(username, password); onClose();}}>Sign Up</Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
