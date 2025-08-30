import {Button, CircularProgress} from "@heroui/react";
import {useEffect, useState} from "react";
import {useNavigate, useParams} from "react-router-dom";
import {downloadFile, getSourceInfo} from "../../../shared/api/sources";
import type {SourceInfo} from "../../../shared/api/sources";
import {SourceTypeChip} from "../../../entities/source";
import {DownloadIcon} from "../../../shared/icons";
import {ArrowLeftIcon} from "@heroui/shared-icons";

export function SourcePage() {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const [info, setInfo] = useState<SourceInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    setIsLoading(true);
    getSourceInfo(id)
      .then(setInfo)
      .finally(() => setIsLoading(false));
  }, [id]);

  if (isLoading) return <div className="p-8">Loading...</div>;
  if (!info) return <div className="p-8">Source not found</div>;

  const percent = info.storage_total
    ? (info.storage_used / info.storage_total) * 100
    : 0;

  return (
    <div className="p-8 flex flex-col gap-6">
      <Button isIconOnly variant="light" onPress={() => navigate(-1)}>
        <ArrowLeftIcon className="w-5 h-5" />
      </Button>
      <h1 className="text-3xl font-bold">{info.name}</h1>
      <SourceTypeChip type={info.type} />
      <div className="flex justify-center">
        <CircularProgress
          classNames={{
            svg: "w-36 h-36 drop-shadow-md",
            indicator: "stroke-white",
            track: "stroke-white/10",
            value: "text-3xl font-semibold text-white",
          }}
          showValueLabel
          strokeWidth={4}
          value={percent}
        />
      </div>
      <div className="border-2 border-dashed rounded p-4">
        {info.files.length === 0 ? (
          <p>No files</p>
        ) : (
          <table className="w-full text-left">
            <thead>
              <tr>
                <th className="pb-2">Name</th>
                <th className="pb-2">Size</th>
                <th className="pb-2"></th>
              </tr>
            </thead>
            <tbody>
              {info.files.map((f) => (
                <tr key={f.id} className="border-t">
                  <td className="py-2">{f.name}</td>
                  <td className="py-2">{f.size}</td>
                  <td className="py-2">
                    <Button
                      isIconOnly
                      variant="light"
                      onPress={() => downloadFile(id!, f)}
                    >
                      <DownloadIcon className="w-5 h-5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
