import { describe, expect, it } from "vitest";
import { localDateValue, normalizeHex, parseLocalDate, replaceColorChannel } from "./picker-values";

describe("date picker local dates", () => {
  it("round-trips dates without UTC conversion, including leap days", () => {
    for (const value of ["2026-09-06T00:00:00", "2024-02-29T23:59:59", "2000-01-01"]) {
      const parsed = parseLocalDate(value)!;
      expect(localDateValue(parsed)).toBe(value.slice(0, 10));
      expect(parsed.getHours()).toBe(0);
    }
  });
  it("does not silently normalize invalid or cleared dates", () => {
    for (const value of ["", "invalid", "2026-02-29", "2026-13-01", "2026-04-31"]) expect(parseLocalDate(value)).toBeUndefined();
  });
});

describe("color picker", () => {
  it("normalizes valid HEX values and rejects partial/invalid values", () => {
    expect(normalizeHex(" #AbC ")).toBe("#aabbcc");
    expect(normalizeHex("00FF80")).toBe("#00ff80");
    for (const value of ["", "#", "#ffff", "#gg0000", "#12345678"]) expect(normalizeHex(value)).toBeUndefined();
  });
  it("changes only the selected RGB channel", () => {
    expect(replaceColorChannel("#112233", 1, 255)).toBe("#11ff33");
    expect(replaceColorChannel("#ffffff", 0, 0)).toBe("#00ffff");
    expect(replaceColorChannel("#000000", 2, 256)).toBe("#0000ff");
  });
});
