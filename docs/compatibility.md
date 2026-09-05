# Go 后端兼容说明

## HTTP API

所有 JSON 响应继续使用 `{ "code": number, "data": any }`。业务成功为 `code: 0`，业务失败为 `code: 1`，管理接口未登录或无管理员权限为 `code: 401`。

自 2.0.1 起，API 路径中的旧 `danmu` 拼写不再提供别名：客户端需改用下表中的 `danmaku` 路径，OpenResty 也直接代理新路径，不再执行名称改写。

2.0.2 的前端时间转换同时接受旧后端返回的无时区字符串和 Go 后端返回的 RFC3339 时间，避免重复追加 `Z` 后显示为 `NaN-aN-aN aN:aN:aN`。

2.1.0 将管理前端迁移到 React 与 shadcn/ui，并新增用户管理接口。原有公开弹幕接口及其请求、响应结构不变。

2.1.1 将角色 `2` 明确为弹幕管理员：可以访问弹幕管理接口，但不能访问用户管理接口。

2.1.2 优化弹幕管理界面、统一弹幕控制台品牌图标与时间格式；HTTP API 和数据库结构均未变更。

2.1.3 移除侧栏中重复的个人资料入口，继续通过侧栏底部头像菜单进入个人资料；HTTP API 和数据库结构均未变更。

2.1.4 修复删除确认框不会关闭以及历史弹幕时间精度触发表单校验的问题；HTTP API 和数据库结构均未变更。

2.1.5 增加提交去重：30 秒内客户端 IP、视频 ID 与弹幕内容均相同时，后续提交仍返回原有成功响应，但不写入数据库。服务启动时会自动创建去重查询索引。

2.2.0 将 bilibili 弹幕改为 PostgreSQL 持久化弹幕池：AID 先解析为一一对应的 BVID，再由 BVID + P 解析 CID，最终以 CID 作为弹幕池唯一标识，BVID 和 P 仅作为可选来源元数据。上次上游获取后的固定 10 分钟内只返回缓存，超过后由下一次弹幕池请求被动增量更新，后台也可手动触发；服务不会定时主动刷新。弹幕以发送时间戳和内容去重，空上游响应不再清空既有数据。新增弹幕池及全局关键词过滤、单条屏蔽、本地视频关联和秒级 `offset` 偏移功能；后台可通过 BVID、AID 或 CID（P 默认为 1）创建弹幕池并立即完成首次获取。

2.3.0 新增独立视频管理。外部弹幕查询或提交首次遇到视频 ID 时只创建该 ID 对应的视频记录，不写入名称或来源资料；系统自带弹幕池始终固定关联且不能取消。后台可从视频详情或 bilibili 弹幕池列表双向发起视频关联并设置各自偏移量，上一版 `/api/admin/bilibili/bindings` 管理接口同步下线。视频删除为软删除：外部查询返回空弹幕，提交返回原有业务失败响应，外部请求不会恢复视频；管理员可在视频管理中恢复。

2.4.0 将配置切换为仅支持 JSON 的 `snake_case` 结构，并删除 YAML、PascalCase、`danmu_sql`、`danmu_cache_time` 等旧配置兼容。配置解析会拒绝未知字段，避免拼写错误静默回落到默认值。所有配置时间统一为秒；此前固定为 10 分钟的 bilibili 弹幕池缓存窗口改由 `bilibili_setting.sync_interval_seconds` 控制，默认值为 600。

bilibili 上游 API 基址由 `bilibili_setting.api_base` 配置，默认使用 `https://api.bilibili.com`；末尾斜杠会在加载时移除。CID 元数据缓存时间由 `bilibili_setting.cid_cache_seconds` 配置。

2.5.0 由服务端原生接管爱奇艺弹幕获取。对外入口仍为 `GET /api/danmaku/dplayer/v3/iqiyi/?vid={vid}`，只接受爱奇艺 VID，响应继续使用 DPlayer v3 的 `code/data` 结构。服务会解析 VID、读取视频时长，并发获取爱奇艺 60 秒 Brotli 弹幕分片，同时兼容 XML 和 protobuf 分片内容；单个分片失败不会丢弃其他已成功获取的弹幕。

