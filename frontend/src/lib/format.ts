import type { SessionData } from "@/api/types";

export function formatDateTime(value?: string | number | null): string {
  if (value === undefined || value === null || value === "") return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);

  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date);
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((entry) => entry.type === type)?.value ?? "";

  return `${part("year")}-${part("month")}-${part("day")} ${part("hour")}:${part("minute")}:${part("second")}`;
}

export function initials(value: string): string {
  const trimmed = value.trim();
  return trimmed
    ? Array.from(trimmed).slice(0, 2).join("").toUpperCase()
    : "DM";
}

export function colorHex(value: number): string {
  return `#${Math.max(0, Math.min(0xffffff, value)).toString(16).padStart(6, "0")}`;
}

export function sessionRoleLabel(role: SessionData["role"]): string {
  switch (role) {
    case "SuperAdmin":
      return "管理员";
    case "Admin":
      return "弹幕管理员";
    default:
      return "普通用户";
  }
}
