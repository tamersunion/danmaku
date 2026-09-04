import { describe, expect, it } from "vitest";

import { colorHex, formatDateTime, sessionRoleLabel } from "@/lib/format";

describe("format helpers", () => {
  it("formats RFC3339 timestamps without NaN fragments", () => {
    expect(formatDateTime("2026-09-05T13:14:00Z")).toBe("2026-09-05 21:14:00");
  });

  it("formats colors as six-digit hex values", () => {
    expect(colorHex(255)).toBe("#0000ff");
  });

  it("labels the three permission levels", () => {
    expect(sessionRoleLabel("SuperAdmin")).toBe("管理员");
    expect(sessionRoleLabel("Admin")).toBe("弹幕管理员");
    expect(sessionRoleLabel("GeneralUser")).toBe("普通用户");
  });
});