2.6.0 将爱奇艺弹幕纳入与 bilibili 并列的持久化第三方弹幕池管理：支持 VID 建池、被动增量同步、手动更新、单条屏蔽、全局或池级关键词以及带秒级偏移量的视频关联。视频详情统一管理全部第三方弹幕池，系统池改为直接跳转到已按视频 ID 筛选的弹幕管理页。管理前端的选项选择器统一支持输入搜索。bilibili 的 AID/BVID 按[新版转换规则](https://github.com/ILoveScratch2/bilibili-api-collect-new/blob/main/docs/misc/bvid_desc.md)在本地双向转换，不再依赖 `archive/stat`；弹幕池同时返回和展示 AID、BVID。

2.7.0 增加人工导入的外部弹幕库：由服务端生成 UUID，支持覆盖重导且保留原 ID 和视频关联。管理前端通过 [dan-any](https://github.com/ani-uni/dan-any) 解析本系统、DanUni、bilibili、DPlayer、ArtPlayer、弹弹Play、腾讯、VOD、巴哈姆特、爱奇艺、优酷、芒果 TV 和可还原 ASS 等输入格式。视频详情可直接向既有 DPlayer 提交接口添加系统弹幕、关联全部第三方来源、查看按秒统计的合并弹幕热力图，并导出全部双向格式。bilibili AID 始终由 BVID 在响应时本地计算，不新增 AID 数据库列。

2.8.0 增加外部导入弹幕池的全局／池级关键词过滤，启动时自动创建 `ExternalDanmakuKeyword` 表和唯一索引。关键词按不区分大小写的子串匹配；原数据保留，覆盖重导后仍应用原规则，删除规则可恢复显示。

视频合并输出按偏移后的出现时间跨池去重：内容完全相同且时间差不超过 1 秒时，系统池始终优先；第三方按池内原始记录总数降序处理（包含屏蔽记录），数量相同则保持稳定关联顺序。同池重复记录不删除，被淘汰的记录不继续淘汰其他池记录。此规则同时用于视频读取、导出和热力图，不改写源数据库。合并结果独立缓存到 Redis；未配置 Redis 时使用最多 128 项、按缓存 TTL 到期的进程内缓存。任何来源的弹幕或关键词修改都会使合并缓存失效，关联池和偏移变化会切换缓存键；缓存命中仍会检查上游被动刷新窗口。视频列表的第三方总条数为关联池原始记录总数，不是过滤去重后的输出数。

同版本增加可选 Redis 弹幕缓存。缓存覆盖系统、bilibili、爱奇艺和外部导入池，写入、编辑、软删除、同步、屏蔽或关键词变更后通过命名空间版本立即失效；Redis 临时读写失败时读取会回退 PostgreSQL。配置显式启用 Redis 时，启动阶段连接失败会阻止服务监听，避免部署误以为缓存已经生效。

| 方法 | 路径 | 兼容行为 |
| --- | --- | --- |
| GET | `/api/danmaku/v1?id={vid}` | 返回通用弹幕数组；存在第三方弹幕池关联时自动合并可见弹幕；软删除视频返回空数组 |
| GET | `/api/danmaku/v1/{vid}.xml` | 返回 bilibili XML 格式；软删除视频返回空弹幕文档 |
| GET | `/api/danmaku/export?id={vid}&format={format}&offset={seconds}` | 导出已合并弹幕；支持 `common.json`、`danuni.json`、`danuni.pb`、`bilibili.xml`、`dplayer.json`、`artplayer.json`、`ddplay.json`、`vod.json`、`baha.json`、`ass`；ASS 内嵌可还原原始数据 |
| POST | `/api/danmaku/v1` | 接收通用弹幕 JSON |
| GET/POST | `/api/danmaku/dplayer/v3` | DPlayer v3 查询与提交 |
| GET | `/api/danmaku/dplayer/v3/iqiyi/?vid={vid}` | 按爱奇艺 VID 获取持久化 DPlayer v3 弹幕；可传秒级 `offset` |
| GET/POST | `/api/danmaku/artplayer/v1` | ArtPlayer 查询与提交；查询默认 XML，`.json` 返回 JSON |
| GET | `/api/danmaku/v1/bilibili/*`、`/api/danmaku/dplayer/v3/bilibili`、`/api/danmaku/artplayer/v1/bilibili/*` | 按 `cid`，或 `aid`/`bvid` + `p` 获取持久化 bilibili 弹幕；可传 `offset`，正数延后、负数提前，单位为秒 |
| GET | `/api/other/bilibili/queryaid` | 查询 `aid`、`bvid` 和分 P 列表 |
| POST | `/api/admin/login` | 建立 `DCookie` 会话并写入 `ClientAuth` 角色 Cookie |
| GET | `/api/admin/logout` | 清理 Cookie 并以 302 跳转 `/` |
| GET | `/api/admin/auth/options` | 返回 CAS 登录选项 |
| GET | `/api/admin/session` | 从服务端 Cookie 恢复当前管理会话 |
| GET/POST | `/api/admin/user/*` | 用户资料与密码操作 |
| GET/POST | `/api/admin/users` | 管理员分页查询用户或创建本地用户；CAS 开启时不允许创建本地用户 |
| GET/PUT/DELETE | `/api/admin/users/{id}` | 管理员查询、更新或删除用户；CAS 用户只能修改角色 |
| PATCH | `/api/admin/users/{id}/status` | 管理员启用或停用用户 |
| GET/POST | `/api/admin/videos` | 分页查询视频或创建视频；可按视频 ID、名称和删除状态筛选 |
| GET/PUT/DELETE | `/api/admin/videos/{id}` | 查询详情、修改名称或软删除视频 |
| PATCH | `/api/admin/videos/{id}/status` | 恢复或软删除视频 |
| POST/DELETE | `/api/admin/videos/{id}/bilibili-bindings[/{bindingId}]` | 为指定视频新增、更新或删除 bilibili 弹幕池关联及秒级偏移量 |
| POST/DELETE | `/api/admin/videos/{id}/iqiyi-bindings[/{bindingId}]` | 为指定视频新增、更新或删除爱奇艺弹幕池关联及秒级偏移量 |
| POST/DELETE | `/api/admin/videos/{id}/external-bindings[/{bindingId}]` | 为指定视频新增、更新或删除人工导入弹幕池关联及秒级偏移量 |
| GET | `/api/admin/videos/{id}/heatmap?granularity={seconds}` | 返回系统和全部已关联第三方弹幕的热力图桶；默认粒度 1 秒 |
| GET | `/api/admin/danmakulist[/vids\|/dateselect\|/baseselect]` | 管理列表和筛选 |
| GET/POST | `/api/admin/danmakuedit[/delete\|/edit]` | 查询、软删除和编辑 |
| GET/POST | `/api/admin/bilibili/pools`、`/pools/{id}/sync`、`/pools/{id}/danmaku` | 查询弹幕池；通过 BVID/AID/CID + P（默认 1）创建并立即同步；强制增量同步及审阅池内弹幕 |
| PATCH | `/api/admin/bilibili/danmaku/{id}/blocked` | 设置单条 bilibili 弹幕的手动屏蔽状态 |
| GET/POST/DELETE | `/api/admin/bilibili/keywords[/{id}]` | 管理全局或指定弹幕池的过滤关键词 |
| GET/POST | `/api/admin/iqiyi/pools`、`/pools/{id}/sync`、`/pools/{id}/danmaku` | 按爱奇艺 VID 查询、创建、同步和审阅持久化弹幕池 |
| PATCH | `/api/admin/iqiyi/danmaku/{id}/blocked` | 设置单条爱奇艺弹幕的手动屏蔽状态 |
| GET/POST/DELETE | `/api/admin/iqiyi/keywords[/{id}]` | 管理爱奇艺全局或指定弹幕池的过滤关键词 |
| GET/POST | `/api/admin/external` | 分页查询外部弹幕池，或创建人工导入弹幕池并由后端生成 UUID |
| GET/PUT | `/api/admin/external/{uuid}` | 查询弹幕池详情，或覆盖重导内容且保留 UUID 和已有视频关联 |
| GET | `/api/admin/external/{uuid}/danmaku` | 分页审阅人工导入弹幕池内容 |

客户端 IP 的取值顺序保持代理部署习惯：`X-Real-IP`、`X-Forwarded-For`、请求远端地址。提交时 JSON 未提供 `referer` 则读取 `Referer` 请求头。

## SignalR

端点为 `/api/live/danmaku`，支持 WebSocket 和 Server-Sent Events：

- `Connection(group)`：加入指定组。
- `SendMessage(group, user, message)`：向同组内除发送者以外的客户端触发 `ReceiveMessage(user, message)`。

## PostgreSQL

Go 后端启动时在单个事务中迁移并读写以下区分大小写对象：

- 表：`"Danmaku"`、`"Video"`、`"User"`、`"HttpClientCache"`、`"BilibiliDanmakuPool"`、`"BilibiliDanmaku"`、`"BilibiliDanmakuKeyword"`、`"BilibiliDanmakuBinding"`、`"IqiyiDanmakuPool"`、`"IqiyiDanmaku"`、`"IqiyiDanmakuKeyword"`、`"IqiyiDanmakuBinding"`、`"ExternalDanmakuPool"`、`"ExternalDanmaku"`、`"ExternalDanmakuBinding"`。
- 软删除列：`"Danmaku"."IsDelete"`、`"Video"."IsDelete"`。
- 视频外键：`"Danmaku"."VideoId"` -> `"Video"."Id"`。
- 视频资料：唯一 `"Video"."Vid"` 及可空 `"Video"."Name"`；系统自带弹幕池为固有关系，不另建可删除的关联行。
- 弹幕 JSONB：`"Danmaku"."Data"`，内部属性继续写为 `Time`、`Mode`、`Size`、`Color`、`TimeStamp`、`Pool`、`Author`、`AuthorId`、`Text`。
- 来源 JSONB：`"Video"."Referer"`，内部属性继续写为 `Protocol`、`Host`、`Port`、`Path`、`Query`、`Fragment`。

数据库读取同时兼容既有 PascalCase JSONB 和可能存在的 camelCase 数据，但新增数据仍写成旧 EF Core 的 PascalCase 结构。

bilibili 弹幕池以唯一 CID 为准；BVID 与 P 只保存为辅助解析和展示元数据，从 CID 直接创建时 BVID 可为空，后续通过 BVID/AID 访问同一 CID 时会补齐。弹幕正文同样以 PascalCase JSONB 保存，并额外记录发送时间戳及内容 SHA-256 用于增量去重。手动屏蔽状态保存在弹幕行上；关键词规则在公开查询时计算，命中的行仍保留在数据库中但默认不返回。视频关联只保存本地 `Vid`、弹幕池和偏移量，不给本地视频模型增加分 P 字段。

AID 不写入数据库：只要弹幕池存在 BVID，API 和管理前端展示的 AID 都由服务端使用可逆算法实时计算。旧数据库中缺少 `BV` 前缀的 BVID 也会先在内存中规范化，不触发数据写回。

爱奇艺弹幕池以爱奇艺 VID 唯一标识；弹幕按出现时间（毫秒精度）与内容 SHA-256 增量去重。上游返回空内容时仅更新同步时间，不删除已有缓存。屏蔽、关键词和视频偏移关联与 bilibili 弹幕池相互独立。

外部导入弹幕池使用 UUID 唯一标识，弹幕按出现时间（毫秒精度）与内容 SHA-256 去重；覆盖重导在事务中替换弹幕内容，不重建弹幕池或视频关联。新表和索引与其他结构一样在服务开始监听前自动创建。

已有 `"Danmu"` 表、`FK_Danmu_*` 外键和 `IX_Danmu_*` 索引会自动改为 `Danmaku` 命名。`"User"` 会新增 `"CASSubject"`、`"CASDisplayName"`、`"CASAvatar"`、`"Enabled"` 字段及唯一索引，用于稳定绑定、同步 CAS 资料及停用账户。`"Video"` 会新增名称和软删除字段、统一时间列名称、按 `Vid` 合并旧重复行并重新关联弹幕；既有 bilibili 或爱奇艺关联中的视频 ID 会补建为视频记录。结构检查在服务启动、开始监听前自动完成。

用户角色 `1` 为管理员，可管理弹幕和用户；角色 `2` 为弹幕管理员，只能管理弹幕；角色 `3` 为普通用户，只能查看自己的资料。CAS 启用后，CAS 返回的用户名、显示名、邮箱和头像会在登录时同步；包括电话在内的本地资料及密码均不可由本项目修改，只允许管理员调整角色或启停账户。
