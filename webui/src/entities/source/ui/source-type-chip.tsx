import {Avatar, Chip} from "@heroui/react";
import {GoogleDriveIcon} from "../../../shared/icons";

export function SourceTypeChip({type}: {type: string}) {
  if (type === "gdrive") {
    return (
      <Chip
        size="sm"
        avatar={
          <Avatar className="w-4 h-4" radius="full" icon={<GoogleDriveIcon />} />
        }
      >
        Google Drive
      </Chip>
    );
  }
  return <Chip size="sm">{type}</Chip>;
}
