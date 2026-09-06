import { expect, it } from "vitest";
import { dandanplayPoolLabel } from "./dandanplay";

it("distinguishes pools sharing an episode ID in all selectors", () => {
  expect(dandanplayPoolLabel({ episodeId: "123", withRelated: true })).toBe("123 · 包含关联");
  expect(dandanplayPoolLabel({ episodeId: "123", withRelated: false })).toBe("123 · 不含关联");
});
