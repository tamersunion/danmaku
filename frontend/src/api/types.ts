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
  p: number;
  cid: number;
  offset: number;
  createTime: string;
  updateTime: string;
};

export type ManagedVideo = {
  id: number;
  vid: string;
  name: string;
  isDelete: boolean;
  defaultPool: true;
  danmakuCount: number;
  bilibiliPoolCount: number;
  bilibiliBindings?: BilibiliBinding[];
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
