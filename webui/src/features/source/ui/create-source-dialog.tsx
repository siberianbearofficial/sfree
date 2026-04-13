import {Button, Checkbox, Input, Modal, ModalBody, ModalContent, ModalFooter, ModalHeader, Select, SelectItem, Textarea} from "@heroui/react";
import {addToast} from "@heroui/toast";
import {useState} from "react";
import {createGDriveSource, createTelegramSource, createS3Source} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";

type SourceType = "gdrive" | "telegram" | "s3";

type Props = {isOpen: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void};

export function CreateSourceDialog({isOpen, onOpenChange, onCreated}: Props) {
  const [sourceType, setSourceType] = useState<SourceType>("gdrive");
  const [name, setName] = useState("");
  // gdrive
  const [key, setKey] = useState("");
  // telegram
  const [token, setToken] = useState("");
  const [chatId, setChatId] = useState("");
  // s3
  const [endpoint, setEndpoint] = useState("");
  const [bucket, setBucket] = useState("");
  const [accessKeyId, setAccessKeyId] = useState("");
  const [secretAccessKey, setSecretAccessKey] = useState("");
  const [region, setRegion] = useState("");
  const [pathStyle, setPathStyle] = useState(false);

  const [isLoading, setIsLoading] = useState(false);

  const isValid = (() => {
    if (!name.trim()) return false;
    if (sourceType === "gdrive") return !!key.trim();
    if (sourceType === "telegram") return !!token.trim() && !!chatId.trim();
    return !!endpoint.trim() && !!bucket.trim() && !!accessKeyId.trim() && !!secretAccessKey.trim();
  })();

  function reset() {
    setSourceType("gdrive");
    setName("");
    setKey("");
    setToken("");
    setChatId("");
    setEndpoint("");
    setBucket("");
    setAccessKeyId("");
    setSecretAccessKey("");
    setRegion("");
    setPathStyle(false);
  }

  async function handleCreate(onClose: () => void) {
    setIsLoading(true);
    try {
      if (sourceType === "gdrive") {
        await createGDriveSource(name, key);
      } else if (sourceType === "telegram") {
        await createTelegramSource(name, token, chatId);
      } else {
        await createS3Source({name, endpoint, bucket, access_key_id: accessKeyId, secret_access_key: secretAccessKey, region: region || undefined, path_style: pathStyle || undefined});
      }
      addToast({title: "Source created", description: `${name} is ready`, color: "success", timeout: 4000});
      onCreated();
      onClose();
    } catch (err) {
      showErrorToast(err);
    } finally {
      setIsLoading(false);
    }
  }

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
            <ModalHeader>Create Source</ModalHeader>
            <ModalBody>
              <Select
                label="Type"
                selectedKeys={[sourceType]}
                onSelectionChange={(keys) => {
                  const val = [...keys][0] as SourceType;
                  if (val) setSourceType(val);
                }}
              >
                <SelectItem key="gdrive">Google Drive</SelectItem>
                <SelectItem key="telegram">Telegram</SelectItem>
                <SelectItem key="s3">S3-Compatible</SelectItem>
              </Select>
              <Input label="Name" isRequired value={name} onChange={(e) => setName(e.target.value)} />
              {sourceType === "gdrive" && (
                <Textarea label="Service Account Key (JSON)" isRequired value={key} onChange={(e) => setKey(e.target.value)} />
              )}
              {sourceType === "telegram" && (
                <>
                  <Input label="Bot Token" isRequired value={token} onChange={(e) => setToken(e.target.value)} />
                  <Input label="Chat ID" isRequired value={chatId} onChange={(e) => setChatId(e.target.value)} />
                </>
              )}
              {sourceType === "s3" && (
                <>
                  <Input label="Endpoint" isRequired placeholder="https://s3.amazonaws.com" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
                  <Input label="Bucket" isRequired value={bucket} onChange={(e) => setBucket(e.target.value)} />
                  <Input label="Access Key ID" isRequired value={accessKeyId} onChange={(e) => setAccessKeyId(e.target.value)} />
                  <Input label="Secret Access Key" isRequired type="password" value={secretAccessKey} onChange={(e) => setSecretAccessKey(e.target.value)} />
                  <Input label="Region" placeholder="us-east-1 (optional)" value={region} onChange={(e) => setRegion(e.target.value)} />
                  <Checkbox isSelected={pathStyle} onValueChange={setPathStyle}>Path-style access</Checkbox>
                </>
              )}
            </ModalBody>
            <ModalFooter>
              <Button color="primary" isDisabled={!isValid} isLoading={isLoading} onPress={() => handleCreate(onClose)}>
                Create
              </Button>
            </ModalFooter>
          </>
        )}
      </ModalContent>
    </Modal>
  );
}
