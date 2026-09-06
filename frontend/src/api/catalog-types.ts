import type { DanmakuData } from "./types";
export type CatalogPool = {
  id: number;
  episodeId: string;
  danmakuCount: number;
  blockedCount: number;
  bindingCount: number;
  lastAttemptTime?: string | null;
  lastSyncTime?: string | null;
  createTime: string;
  updateTime: string;
};

export type CatalogPoolDanmaku = {
  id: number;
  poolId: number;
  data: DanmakuData;
  isBlocked: boolean;
  manuallyBlocked: boolean;
  createTime: string;
  updateTime: string;
};

export type CatalogKeyword = {
  id: number;
  poolId?: number | null;
  poolEpisodeId: string;
  keyword: string;
  createTime: string;
};

export type CatalogBinding = {
  id: number;
  vid: string;
  poolId: number;
  poolEpisodeId: string;
  offset: number;
  createTime: string;
  updateTime: string;
};
