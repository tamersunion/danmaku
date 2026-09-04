import { describe, expect, it } from "vitest";

import { colorHex, formatDateTime } from "@/lib/format";

describe("format helpers", () => {
  it("formats RFC3339 timestamps without NaN fragments", () => {
    expect(formatDateTime("2026-09-04T14:06:45Z")).not.toContain("NaN");
  });

  it("formats colors as six-digit hex values", () => {
    expect(colorHex(255)).toBe("#0000ff");
  });
});
