import {
  Button,
  Card,
  CardBody,
  Checkbox,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Textarea,
} from "@heroui/react";
import {useState, useMemo} from "react";
import {
  createGDriveSource,
  createS3Source,
  createTelegramSource,
} from "../../../shared/api/sources";
import type {Source} from "../../../shared/api/sources";
import {showErrorToast} from "../../../shared/api/error";
import {GoogleDriveIcon, TelegramIcon, S3Icon} from "../../../shared/icons";

/* ---------- types ---------- */

type SourceType = "gdrive" | "telegram" | "s3";
type Step = "select" | "form" | "success";

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
  onNavigateToSource?: (id: string) => void;
};

/* ---------- provider metadata ---------- */

const providers: {
  key: SourceType;
  label: string;
  icon: React.ReactNode;
  description: string;
}[] = [
  {
    key: "gdrive",
    label: "Google Drive",
    icon: <GoogleDriveIcon className="w-6 h-6" />,
    description:
      "Connect a Google Drive folder via a service account. Files sync automatically.",
  },
  {
    key: "telegram",
    label: "Telegram",
    icon: <TelegramIcon className="w-6 h-6" />,
    description:
      "Receive files from a Telegram bot. Create a bot via @BotFather first.",
  },
  {
    key: "s3",
    label: "S3-Compatible",
    icon: <S3Icon className="w-6 h-6" />,
    description:
      "Connect any S3-compatible bucket (AWS, MinIO, Backblaze, etc.).",
  },
];

/* ---------- field-level helpers ---------- */

type FieldMeta = {
  label: string;
  description: string;
  placeholder?: string;
  type?: string;
  required?: boolean;
  multiline?: boolean;
};

const fieldsByProvider: Record<string, FieldMeta[]> = {
  gdrive: [
    {
      label: "Source Name",
      description: "A friendly name to identify this source in your dashboard.",
      placeholder: "e.g. Marketing Assets",
      required: true,
    },
    {
      label: "Service Account Key (JSON)",
      description:
        'Paste the full JSON key from Google Cloud Console. Go to IAM & Admin > Service Accounts > Keys > Add Key > JSON.',
      placeholder: '{"type": "service_account", "project_id": "...", ...}',
      required: true,
      multiline: true,
    },
  ],
  telegram: [
    {
      label: "Source Name",
      description: "A friendly name to identify this source in your dashboard.",
      placeholder: "e.g. Team File Bot",
      required: true,
    },
    {
      label: "Bot Token",
      description:
        "The API token from @BotFather. It looks like 123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11.",
      placeholder: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
      required: true,
    },
    {
      label: "Chat ID",
      description:
        "The chat where the bot receives files. Send a message to the bot, then use the getUpdates API to find your chat ID.",
      placeholder: "-1001234567890",
      required: true,
    },
  ],
  s3: [
    {
      label: "Source Name",
      description: "A friendly name to identify this source in your dashboard.",
      placeholder: "e.g. Production Backups",
      required: true,
    },
    {
      label: "Endpoint",
      description:
        "The S3-compatible endpoint URL. For AWS use https://s3.amazonaws.com. For MinIO use your server URL.",
      placeholder: "https://s3.amazonaws.com",
      required: true,
    },
    {
      label: "Bucket",
      description: "The bucket name to connect to. It must already exist.",
      placeholder: "my-bucket",
      required: true,
    },
    {
      label: "Access Key ID",
      description:
        "Your access key for authentication. For AWS, find this in IAM > Security Credentials.",
      placeholder: "AKIAIOSFODNN7EXAMPLE",
      required: true,
    },
    {
      label: "Secret Access Key",
      description: "Your secret key. This is stored securely and never shown again after creation.",
      placeholder: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
      required: true,
      type: "password",
    },
    {
      label: "Region",
      description: "The bucket region. Leave empty if your provider doesn't require it.",
      placeholder: "us-east-1",
    },
  ],
};

/* ---------- component ---------- */

