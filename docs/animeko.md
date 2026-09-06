# Animeko

2.11.0 新增「第三方弹幕库 → Animeko」，支持关键词搜索 Bangumi 作品、选择剧集，或手动输入 Bangumi 剧集 ID 创建弹幕池并同步

池以归一化的正整数 Bangumi 剧集 ID 唯一标识，不与弹弹play 剧集 ID 混用。支持增量更新、单条屏蔽、全局及池级关键词过滤、视频关联和秒级偏移，参与合并去重、统计、热力图及导出。空响应不清除历史弹幕，更新与规则变化使池缓存和合并缓存失效

公开读取先返回本地/Redis 已有数据，再在后台检查同步窗口并获取上游。冷池首次返回成功空列表，无定时刷新；后台创建和手动同步等待上游完成并报告错误

## 来源与关联弹幕

参考 [danmu_api 的 Animeko 实现](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/animeko.js)，读取 `GET /v1/danmaku/{episodeId}`

公开的 [Animeko API 客户端](https://github.com/open-ani/animeko/blob/main/client/src/commonMain/gen/me/him188/ani/client/apis/DanmakuAniApi.kt) 不提供关联来源开关，弹幕模型也没有逐条来源字段。[应用端弹幕仓库](https://github.com/open-ani/animeko/blob/main/app/shared/app-data/src/commonMain/kotlin/domain/danmaku/DanmakuRepository.kt) 将 Animeko 与弹弹play 作为不同 provider 在客户端合并。因此本项目仅接入 Animeko API，不额外合并弹弹play，也不展示无法生效的「包含关联弹幕」开关；上述接口不能用于断言服务端历史数据的全部来源

## 配置

```json
"animeko_setting": {
  "api_base": "https://api.animeko.org",
  "bangumi_api_base": "https://api.bgm.tv",
  "sync_interval_seconds": 600
}
```

## 接口

公开 GET，`episodeId` 为 Bangumi 剧集 ID，`offset` 为有符号秒数

- `/api/danmaku/dplayer/v3/animeko/?episodeId=1227087&offset=-1.5`
- `/api/danmaku/v1/animeko?episodeId=1227087`
- `/api/danmaku/v1/animeko/xml?episodeId=1227087`
- `/api/danmaku/artplayer/v1/animeko?episodeId=1227087`

管理接口限管理员和弹幕管理员

- GET `/api/admin/animeko/search?keyword=葬送的芙莉莲`
- GET `/api/admin/animeko/anime/{subjectId}/episodes`
- GET/POST `/api/admin/animeko/pools`，创建体 `{"episodeId":"1227087"}`
- POST `/api/admin/animeko/pools/{id}/sync`
- GET `/api/admin/animeko/pools/{id}/danmaku`
- PATCH `/api/admin/animeko/danmaku/{id}/blocked`，屏蔽体 `{"blocked":true}`
- GET/POST `/api/admin/animeko/keywords`
- DELETE `/api/admin/animeko/keywords/{id}`
- POST `/api/admin/videos/{id}/animeko-bindings`，请求体 `{"poolId":1,"offset":1.5}`
- DELETE `/api/admin/videos/{id}/animeko-bindings/{bindingId}`

启动事务自动创建四张 Animeko 表及索引、视频外键，重复启动不清空已有数据。可设置指向隔离测试库的 `DANMAKU_TEST_POSTGRES` 运行集成测试，覆盖重复初始化、增量去重、过滤、关联和数据保留；不要使用生产数据库
