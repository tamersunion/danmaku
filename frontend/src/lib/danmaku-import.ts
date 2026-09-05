import {
  ArtplayerAdapter,
  BahaAdapter,
  BiliCommandGrpcAdapter,
  BiliGrpcAdapter,
  BiliUpAdapter,
  BiliXmlAdapter,
  DanuniJsonAdapter,
  DanuniPbAdapter,
  DdplayAdapter,
  DplayerAdapter,
  IqiyiAdapter,
  MgtvAdapter,
  TencentAdapter,
  VodAdapter,
  YoukuAdapter,
} from "@dan-uni/dan-any/adapters";
import { UniDB } from "@dan-uni/dan-any/core/main/pure";
import { decode as decodeBase16384 } from "base16384";
import brotliDecompress from "brotli/decompress.js";
import { ungzip } from "pako";

import type { DanmakuData } from "@/api/types";

export const importFormats = [
  { value: "common.json", label: "本系统 JSON" },
  { value: "danuni.json", label: "DanUni JSON" },
  { value: "danuni.pb", label: "DanUni Protobuf" },
  { value: "bilibili.xml", label: "bilibili XML" },
  { value: "bilibili.pb", label: "bilibili 普通/高级 Protobuf" },
  { value: "bilibili-command.pb", label: "bilibili 指令 Protobuf" },
  { value: "bilibili-up.json", label: "bilibili UP 主 JSON" },
  { value: "dplayer.json", label: "DPlayer JSON" },
  { value: "artplayer.json", label: "ArtPlayer JSON" },
  { value: "ddplay.json", label: "弹弹Play JSON" },
  { value: "tencent.json", label: "腾讯视频 JSON" },
  { value: "vod.json", label: "VOD JSON" },
  { value: "baha.json", label: "巴哈姆特 JSON" },
  { value: "iqiyi.xml", label: "爱奇艺 XML" },
  { value: "youku.json", label: "优酷 JSON" },
  { value: "mgtv.json", label: "芒果 TV JSON" },
  { value: "ass", label: "dan-any ASS" },
] as const;

export type ImportFormat = (typeof importFormats)[number]["value"];

export async function importDanmakuFile(
  file: File,
  format: ImportFormat,
): Promise<DanmakuData[]> {
  if (format === "common.json") return importCommonJSON(await file.text());

  const database = new UniDB().init();
  try {
    const binary = async () => new Uint8Array(await file.arrayBuffer());
    const json = async () => JSON.parse(await file.text()) as unknown;
    if (format === "ass") {
      const chunk = await importDanAnyAss(database, await file.text());
      return normalizedDanmaku(chunk.$danmakus);
    }
    let adapter;
    switch (format) {
      case "danuni.json":
        adapter = DanuniJsonAdapter((await json()) as never);
        break;
      case "danuni.pb":
        adapter = DanuniPbAdapter(await binary());
        break;
      case "bilibili.xml":
        adapter = BiliXmlAdapter(await file.text());
        break;
      case "bilibili.pb":
        adapter = BiliGrpcAdapter(await binary());
        break;
      case "bilibili-command.pb":
        adapter = BiliCommandGrpcAdapter(await binary());
        break;
      case "bilibili-up.json":
        adapter = BiliUpAdapter((await json()) as never);
        break;
      case "dplayer.json":
        adapter = DplayerAdapter(normalizeDPlayer(await json()) as never);
        break;
      case "artplayer.json":
        adapter = ArtplayerAdapter(normalizeArtPlayer(await json()) as never);
        break;
      case "ddplay.json":
        adapter = DdplayAdapter((await json()) as never);
        break;
      case "tencent.json":
        adapter = TencentAdapter((await json()) as never);
        break;
      case "vod.json":
        adapter = VodAdapter((await json()) as never);
        break;
      case "baha.json":
        adapter = BahaAdapter((await json()) as never);
        break;
      case "iqiyi.xml":
        adapter = IqiyiAdapter(await file.text());
        break;
      case "youku.json":
        adapter = YoukuAdapter((await json()) as never);
        break;
      case "mgtv.json":
        adapter = MgtvAdapter((await json()) as never);
        break;
    }
    const chunk = await database.import(adapter);
    return normalizedDanmaku(chunk.$danmakus);
  } catch (error) {
    const message = error instanceof Error ? error.message : "文件内容无效";
    throw new Error(`无法按所选格式解析文件：${message}`);
  } finally {
    database.close();
  }
}

function normalizedDanmaku(
  data: Array<{
    progress: number;
    mode: string;
    fontsize: number;
    color: number;
    ctime: Date;
    pool: string;
    senderID: string;
    content: string;
  }>,
): DanmakuData[] {
  return data.map((item) => ({
      time: Math.max(0, item.progress / 1000),
      mode: toBilibiliMode(item.mode),
      size: item.fontsize || 25,
      color: item.color || 0xffffff,
      timeStamp: validTimestamp(item.ctime),
      pool: toPool(item.pool),
      author: item.senderID ?? "",
      authorId: 0,
      text: item.content,
    }));
}

