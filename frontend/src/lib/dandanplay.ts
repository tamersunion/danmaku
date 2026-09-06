export function dandanplayPoolLabel(pool: { episodeId: string; withRelated: boolean }): string {
  return `${pool.episodeId} · ${pool.withRelated ? "包含关联" : "不含关联"}`;
}
