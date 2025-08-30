import type {SVGProps} from "react";

export function DownloadIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" {...props}>
      <path d="M4 20h16" />
      <path d="M12 4v12" />
      <path d="M7 12l5 5 5-5" />
    </svg>
  );
}
