import type { ComponentProps } from "react";

export function DanmakuLogo(props: ComponentProps<"svg">) {
  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      {...props}
    >
      <rect
        x="1.75"
        y="3.25"
        width="28.5"
        height="25.5"
        rx="6.75"
        stroke="currentColor"
        strokeWidth="2.5"
      />
      <circle cx="8" cy="10" r="1.75" fill="currentColor" />
      <path
        d="M12 10H24"
        stroke="currentColor"
        strokeWidth="2.75"
        strokeLinecap="round"
      />
      <circle cx="5.5" cy="16" r="1.75" fill="currentColor" />
      <path
        d="M9.5 16H20.5"
        stroke="currentColor"
        strokeWidth="2.75"
        strokeLinecap="round"
      />
      <circle cx="11" cy="22" r="1.75" fill="currentColor" />
      <path
        d="M15 22H26.5"
        stroke="currentColor"
        strokeWidth="2.75"
        strokeLinecap="round"
      />
    </svg>
  );
}
