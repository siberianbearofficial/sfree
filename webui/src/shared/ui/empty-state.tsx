import type {ReactNode} from "react";
import {Button} from "@heroui/react";

type Props = {
  icon?: ReactNode;
  title: string;
  description?: string;
  ctaLabel?: string;
  onCtaPress?: () => void;
  variant?: "default" | "danger";
};

export function EmptyState({icon, title, description, ctaLabel, onCtaPress, variant = "default"}: Props) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[200px] text-center py-8">
      {icon && <div className={variant === "danger" ? "text-danger mb-3" : "text-default-300 mb-3"}>{icon}</div>}
      <h3 className="text-lg font-semibold">{title}</h3>
      {description && <p className="text-default-500 text-sm mt-1 max-w-md">{description}</p>}
      {ctaLabel && onCtaPress && (
        <Button color={variant === "danger" ? "danger" : "primary"} variant={variant === "danger" ? "flat" : "solid"} className="mt-4" onPress={onCtaPress}>
          {ctaLabel}
        </Button>
      )}
    </div>
  );
}
