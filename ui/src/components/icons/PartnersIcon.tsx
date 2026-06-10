import type { SVGProps } from "react";

export function PartnersIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      <path d="m11 17 2 2a1 1 0 1 0 3-3" />
      <path d="m14 14 2.5 2.5a1 1 0 1 0 3-3l-3.88-3.88a3 3 0 0 0-4.24 0l-7.38 7.38a6 6 0 1 0 8.49 8.49l.05-.05" />
      <path d="m9 18.5 2.5-2.5" />
      <path d="m14 14-2.5-2.5" />
      <path d="m9 13.5 2.5-2.5" />
      <path d="m14 9.5-2.5-2.5" />
      <path d="M17 11.5 19.5 9a1 1 0 1 0-3-3l-3.88 3.88a3 3 0 0 0-4.24 0l-7.38-7.38a6 6 0 1 0 8.49-8.49l.05.05" />
    </svg>
  );
}
