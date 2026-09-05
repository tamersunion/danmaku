export function localDateValue(date: Date): string {
  return [String(date.getFullYear()).padStart(4, "0"), String(date.getMonth()+1).padStart(2, "0"), String(date.getDate()).padStart(2, "0")].join("-");
}

export function parseLocalDate(value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})(?:T|$)/.exec(value);
  if (!match) return undefined;
  const date = new Date(0);
  date.setFullYear(Number(match[1]), Number(match[2])-1, Number(match[3]));
  date.setHours(0, 0, 0, 0);
  return localDateValue(date) === match[0].replace(/T$/, "") ? date : undefined;
}

export function normalizeHex(value: string): string | undefined {
  const hex = value.trim().replace(/^#/, "");
  if (/^[\da-f]{6}$/i.test(hex)) return `#${hex.toLowerCase()}`;
  if (/^[\da-f]{3}$/i.test(hex)) return `#${[...hex].map(char => char+char).join("").toLowerCase()}`;
  return undefined;
}

export function replaceColorChannel(color: string, index: number, channel: number): string {
  const channels = [1, 3, 5].map(start => Number.parseInt(color.slice(start, start+2), 16));
  channels[index] = Math.max(0, Math.min(255, Math.round(channel)));
  return `#${channels.map(value => value.toString(16).padStart(2, "0")).join("")}`;
}
