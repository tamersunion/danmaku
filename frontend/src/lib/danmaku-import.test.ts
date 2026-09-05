import { describe, expect, it } from "vitest";
import { gzip } from "pako";

import { importDanmakuFile } from "@/lib/danmaku-import";

describe("importDanmakuFile", () => {
  it("imports the existing common JSON response", async () => {
    const file = new File(
      [
        JSON.stringify({
          code: 0,
          data: [
            {
              time: 1.25,
              mode: 1,
              size: 25,
              color: 0xffffff,
              timeStamp: 123,
              pool: 0,
              author: "alice",
              authorId: 0,
              text: "hello",
            },
          ],
        }),
      ],
      "common.json",
    );
    const result = await importDanmakuFile(file, "common.json");
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ time: 1.25, text: "hello" });
  });

  it("imports DPlayer JSON through dan-any", async () => {
    const file = new File(
      [JSON.stringify({ code: 0, data: [[2.5, 1, 0xff0000, "sender", "top"]] })],
      "dplayer.json",
    );
    const result = await importDanmakuFile(file, "dplayer.json");
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ time: 2.5, mode: 5, color: 0xff0000, text: "top" });
  });

  it("imports bilibili XML through dan-any", async () => {
    const file = new File(
      ['<?xml version="1.0" encoding="UTF-8"?><i><chatid>99</chatid><d p="3,4,25,255,123,0,hash,1,0">bottom</d></i>'],
      "bilibili.xml",
    );
    const result = await importDanmakuFile(file, "bilibili.xml");
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ time: 3, mode: 4, color: 255, text: "bottom" });
  });

  it("restores the Raw payload from a dan-any ASS file", async () => {
    const raw = JSON.stringify({
      list: [
        {
          time: 4.5,
          type: 3,
          fontSizeType: 25,
          content: "ass-top",
          color: { r: 255, g: 255, b: 255 },
        },
      ],
    });
    const encoded = btoa(
      Array.from(gzip(raw), (value) => String.fromCharCode(value)).join(""),
    );
    const file = new File(
      [`[Script Info]\n;RawCompressType: gzip\n;RawBaseType: base64\n;Raw: ${encoded}`],
      "danmaku.ass",
    );
    const result = await importDanmakuFile(file, "ass");
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ time: 4.5, mode: 5, text: "ass-top" });
  });
});
