export type ApiResponse<T> = { code: number; data: T };

export type AuthOptions = {
  casEnabled: boolean;
  defaultCAS: boolean;
  casLoginPath: string;
  localLoginPath: string;
};

export type SessionData = {
  id: number;
  name: string;
  username: string;
  role: "SuperAdmin" | "Admin" | "GeneralUser";
  email?: string;
  avatar?: string;
  provider: "local" | "cas";
};

export type Session = SessionData & {
  casEnabled: boolean;
  canManageDanmaku: boolean;
  canManageUsers: boolean;
  profileEditable: boolean;
};

export type PageMeta = { page: number; pageSize: number; total: number };

export type DanmakuData = {
  time: number;
  mode: number;
  size: number;
  color: number;
  timeStamp: number;
  pool: number;
  author: string;
  authorId: number;
  text: string | null;
};

export type Danmaku = {
  id: string;
  vid: string;
  data: DanmakuData;
  ip: string;
  isDelete: boolean;
  createTime: string;
  updateTime: string;
};

export type BilibiliPool = {
  id: number;
  bvid: string;
  aid: number;
  p: number;
  cid: number;
  danmakuCount: number;
  blockedCount: number;
  bindingCount: number;
  lastAttemptTime?: string | null;
  lastSyncTime?: string | null;
  createTime: string;
  updateTime: string;
};

export type BilibiliPoolDanmaku = {
  id: number;
  poolId: number;
  data: DanmakuData;
  isBlocked: boolean;
  manuallyBlocked: boolean;
  createTime: string;
  updateTime: string;
};

export type BilibiliKeyword = {
  id: number;
  poolId?: number | null;
  poolBvid: string;
  poolAid: number;
  poolP: number;
  poolCid: number;
  keyword: string;
  createTime: string;
};

export type BilibiliBinding = {
  id: number;
  vid: string;
  poolId: number;
  bvid: string;
  aid: number;
  p: number;
  cid: number;
  offset: number;
  createTime: string;
  updateTime: string;
};

export type IqiyiPool = {
  id: number;
  vid: string;
  danmakuCount: number;
  blockedCount: number;
  bindingCount: number;
  lastAttemptTime?: string | null;
  lastSyncTime?: string | null;
  createTime: string;
  updateTime: string;
};

export type IqiyiPoolDanmaku = {
  id: number;
  poolId: number;
  data: DanmakuData;
  isBlocked: boolean;
  manuallyBlocked: boolean;
  createTime: string;
  updateTime: string;
};

export type IqiyiKeyword = {
  id: number;
  poolId?: number | null;
  poolVid: string;
  keyword: string;
  createTime: string;
};

export type IqiyiBinding = {
  id: number;
  vid: string;
  poolId: number;
  poolVid: string;
  offset: number;
  createTime: string;
  updateTime: string;
};

export type DandanplayPool = {
  withRelated: boolean;
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

export type DandanplayPoolDanmaku = {
  id: number;
  poolId: number;
  data: DanmakuData;
  isBlocked: boolean;
  manuallyBlocked: boolean;
  createTime: string;
  updateTime: string;
};

export type DandanplayKeyword = {
  withRelated: boolean;
  id: number;
  poolId?: number | null;
  poolEpisodeId: string;
  keyword: string;
  createTime: string;
};

export type DandanplayBinding = {
  withRelated: boolean;
  id: number;
  vid: string;
  poolId: number;
  poolEpisodeId: string;
  offset: number;
  createTime: string;
  updateTime: string;
};

export type ExternalPool = {
  blockedCount: number;
  id: string;
  name: string;
  sourceFormat: string;
  danmakuCount: number;
  bindingCount: number;
  bindings?: ExternalBinding[];
  createTime: string;
  updateTime: string;
};

export type ExternalPoolDanmaku = {
  keywordBlocked: boolean;
  id: number;
  poolId: string;
  data: DanmakuData;
  createTime: string;
  updateTime: string;
};

export type ExternalBinding = {
  id: number;
  vid: string;
  poolId: string;
  poolName: string;
  offset: number;
  createTime: string;
  updateTime: string;
};

export type HeatmapPoint = {
  time: number;
  count: number;
};

export type ManagedVideo = {
  thirdPartyDanmakuCount: number;
  id: number;
  vid: string;
  name: string;
  isDelete: boolean;
  defaultPool: true;
  danmakuCount: number;
  bilibiliPoolCount: number;
  bilibiliBindings?: BilibiliBinding[];
  iqiyiPoolCount: number;
  dandanplayPoolCount: number;
  iqiyiBindings?: IqiyiBinding[];
  dandanplayBindings?: DandanplayBinding[];
  externalPoolCount: number;
  externalBindings?: ExternalBinding[];
  createTime: string;
  updateTime: string;
};

export type ManagedUser = {
  id: number;
  name: string;
  displayName: string;
  role: "administrator" | "danmaku_manager" | "user";
  superAdmin: boolean;
  enabled: boolean;
  provider: "local" | "cas";
  profileMutable: boolean;
  email?: string | null;
  avatar?: string | null;
  createTime: string;
  updateTime: string;
};

export type UserProfile = {
  id: number;
  name: string;
  role: number;
  enabled: boolean;
  phoneNumber?: string | null;
  email?: string | null;
  createTime: string;
  updateTime: string;
};
