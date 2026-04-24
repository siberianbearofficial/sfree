import {
  Button,
  Divider,
  Input,
  Link,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
} from "@heroui/react";
import {useState} from "react";
import {saveAuth} from "../../../shared/lib/auth";
import {apiUrl} from "../../../shared/api/client";
import {GitHubIcon} from "../../../shared/icons";

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onSwitchToRegister?: () => void;
};

export function LoginDialog({isOpen, onOpenChange, onSwitchToRegister}: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function reset() {
    setUsername("");
    setPassword("");
    setError(null);
  }

  const canSubmit = username.trim().length > 0 && password.length > 0;

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) reset();
        onOpenChange(open);
      }}
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader className="flex flex-col gap-1">
              Log In
              <span className="text-sm font-normal text-default-500">
                Sign in to your SFree account
              </span>
            </ModalHeader>
            <ModalBody>
              <Button
                variant="bordered"
                as="a"
                href={apiUrl("/auth/github")}
                startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
                className="w-full"
              >
                Continue with GitHub
              </Button>
              <div className="flex items-center gap-3 my-1">
                <Divider className="flex-1" />
                <span className="text-xs text-default-400">or</span>
                <Divider className="flex-1" />
              </div>
              <Input
                label="Username"
                value={username}
                isInvalid={!!error}
                onChange={(e) => {
                  setUsername(e.target.value);
                  setError(null);
                }}
                autoFocus
              />
              <Input
                label="Password"
                type="password"
                value={password}
                isInvalid={!!error}
                errorMessage={error}
                onChange={(e) => {
                  setPassword(e.target.value);
                  setError(null);
                }}
              />
            </ModalBody>
            <ModalFooter className="flex flex-col gap-3">
              <Button
                color="primary"
                className="w-full"
                isLoading={isLoading}
                isDisabled={!canSubmit}
                onPress={async () => {
                  setIsLoading(true);
                  setError(null);
                  try {
                    saveAuth(username.trim(), password);
                    onClose();
                  } catch {
                    setError("Unable to log in. Check your credentials and try again.");
                  } finally {
                    setIsLoading(false);
                  }
                }}
              >
                Log In
              </Button>
              {onSwitchToRegister && (
                <p className="text-sm text-center text-default-500">
                  No account yet?{" "}
                  <Link size="sm" className="cursor-pointer" onPress={onSwitchToRegister}>
                    Sign up
                  </Link>
                </p>
              )}
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
