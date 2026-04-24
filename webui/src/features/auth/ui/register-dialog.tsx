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
  Snippet,
} from "@heroui/react";
import {useEffect, useState} from "react";
import {createUser} from "../../../shared/api/users";
import {saveAuth} from "../../../shared/lib/auth";
import {showErrorToast} from "../../../shared/api/error";
import {apiUrl} from "../../../shared/api/client";
import {GitHubIcon} from "../../../shared/icons";

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onSwitchToLogin?: () => void;
};

export function RegisterDialog({isOpen, onOpenChange, onSwitchToLogin}: Props) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function reset() {
    setUsername("");
    setPassword(null);
    setError(null);
  }

  useEffect(() => {
    if (!isOpen) reset();
  }, [isOpen]);

  const canSubmit = username.trim().length > 0;

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange}>
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader className="flex flex-col gap-1">
              {password ? "Account Created" : "Sign Up"}
              <span className="text-sm font-normal text-default-500">
                {password
                  ? "Save your password before closing this dialog"
                  : "Create a free SFree account"}
              </span>
            </ModalHeader>
            <ModalBody>
              {password ? (
                <div className="flex flex-col gap-3">
                  <p className="text-sm text-default-600">
                    Your auto-generated password is shown below. Copy it now — it will not
                    be shown again.
                  </p>
                  <Snippet hideSymbol className="w-full">
                    {password}
                  </Snippet>
                </div>
              ) : (
                <>
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
                    errorMessage={error}
                    onChange={(e) => {
                      setUsername(e.target.value);
                      setError(null);
                    }}
                    autoFocus
                  />
                </>
              )}
            </ModalBody>
            <ModalFooter className="flex flex-col gap-3">
              {password ? (
                <Button color="primary" className="w-full" onPress={onClose}>
                  I saved my password
                </Button>
              ) : (
                <>
                  <Button
                    color="primary"
                    className="w-full"
                    isLoading={isLoading}
                    isDisabled={!canSubmit}
                    onPress={async () => {
                      setIsLoading(true);
                      setError(null);
                      try {
                        const result = await createUser(username.trim());
                        saveAuth(username.trim(), result.password);
                        setPassword(result.password);
                      } catch (err) {
                        showErrorToast(err);
                        setError("Could not create account. The username may already be taken.");
                      } finally {
                        setIsLoading(false);
                      }
                    }}
                  >
                    Create Account
                  </Button>
                  {onSwitchToLogin && (
                    <p className="text-sm text-center text-default-500">
                      Already have an account?{" "}
                      <Link size="sm" className="cursor-pointer" onPress={onSwitchToLogin}>
                        Log in
                      </Link>
                    </p>
                  )}
                </>
              )}
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