async function importDanAnyAss(database: ReturnType<UniDB["init"]>, ass: string) {
  const compression = assValue(ass, "RawCompressType") || "gzip";
  const base = assValue(ass, "RawBaseType") || "base64";
  const current = assValue(ass, "RawPb");
  if (current) {
    const bytes = await decompressASS(decodeASSValue(current, base), compression);
    return database.import(DanuniPbAdapter(bytes));
  }
  const legacy = assValue(ass, "Raw");
  if (!legacy) throw new Error("ASS 中没有 dan-any 可还原的 Raw 数据");
  const raw = await decompressASS(decodeASSValue(legacy, base), compression);
  const parsed = JSON.parse(new TextDecoder().decode(raw)) as { list?: unknown[] };
  if (!Array.isArray(parsed.list)) throw new Error("ASS Raw 数据结构无效");
  const restored = parsed.list.map((value) => {
    if (!value || typeof value !== "object") return value;
    const item = value as Record<string, unknown>;
    const source = item.extra && typeof item.extra === "object"
      ? (item.extra as Record<string, unknown>)
      : item;
    const type = finiteNumber(item.type, 1);
    return {
      ...source,
      progress: finiteNumber(source.progress, finiteNumber(item.time, 0)),
      mode: source.mode ?? (type === 2 ? 1 : type === 3 ? 2 : 0),
      fontsize: finiteNumber(source.fontsize, finiteNumber(item.fontSizeType, 25)),
      content: source.content ?? item.content ?? "",
      color: finiteNumber(source.color, assColor(item.color)),
    };
  });
  return database.import(DanuniJsonAdapter(restored as never, { isV1: true }));
}

function assValue(ass: string, name: string): string {
  const prefix = `;${name}:`;
  return ass
    .split(/\r?\n/)
    .find((line) => line.startsWith(prefix))
    ?.slice(prefix.length)
    .trim() ?? "";
}

function decodeASSValue(value: string, base: string): Uint8Array {
  if (base === "base18384") return decodeBase16384(value);
  const binary = atob(value);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

async function decompressASS(bytes: Uint8Array, compression: string): Promise<Uint8Array> {
  if (compression === "zstd") {
    const zstd = await import("@bokuweb/zstd-wasm");
    await zstd.init();
    return zstd.decompress(bytes);
  }
  if (compression === "brotli") return new Uint8Array(brotliDecompress(bytes));
  if (compression === "gzip") return ungzip(bytes);
  throw new Error(`不支持的 ASS 压缩格式：${compression}`);
}

function assColor(value: unknown): number {
  if (!value || typeof value !== "object") return 0xffffff;
  const color = value as { r?: unknown; g?: unknown; b?: unknown };
  return (finiteNumber(color.r, 255) << 16) |
    (finiteNumber(color.g, 255) << 8) |
    finiteNumber(color.b, 255);
}

function importCommonJSON(raw: string): DanmakuData[] {
  const parsed = JSON.parse(raw) as unknown;
  const value =
    parsed && typeof parsed === "object" && "data" in parsed
      ? (parsed as { data: unknown }).data
      : parsed;
  if (!Array.isArray(value)) throw new Error("本系统 JSON 中没有弹幕数组");
  return value.map((item) => normalizeCommonItem(item));
}

function normalizeCommonItem(value: unknown): DanmakuData {
  if (!value || typeof value !== "object") throw new Error("弹幕记录无效");
  const item = value as Record<string, unknown>;
  const text = item.text;
  if (typeof text !== "string") throw new Error("弹幕内容无效");
  return {
    time: finiteNumber(item.time, 0),
    mode: finiteNumber(item.mode, 1),
    size: finiteNumber(item.size, 25),
    color: finiteNumber(item.color, 0xffffff),
    timeStamp: finiteNumber(item.timeStamp, Math.floor(Date.now() / 1000)),
    pool: finiteNumber(item.pool, 0),
    author: typeof item.author === "string" ? item.author : "",
    authorId: finiteNumber(item.authorId, 0),
    text,
  };
}

function normalizeDPlayer(value: unknown): unknown {
  return value && typeof value === "object" && "data" in value
    ? value
    : { code: 0, data: value };
}

function normalizeArtPlayer(value: unknown): unknown {
  if (!value || typeof value !== "object") return value;
  if ("danmuku" in value) return value;
  if ("data" in value && Array.isArray((value as { data: unknown }).data)) {
    return { danmuku: (value as { data: unknown[] }).data };
  }
  return value;
}

function toBilibiliMode(mode: string): number {
  if (mode === "Bottom") return 4;
  if (mode === "Top") return 5;
  if (mode === "Reverse") return 6;
  if (mode === "Ext") return 7;
  return 1;
}

function toPool(pool: string): number {
  if (pool === "Sub") return 1;
  if (pool === "Adv") return 2;
  if (pool === "Ix") return 3;
  return 0;
}

function validTimestamp(value: Date): number {
  const timestamp = value instanceof Date ? value.getTime() : Number.NaN;
  return Number.isFinite(timestamp)
    ? Math.floor(timestamp / 1000)
    : Math.floor(Date.now() / 1000);
}

function finiteNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}
