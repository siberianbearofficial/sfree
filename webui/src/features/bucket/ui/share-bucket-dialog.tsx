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
  Spinner,
} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useCallback, useState, useEffect, useRef} from "react";
import {
  createGrant,
  deleteGrant,
  listGrants,
  updateGrant,
} from "../../../shared/api/grants";
import type {BucketGrant} from "../../../shared/api/grants";
import {DeleteIcon} from "@heroui/shared-icons";
import {showErrorToast} from "../../../shared/api/error";
import {EmptyState} from "../../../shared/ui";

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
  const [isLoadingGrants, setIsLoadingGrants] = useState(false);
  const [grantLoadError, setGrantLoadError] = useState<string | null>(null);
  const grantLoadRequestRef = useRef(0);

  const loadGrants = useCallback(async () => {
    const requestId = grantLoadRequestRef.current + 1;
    grantLoadRequestRef.current = requestId;
    setIsLoadingGrants(true);
    setGrantLoadError(null);
    try {
      const nextGrants = await listGrants(bucketId);
      if (grantLoadRequestRef.current !== requestId) return;
      setGrants(nextGrants);
      setGrantLoadError(null);
    } catch (err) {
      if (grantLoadRequestRef.current !== requestId) return;
      setGrants([]);
      setGrantLoadError(
        err instanceof Error
          ? err.message
          : "Failed to load people with access.",
      );
      showErrorToast(err);
    } finally {
      if (grantLoadRequestRef.current === requestId) {
        setIsLoadingGrants(false);
      }
    }
  }, [bucketId]);

  useEffect(() => {
    if (isOpen) {
      loadGrants();
    }
  }, [isOpen, loadGrants]);

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
      showErrorToast(err);
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
      showErrorToast(err);
    }
  }

  async function handleRevoke(grantId: string) {
    try {
      await deleteGrant(bucketId, grantId);
      setGrants((prev) => prev.filter((g) => g.id !== grantId));
      addToast({title: "Access revoked", color: "success", timeout: 4000});
    } catch (err) {
      showErrorToast(err);
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          grantLoadRequestRef.current += 1;
          setGrants([]);
          setUsername("");
          setGrantLoadError(null);
          setIsLoadingGrants(false);
        }
        onOpenChange();
      }}
      size="lg"
      scrollBehavior="inside"
    >
      <ModalContent>
        {(onClose) => (
          <>
            <ModalHeader>Share Bucket</ModalHeader>
            <ModalBody>
              <div className="flex flex-col sm:flex-row gap-2 sm:items-end">
                <Input
                  label="Username"
                  placeholder="Enter username"
                  value={username}
                  onValueChange={setUsername}
                  className="flex-1"
                />
                <div className="flex gap-2 items-end">
                  <Select
                    label="Role"
                    selectedKeys={new Set([role])}
                    onSelectionChange={(keys) => {
                      const val = Array.from(keys)[0] as string;
                      if (val) setRole(val as BucketGrant["role"]);
                    }}
                    className="w-full sm:w-32"
                  >
                    {ROLES.map((r) => (
                      <SelectItem key={r.key}>{r.label}</SelectItem>
                    ))}
                  </Select>
                  <Button
                    color="primary"
                    isLoading={isAdding}
                    onPress={handleAdd}
                    isDisabled={
                      !username.trim() || isLoadingGrants || !!grantLoadError
                    }
                  >
                    Add
                  </Button>
                </div>
              </div>

              {isLoadingGrants && (
                <div className="flex items-center justify-center gap-2 py-8 text-default-500">
                  <Spinner size="sm" />
                  <span className="text-sm">Loading people with access</span>
                </div>
              )}

              {!isLoadingGrants && grantLoadError && (
                <EmptyState
                  title="Access list failed to load"
                  description={grantLoadError}
                  ctaLabel="Retry"
                  onCtaPress={loadGrants}
                  variant="danger"
                />
              )}

              {!isLoadingGrants && !grantLoadError && grants.length > 0 && (
                <div className="flex flex-col gap-2 mt-4">
                  <p className="text-sm font-semibold text-default-500">
                    People with access
                  </p>
                  {grants.map((g) => (
                    <div
                      key={g.id}
                      className="flex flex-wrap items-center gap-2 border rounded p-2"
                    >
                      <span className="flex-1 min-w-0 text-sm font-medium truncate">
                        {g.username}
                      </span>
                      <div className="flex items-center gap-2">
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
