import type { SVGProps } from "react";

export function EmbedsIcon(props: SVGProps<SVGSVGElement>) {
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
      <rect width="18" height="18" x="3" y="3" rx="2" />
      <path d="M3 9h18" />
      <path d="m10 14-2 2 2 2" />
      <path d="m14 18 2-2-2-2" />
      <path d="m9 9 2.5-4" />
    </svg>
  );
}
