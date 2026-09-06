# 巴哈姆特、腾讯视频、优酷

2.12.0 新增「第三方弹幕库 → 巴哈姆特 / 腾讯视频 / 优酷」，三者使用独立弹幕池、数据库表、同步窗口、关键词规则和 Redis 缓存命名空间

## 搜索与创建

在对应入口点击「添加弹幕池」，输入关键词搜索作品，再选择剧集并创建；也可直接输入视频标识或受支持的视频页面链接

- 巴哈姆特：视频 SN，例如 `ani.gamer.com.tw/animeVideo.php?sn=...`，搜索会自动将简体关键词转为繁体，支持不同剧集分组
- 腾讯视频：视频 VID，例如 `v.qq.com/x/cover/{cid}/{vid}.html` 或包含 `vid` 的播放页，搜索使用作品 CID 加载剧集，支持分页、正片筛选和搜索返回的章节列表
- 优酷：视频 VID（保留末尾的 `==`），例如 `v.youku.com/v_show/id_{vid}.html` 或 `v.youku.com/video?vid=...`，作品 Show ID 用于分页加载剧集

搜索、选集只读取元数据；「创建并同步」才写入弹幕池并等待弹幕获取完成。腾讯 CID、优酷 Show ID 不能代替单集 VID

参考实现：[巴哈姆特](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/bahamut.js)、[腾讯视频](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/tencent.js)、[优酷](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/youku.js)

采用对应平台的搜索、剧集和弹幕接口，不启用参考工程的可选 TMDB 别名搜索、Bangumi-Data 本地库或跨站匹配服务。搜索不到时可尝试平台上的正式标题或手动输入视频 ID

## 缓存与管理

公开读取先返回本地/Redis 快照，再异步检查同步窗口并拉取上游；无历史弹幕时首次返回成功空列表。默认 600 秒内不重复回源，无定时刷新；后台新增和手动同步等待完成并显示错误

腾讯按分段索引读取，优酷根据时长按 60 秒分段；每个同步任务最多同时请求 4 个分段，上游返回异常、鉴权失败或任一分段失败时不提交本次部分结果，保留历史数据。合法空结果也不会清空旧弹幕

三者均支持单条屏蔽、全局/池级关键词过滤、从池或视频两侧关联及偏移，参与视频合并去重、总量统计、热力图和导出。全局关键词仅作用于当前来源，不跨平台应用；被屏蔽的数据仍保留

## 配置

完整字段见 [appsettings.json](../appsettings.json)，三个配置段为 `bahamut_setting`、`tencent_setting`、`youku_setting`

各平台的 `api_base`、`search_api_base`、`episodes_api_base` 和 `sync_interval_seconds` 可配置。腾讯 `api_base` 是 barrage 根地址；巴哈姆特和优酷的这些地址是完整接口地址

优酷还提供 `video_info_api_base`、`session_api_base`、`cna_api_base`，会自动取得临时会话 Cookie 并构造内外层签名，不在日志中打印 Cookie。配置字段及时间单位保持 snake_case 和秒

可选 `cookie` 用于对应来源的普通请求。优酷弹幕分段请求使用本次新取得的临时 Cookie，CNA 请求不携带配置 Cookie

上游可能受地区、登录权限或风控影响。自定义网关需提供兼容 JSON，优酷会话网关还必须保留 ETag/Set-Cookie 响应头；不跟随上游重定向，不自动绕过地区或访问权限限制

## 公开 API

将下列 `{source}` 替换为 `bahamut`、`tencent` 或 `youku`，均为 GET

- `/api/danmaku/dplayer/v3/{source}?episodeId={视频ID}&offset=-1.5`
- `/api/danmaku/artplayer/v1/{source}?episodeId={视频ID}`
- `/api/danmaku/v1/{source}?episodeId={视频ID}`
- `/api/danmaku/v1/{source}/xml?episodeId={视频ID}`

也可使用 `vid`，巴哈姆特可使用 `videoSn` 或 `sn`。查询值应进行 URL 编码；`offset` 为有符号秒数，仅调整输出，不修改池内时间

## 管理 API

管理员和弹幕管理员可访问，普通用户不能管理

- GET `/api/admin/{source}/search?keyword=...`
- GET `/api/admin/{source}/anime/{作品ID}/episodes`
- GET/POST `/api/admin/{source}/pools`，新增体 `{"episodeId":"视频ID"}`
- POST `/api/admin/{source}/pools/{id}/sync`
- GET `/api/admin/{source}/pools/{id}/danmaku`
- PATCH `/api/admin/{source}/danmaku/{id}/blocked`，请求体 `{"blocked":true}`
- GET/POST `/api/admin/{source}/keywords`，创建体 `{"poolId":null,"keyword":"关键词"}`
- DELETE `/api/admin/{source}/keywords/{id}`
- POST `/api/admin/videos/{id}/{source}-bindings`，请求体 `{"poolId":1,"offset":2}`
- DELETE `/api/admin/videos/{id}/{source}-bindings/{bindingId}`

## 启动迁移与验证

启动事务为每家创建 Pool、Danmaku、Keyword、Binding 四张表，以及唯一索引和视频外键。迁移成功后才开始监听，失败则终止启动；重复启动不清空数据，无需手动执行 SQL

Go 测试覆盖解析、分段失败、签名、异步快照、关联及缓存失效；设置 `CATALOG_LIVE_TEST=1` 可运行真实上游搜索选集拉取测试。设置指向隔离 PostgreSQL 测试库的 `DANMAKU_TEST_POSTGRES` 可运行迁移、重复初始化、来源隔离、过滤、去重及关联集成测试，禁止使用生产库

