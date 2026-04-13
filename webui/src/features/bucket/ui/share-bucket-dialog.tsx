import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Select,
  SelectItem,
} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useState, useEffect} from "react";
import {
  createGrant,
  deleteGrant,
  listGrants,
  updateGrant,
} from "../../../shared/api/grants";
import type {BucketGrant} from "../../../shared/api/grants";
import {DeleteIcon} from "@heroui/shared-icons";

const ROLES = [
  {key: "viewer", label: "Viewer"},
  {key: "editor", label: "Editor"},
  {key: "owner", label: "Owner"},
] as const;

type Props = {
  isOpen: boolean;
  onOpenChange: () => void;
  bucketId: string;
};

export function ShareBucketDialog({isOpen, onOpenChange, bucketId}: Props) {
  const [grants, setGrants] = useState<BucketGrant[]>([]);
  const [username, setUsername] = useState("");
  const [role, setRole] = useState<"owner" | "editor" | "viewer">("viewer");
  const [isAdding, setIsAdding] = useState(false);

  useEffect(() => {
    if (isOpen) {
      listGrants(bucketId)
        .then(setGrants)
        .catch(() => setGrants([]));
    }
  }, [isOpen, bucketId]);

  async function handleAdd() {
    if (!username.trim()) return;
    setIsAdding(true);
    try {
      const grant = await createGrant(bucketId, username.trim(), role);
      setGrants((prev) => [...prev, grant]);
      setUsername("");
      addToast({
        title: "Access granted",
        description: `${username} can now access this bucket as ${role}`,
        color: "success",
        timeout: 4000,
      });
    } catch (err) {
      addToast({
        title: "Failed to grant access",
        description: err instanceof Error ? err.message : "Unknown error",
        color: "danger",
        timeout: 4000,
      });
    } finally {
      setIsAdding(false);
    }
  }

  async function handleRoleChange(grantId: string, newRole: string) {
    try {
      await updateGrant(bucketId, grantId, newRole as BucketGrant["role"]);
      setGrants((prev) =>
        prev.map((g) =>
          g.id === grantId ? {...g, role: newRole as BucketGrant["role"]} : g,
        ),
      );
    } catch (err) {
      addToast({
        title: "Failed to update role",
        description: err instanceof Error ? err.message : "Unknown error",
        color: "danger",
        timeout: 4000,
      });
    }
  }

  async function handleRevoke(grantId: string) {
    try {
      await deleteGrant(bucketId, grantId);
      setGrants((prev) => prev.filter((g) => g.id !== grantId));
      addToast({title: "Access revoked", color: "success", timeout: 4000});
    } catch (err) {
      addToast({
        title: "Failed to revoke access",
        description: err instanceof Error ? err.message : "Unknown error",
        color: "danger",
        timeout: 4000,
      });
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          setGrants([]);
          setUsername("");
        }
        onOpenChange();
      }}
      size="lg"
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Share Bucket</ModalHeader>
            <ModalBody>
              <div className="flex gap-2 items-end">
                <Input
                  label="Username"
                  placeholder="Enter username"
                  value={username}
                  onValueChange={setUsername}
                  className="flex-1"
                />
                <Select
                  label="Role"
                  selectedKeys={new Set([role])}
                  onSelectionChange={(keys) => {
                    const val = Array.from(keys)[0] as string;
                    if (val) setRole(val as BucketGrant["role"]);
                  }}
                  className="w-32"
                >
                  {ROLES.map((r) => (
                    <SelectItem key={r.key}>{r.label}</SelectItem>
                  ))}
                </Select>
                <Button
                  color="primary"
                  isLoading={isAdding}
                  onPress={handleAdd}
                  isDisabled={!username.trim()}
                >
                  Add
                </Button>
              </div>

              {grants.length > 0 && (
                <div className="flex flex-col gap-2 mt-4">
                  <p className="text-sm font-semibold text-default-500">
                    People with access
                  </p>
                  {grants.map((g) => (
                    <div
                      key={g.id}
                      className="flex items-center gap-2 border rounded p-2"
                    >
                      <span className="flex-1 text-sm font-medium">
                        {g.username}
                      </span>
                      <Select
                        size="sm"
                        selectedKeys={new Set([g.role])}
                        onSelectionChange={(keys) => {
                          const val = Array.from(keys)[0] as string;
                          if (val && val !== g.role)
                            handleRoleChange(g.id, val);
                        }}
                        className="w-28"
                        aria-label="Role"
                      >
                        {ROLES.map((r) => (
                          <SelectItem key={r.key}>{r.label}</SelectItem>
                        ))}
                      </Select>
                      <Button
                        isIconOnly
                        size="sm"
                        variant="light"
                        color="danger"
                        onPress={() => handleRevoke(g.id)}
                      >
                        <DeleteIcon className="w-4 h-4" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </ModalBody>
            <ModalFooter>
              <Button variant="light" onPress={onClose}>
                Close
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
