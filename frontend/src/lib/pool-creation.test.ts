import { describe, expect, it, vi } from "vitest";
import { poolCreationWorkflow } from "./pool-creation";

describe("pool creation with optional video binding", () => {
  const body = { episodeId: "123" };
  const result = { pool: { id: 7 } };

  it("creates a standalone pool without sending a binding", async () => {
    const create = vi.fn().mockResolvedValue(result);
    const bind = vi.fn();
    expect(await poolCreationWorkflow().run(body, { videoID: "", offset: "" }, create, bind)).toEqual(result);
    expect(create).toHaveBeenCalledOnce();
    expect(bind).not.toHaveBeenCalled();
  });

  it.each(["0", "1.25", "-1.25"])("creates before binding with offset %s", async (offset) => {
    const create = vi.fn().mockResolvedValue(result);
    const bind = vi.fn(async (value) => { expect(create).toHaveBeenCalledOnce(); expect(value).toEqual(result); });
    await poolCreationWorkflow().run(body, { videoID: "42", offset }, create, bind);
    expect(bind).toHaveBeenCalledWith(result, 42, Number(offset));
  });

  it("does not bind when pool creation fails", async () => {
    const bind = vi.fn();
    await expect(poolCreationWorkflow().run(body, { videoID: "42", offset: "0" }, async () => { throw new Error("upstream failed"); }, bind)).rejects.toThrow("upstream failed");
    expect(bind).not.toHaveBeenCalled();
  });

  it("retries only binding after partial success, including external string IDs", async () => {
    const flow = poolCreationWorkflow();
    const external = { id: "external-uuid" };
    const create = vi.fn().mockResolvedValue(external);
    const bind = vi.fn().mockRejectedValueOnce(new Error("network error")).mockResolvedValueOnce({});
    await expect(flow.run(body, { videoID: "42", offset: "0" }, create, bind)).rejects.toThrow("弹幕池已创建，但关联视频失败");
    expect(await flow.run(body, { videoID: "43", offset: "-2.5" }, create, bind)).toEqual(external);
    expect(create).toHaveBeenCalledOnce();
    expect(bind).toHaveBeenLastCalledWith(external, 43, -2.5);
  });

  it("allows removing the optional binding after a binding failure", async () => {
    const flow = poolCreationWorkflow();
    const create = vi.fn().mockResolvedValue(result);
    const bind = vi.fn().mockRejectedValue(new Error("deleted video"));
    await expect(flow.run(body, { videoID: "42", offset: "0" }, create, bind)).rejects.toThrow();
    await flow.run(body, { videoID: "", offset: "0" }, create, bind);
    expect(create).toHaveBeenCalledOnce();
    expect(bind).toHaveBeenCalledOnce();
  });

  it.each([
    { videoID: "-1", offset: "0" }, { videoID: "1.5", offset: "0" },
    { videoID: "42", offset: "Infinity" }, { videoID: "42", offset: "" },
    { videoID: "42", offset: "invalid" },
  ])("rejects invalid association before creating: %o", async (target) => {
    const create = vi.fn();
    await expect(poolCreationWorkflow().run(body, target, create, vi.fn())).rejects.toThrow("有效");
    expect(create).not.toHaveBeenCalled();
  });

  it("does not reuse a previous pool when source inputs change", async () => {
    const flow = poolCreationWorkflow();
    const create = vi.fn().mockResolvedValue(result);
    const bind = vi.fn().mockRejectedValueOnce(new Error("failed")).mockResolvedValueOnce({});
    await expect(flow.run(body, { videoID: "42", offset: "0" }, create, bind)).rejects.toThrow();
    await flow.run({ episodeId: "456" }, { videoID: "42", offset: "0" }, create, bind);
    expect(create).toHaveBeenCalledTimes(2);
  });

  it("clears saved results when the creation dialog is closed", async () => {
    const flow = poolCreationWorkflow();
    const create = vi.fn().mockResolvedValue(result);
    const bind = vi.fn().mockRejectedValueOnce(new Error("failed")).mockResolvedValueOnce({});
    await expect(flow.run(body, { videoID: "42", offset: "0" }, create, bind)).rejects.toThrow();
    flow.reset();
    await flow.run(body, { videoID: "42", offset: "0" }, create, bind);
    expect(create).toHaveBeenCalledTimes(2);
  });
});
