export type PoolVideoTarget = { videoID: string; offset: string };

// Keep a successful creation until its optional binding succeeds. A retry must
// not create a second external-import pool or fetch upstream danmaku again.
export function poolCreationWorkflow<TResult>() {
  let saved: { key: string; result: TResult } | undefined;
  return {
    reset() { saved = undefined; },
    async run(
      body: unknown,
      target: PoolVideoTarget,
      create: () => Promise<TResult>,
      bind: (result: TResult, videoID: number, offset: number) => Promise<unknown>,
    ): Promise<TResult> {
      const videoID = Number(target.videoID);
      const offset = Number(target.offset);
      if (target.videoID && (!Number.isSafeInteger(videoID) || videoID <= 0 || !target.offset.trim() || !Number.isFinite(offset))) {
        throw new Error("请选择有效视频并输入有效偏移量");
      }
      const key = JSON.stringify(body);
      if (!saved || saved.key !== key) saved = { key, result: await create() };
      const result = saved.result;
      if (target.videoID) {
        try {
          await bind(result, videoID, offset);
        } catch (error) {
          throw new Error(`弹幕池已创建，但关联视频失败；保持创建内容不变再次提交可重试关联：${error instanceof Error ? error.message : "请求失败"}`);
        }
      }
      saved = undefined;
      return result;
    },
  };
}
