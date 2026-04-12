import {Avatar, Chip} from "@heroui/react";
import {GoogleDriveIcon, TelegramIcon, S3Icon} from "../../../shared/icons";

const types: Record<string, {label: string; icon: React.ReactNode}> = {
  gdrive: {label: "Google Drive", icon: <GoogleDriveIcon />},
  telegram: {label: "Telegram", icon: <TelegramIcon />},
  s3: {label: "S3-Compatible", icon: <S3Icon />},
};

export function SourceTypeChip({type}: {type: string}) {
  const info = types[type];
  if (info) {
    return (
      <Chip
        size="sm"
        avatar={<Avatar className="w-4 h-4" radius="full" icon={info.icon} />}
      >
        {info.label}
      </Chip>
    );
  }
  return <Chip size="sm">{type}</Chip>;
}
