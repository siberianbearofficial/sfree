import type {SourceHealth, SourceHealthStatus} from "../../../shared/api/sources";

export const sourceHealthColor: Record<
  SourceHealthStatus,
  "success" | "warning" | "danger"
> = {
  healthy: "success",
  degraded: "warning",
  unhealthy: "danger",
};

export type SourceQuotaState =
  | {
      kind: "available";
      totalBytes: number;
      usedBytes: number;
      freeBytes: number;
      percent: number;
    }
  | {
      kind: "unavailable";
      label: string;
      description: string;
    };

export function getSourceQuotaState(
  health: SourceHealth | null | undefined,
): SourceQuotaState {
  if (
    health &&
    health.quota_total_bytes !== null &&
    health.quota_used_bytes !== null &&
    health.quota_free_bytes !== null &&
    health.quota_total_bytes > 0
  ) {
    const freeBytes = Math.max(0, health.quota_free_bytes);
    const percent = Math.max(
      0,
      Math.min(100, (health.quota_used_bytes / health.quota_total_bytes) * 100),
    );
    return {
      kind: "available",
      totalBytes: health.quota_total_bytes,
      usedBytes: health.quota_used_bytes,
      freeBytes,
      percent,
    };
  }

  if (!health) {
    return {
      kind: "unavailable",
      label: "Health unavailable",
      description: "SFree could not load a trustworthy capacity signal for this source.",
    };
  }

  switch (health.type) {
    case "telegram":
      return {
        kind: "unavailable",
        label: "Quota unavailable",
        description: "Telegram sources do not expose native quota metadata to SFree.",
      };
    case "s3":
      return {
        kind: "unavailable",
        label: "Quota unavailable",
        description: "S3-compatible sources are checked for reachability here, not provider-wide capacity limits.",
      };
    default:
      return {
        kind: "unavailable",
        label: "Quota unavailable",
        description: health.message || "This source did not return a trustworthy quota signal.",
      };
  }
}
