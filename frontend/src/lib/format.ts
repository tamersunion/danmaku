export function formatDateTime(value?: string | number | null): string {
  if (value === undefined || value === null || value === "") return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? String(value)
    : date.toLocaleString("zh-CN", { hour12: false });
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
