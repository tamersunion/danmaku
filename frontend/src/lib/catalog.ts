import type { CatalogPool } from "@/api/types";
export const catalogSources = ["bahamut", "tencent", "youku"] as const;
export type CatalogSource = typeof catalogSources[number];
export const catalogLabels: Record<CatalogSource, string> = { bahamut: "巴哈姆特", tencent: "腾讯视频", youku: "优酷" };
export const catalogIDLabels: Record<CatalogSource, string> = { bahamut: "视频 SN", tencent: "视频 VID", youku: "视频 VID" };
export function catalogPoolLabel(pool: Pick<CatalogPool, "episodeId">) { return pool.episodeId; }