export function CreateSourceDialog({
  isOpen,
  onOpenChange,
  onCreated,
  onNavigateToSource,
}: Props) {
  const [step, setStep] = useState<Step>("select");
  const [sourceType, setSourceType] = useState<SourceType>("gdrive");
  const [isLoading, setIsLoading] = useState(false);
  const [createdSource, setCreatedSource] = useState<Source | null>(null);

  // shared
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

  // track which fields have been touched for validation UX
  const [touched, setTouched] = useState<Set<string>>(new Set());

  function markTouched(field: string) {
    setTouched((prev) => new Set(prev).add(field));
  }

  const isValid = useMemo(() => {
    if (!name.trim()) return false;
    if (sourceType === "gdrive") return !!key.trim();
    if (sourceType === "telegram")
      return !!token.trim() && !!chatId.trim();
    return (
      !!endpoint.trim() &&
      !!bucket.trim() &&
      !!accessKeyId.trim() &&
      !!secretAccessKey.trim()
    );
  }, [name, sourceType, key, token, chatId, endpoint, bucket, accessKeyId, secretAccessKey]);

  function fieldError(field: string, value: string): string | undefined {
    if (!touched.has(field)) return undefined;
    if (!value.trim()) return "This field is required.";
    if (field === "Service Account Key (JSON)") {
      try {
        const parsed = JSON.parse(value);
        if (!parsed.type || !parsed.project_id)
          return "Key must contain \"type\" and \"project_id\" fields.";
      } catch {
        return "Invalid JSON. Paste the entire service account key file.";
      }
    }
    return undefined;
  }

  function reset() {
    setStep("select");
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
    setIsLoading(false);
    setCreatedSource(null);
    setTouched(new Set());
  }

  function handleSelectProvider(type: SourceType) {
    setSourceType(type);
    setStep("form");
  }

  function handleBack() {
    setStep("select");
    setTouched(new Set());
  }

  async function handleCreate() {
    setIsLoading(true);
    try {
      let source: Source;
      if (sourceType === "gdrive") {
        source = await createGDriveSource(name, key);
      } else if (sourceType === "telegram") {
        source = await createTelegramSource(name, token, chatId);
      } else {
        source = await createS3Source({
          name,
          endpoint,
          bucket,
          access_key_id: accessKeyId,
          secret_access_key: secretAccessKey,
          region: region || undefined,
          path_style: pathStyle || undefined,
        });
      }
      setCreatedSource(source);
      setStep("success");
      onCreated();
    } catch (err) {
      showErrorToast(err);
    } finally {
      setIsLoading(false);
    }
  }

  function handleClose(onClose: () => void) {
    reset();
    onClose();
  }

  /* ---- field value getters/setters by label ---- */

  function getFieldValue(label: string): string {
    switch (label) {
      case "Source Name": return name;
      case "Service Account Key (JSON)": return key;
      case "Bot Token": return token;
      case "Chat ID": return chatId;
      case "Endpoint": return endpoint;
      case "Bucket": return bucket;
      case "Access Key ID": return accessKeyId;
      case "Secret Access Key": return secretAccessKey;
      case "Region": return region;
      default: return "";
    }
  }

  function setFieldValue(label: string, value: string) {
    switch (label) {
      case "Source Name": setName(value); break;
      case "Service Account Key (JSON)": setKey(value); break;
      case "Bot Token": setToken(value); break;
      case "Chat ID": setChatId(value); break;
      case "Endpoint": setEndpoint(value); break;
      case "Bucket": setBucket(value); break;
      case "Access Key ID": setAccessKeyId(value); break;
      case "Secret Access Key": setSecretAccessKey(value); break;
      case "Region": setRegion(value); break;
    }
  }

  const providerInfo = providers.find((p) => p.key === sourceType)!;

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) reset();
        onOpenChange(open);
      }}
      size={step === "select" ? "2xl" : "lg"}
    >
      <ModalContent>
        {(onClose) => {
          /* ---- Step 1: Provider selection ---- */
          if (step === "select") {
            return (
              <>
                <ModalHeader className="flex flex-col gap-1">
                  <span>Connect a Source</span>
                  <span className="text-sm font-normal text-default-500">
                    Choose where your files live. You can add more sources later.
                  </span>
                </ModalHeader>
                <ModalBody>
                  <div className="grid gap-3 sm:grid-cols-3">
                    {providers.map((p) => (
                      <Card
                        key={p.key}
                        isPressable
                        onPress={() => handleSelectProvider(p.key)}
                        className="border-2 border-transparent hover:border-primary transition-colors"
                      >
                        <CardBody className="flex flex-col items-center text-center gap-3 p-4">
                          <div className="w-10 h-10 flex items-center justify-center rounded-full bg-default-100">
                            {p.icon}
                          </div>
                          <div>
                            <p className="font-semibold text-sm">{p.label}</p>
                            <p className="text-xs text-default-500 mt-1">
                              {p.description}
                            </p>
                          </div>
                        </CardBody>
                      </Card>
                    ))}
                  </div>
                </ModalBody>
                <ModalFooter>
                  <Button variant="flat" onPress={() => handleClose(onClose)}>
                    Cancel
                  </Button>
                </ModalFooter>
              </>
            );
          }

          /* ---- Step 3: Success ---- */
          if (step === "success" && createdSource) {
            return (
              <>
                <ModalHeader>Source Connected</ModalHeader>
                <ModalBody>
                  <div className="flex flex-col items-center text-center gap-4 py-4">
                    <div className="w-12 h-12 flex items-center justify-center rounded-full bg-success-100">
                      <CheckIcon className="w-6 h-6 text-success" />
                    </div>
                    <div>
                      <p className="text-lg font-semibold">
                        {createdSource.name} is ready
                      </p>
                      <p className="text-sm text-default-500 mt-1">
                        Your {providerInfo.label} source has been connected
                        successfully. You can now browse files or configure
                        additional settings.
                      </p>
                    </div>
                  </div>
                </ModalBody>
                <ModalFooter className="flex justify-between">
                  <Button
                    variant="flat"
                    onPress={() => handleClose(onClose)}
                  >
                    Close
                  </Button>
                  <Button
                    color="primary"
                    onPress={() => {
                      onNavigateToSource?.(createdSource.id);
                      handleClose(onClose);
                    }}
                  >
                    View Source
                  </Button>
                </ModalFooter>
              </>
            );
          }

          /* ---- Step 2: Provider-specific form ---- */
          const fields = fieldsByProvider[sourceType] ?? [];

          return (
            <>
              <ModalHeader className="flex flex-col gap-1">
                <div className="flex items-center gap-2">
                  <div className="w-6 h-6 flex items-center justify-center">
                    {providerInfo.icon}
                  </div>
                  <span>Connect {providerInfo.label}</span>
                </div>
                <span className="text-sm font-normal text-default-500">
                  {providerInfo.description}
                </span>
              </ModalHeader>
              <ModalBody className="flex flex-col gap-4">
                {fields.map((field) => {
                  const value = getFieldValue(field.label);
                  const error = field.required
                    ? fieldError(field.label, value)
                    : undefined;

                  if (field.multiline) {
                    return (
                      <div key={field.label} className="flex flex-col gap-1">
                        <Textarea
                          label={field.label}
                          isRequired={field.required}
                          value={value}
                          onChange={(e) =>
                            setFieldValue(field.label, e.target.value)
                          }
                          onBlur={() => markTouched(field.label)}
                          placeholder={field.placeholder}
                          isInvalid={!!error}
                          errorMessage={error}
                          minRows={3}
                          maxRows={6}
                        />
                        <p className="text-xs text-default-400 px-1">
                          {field.description}
                        </p>
                      </div>
                    );
                  }

                  return (
                    <div key={field.label} className="flex flex-col gap-1">
                      <Input
                        label={field.label}
                        isRequired={field.required}
                        type={field.type ?? "text"}
                        value={value}
                        onChange={(e) =>
                          setFieldValue(field.label, e.target.value)
                        }
                        onBlur={() => markTouched(field.label)}
                        placeholder={field.placeholder}
                        isInvalid={!!error}
                        errorMessage={error}
                      />
                      <p className="text-xs text-default-400 px-1">
                        {field.description}
                      </p>
                    </div>
                  );
                })}
                {sourceType === "s3" && (
                  <Checkbox isSelected={pathStyle} onValueChange={setPathStyle}>
                    <span className="text-sm">Path-style access</span>
                    <p className="text-xs text-default-400">
                      Enable if your provider requires path-style URLs instead of
                      virtual-hosted (e.g. MinIO).
                    </p>
                  </Checkbox>
                )}
              </ModalBody>
              <ModalFooter className="flex justify-between">
                <Button variant="flat" onPress={handleBack}>
                  Back
                </Button>
                <Button
                  color="primary"
                  isDisabled={!isValid}
                  isLoading={isLoading}
                  onPress={() => handleCreate()}
                >
                  Connect Source
                </Button>
              </ModalFooter>
            </>
          );
        }}
      </ModalContent>
    </Modal>
  );
}

/* ---------- inline check icon ---------- */

function CheckIcon(props: {className?: string}) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}
