import type {SVGProps} from "react";

export function S3Icon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" {...props}>
      <path d="M12 2C6.48 2 2 4.69 2 8v8c0 3.31 4.48 6 10 6s10-2.69 10-6V8c0-3.31-4.48-6-10-6zm0 2c4.42 0 8 1.79 8 4s-3.58 4-8 4-8-1.79-8-4 3.58-4 8-4zm8 12c0 2.21-3.58 4-8 4s-8-1.79-8-4v-2.23c1.83 1.38 4.74 2.23 8 2.23s6.17-.85 8-2.23V16zm0-4c0 2.21-3.58 4-8 4s-8-1.79-8-4V9.77C5.83 11.15 8.74 12 12 12s6.17-.85 8-2.23V12z" />
    </svg>
  );
}
