# 弹弹play

2.10.0 新增弹弹play 拉取和管理，控制台入口为「第三方弹幕库 → 弹弹play」

## 弹幕池

新增弹幕池时可通过 **关键词搜索 → 选择作品 → 选择剧集** 自动填入剧集 ID，也可手动输入弹弹play **剧集 ID（episodeId，正整数）** 创建并立即拉取，不使用番剧 ID 或本地视频 ID。相同剧集 ID 只建立一个弹幕池，前导零会归一化

搜索先请求 `/v2/search/anime?keyword=...`，无结果时会去掉季度信息再尝试一次；选定作品后请求 `/v2/bangumi/{animeId}` 加载剧集。搜索和查看剧集不会创建弹幕池或拉取弹幕，只有「创建并同步」才会执行写入

- 默认 600 秒内复用缓存，超时后的下一次请求被动同步，不启动定时刷新
- 后台「立即同步」绕过同步窗口
- 按出现时间（毫秒）和弹幕内容增量去重，上游空响应不会删除历史弹幕
- 上游失败时公开读取保留缓存，手动同步会报告错误；失败尝试同样受同步窗口限制，避免重复回源
- 支持单条手动屏蔽、全局/指定池关键词过滤，屏蔽数据继续保留
- 视频详情和弹幕池列表均可发起关联并设置秒级偏移，参与视频合并、跨池去重、总量统计、热力图和现有格式导出
- Redis 启用时缓存池数据，同步和过滤状态变更使池缓存、合并结果缓存失效

## 配置

```json
"dandanplay_setting": {
  "api_base": "https://api.danmaku.weeblify.app/ddp/v1",
  "sync_interval_seconds": 600
}
```

参考 [danmu_api 的弹弹play 来源实现](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/dandan.js)，默认使用第三方网关，而不是弹弹play 官方域名。该网关会收到剧集 ID；其可用性由第三方维护

`api_base` 可改为自托管或其他兼容网关的完整地址，网关须接受 `path=/v2/comment/{episodeId}?from=0&withRelated=true&chConvert=0` 查询参数并返回原始 `comments` JSON。它不是任意官方 API 根地址，也不内置第三方应用密钥

本实现不启用参考项目依赖额外服务的 NipaPlay/TMDB/Bangumi-Data 搜索或跨站兜底，作品搜索和弹幕均使用上述可配置网关

## 公开 API

均为 GET，支持有符号的 `offset` 秒数

- DPlayer：`/api/danmaku/dplayer/v3/dandanplay/?episodeId=123&offset=-1.5`
- 通用 JSON：`/api/danmaku/v1/dandanplay?episodeId=123&offset=2`
- bilibili XML：`/api/danmaku/v1/dandanplay/xml?episodeId=123`
- ArtPlayer：`/api/danmaku/artplayer/v1/dandanplay?episodeId=123`

关联视频后，原有视频读取与导出接口自动包含该池。软删除视频仍返回空弹幕，不能由外部调用恢复

## 管理 API

管理员和弹幕管理员可访问，普通用户不可访问

- GET `/api/admin/dandanplay/search?keyword={作品名称}`
- GET `/api/admin/dandanplay/anime/{animeId}/episodes`

- GET/POST `/api/admin/dandanplay/pools`，新增请求体 `{"episodeId":"123"}`
- POST `/api/admin/dandanplay/pools/{id}/sync`
- GET `/api/admin/dandanplay/pools/{id}/danmaku`
- PATCH `/api/admin/dandanplay/danmaku/{id}/blocked`，请求体 `{"blocked":true}`
- GET/POST `/api/admin/dandanplay/keywords`，新增请求体 `{"poolId":null,"keyword":"示例"}`，null 表示全局
- DELETE `/api/admin/dandanplay/keywords/{id}`
- POST `/api/admin/videos/{videoId}/dandanplay-bindings`，请求体 `{"poolId":1,"offset":2.5}`
- DELETE `/api/admin/videos/{videoId}/dandanplay-bindings/{bindingId}`

服务启动时在现有迁移事务内自动创建 `DandanplayDanmakuPool`、`DandanplayDanmaku`、`DandanplayDanmakuKeyword`、`DandanplayDanmakuBinding` 及索引、外键，无需手动执行 SQL
