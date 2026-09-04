# Go 后端兼容说明

## HTTP API

所有 JSON 响应继续使用 `{ "code": number, "data": any }`。业务成功为 `code: 0`，业务失败为 `code: 1`，管理接口未登录或无管理员权限为 `code: 401`。

自 2.0.1 起，API 路径中的旧 `danmu` 拼写不再提供别名：客户端需改用下表中的 `danmaku` 路径，OpenResty 也直接代理新路径，不再执行名称改写。

2.0.2 的前端时间转换同时接受旧后端返回的无时区字符串和 Go 后端返回的 RFC3339 时间，避免重复追加 `Z` 后显示为 `NaN-aN-aN aN:aN:aN`。

2.1.0 将管理前端迁移到 React 与 shadcn/ui，并新增用户管理接口。原有公开弹幕接口及其请求、响应结构不变。

2.1.1 将角色 `2` 明确为弹幕管理员：可以访问弹幕管理接口，但不能访问用户管理接口。

2.1.2 优化弹幕管理界面、统一弹幕控制台品牌图标与时间格式；HTTP API 和数据库结构均未变更。

2.1.3 移除侧栏中重复的个人资料入口，继续通过侧栏底部头像菜单进入个人资料；HTTP API 和数据库结构均未变更。

| 方法 | 路径 | 兼容行为 |
| --- | --- | --- |
| GET | `/api/danmaku/v1?id={vid}` | 返回通用弹幕数组 |
| GET | `/api/danmaku/v1/{vid}.xml` | 返回 Bilibili XML 格式 |
| POST | `/api/danmaku/v1` | 接收通用弹幕 JSON |
| GET/POST | `/api/danmaku/dplayer/v3` | DPlayer v3 查询与提交 |
| GET/POST | `/api/danmaku/artplayer/v1` | ArtPlayer 查询与提交；查询默认 XML，`.json` 返回 JSON |
| GET | `/api/danmaku/v1/bilibili/*`、`/api/danmaku/dplayer/v3/bilibili`、`/api/danmaku/artplayer/v1/bilibili/*` | 按 `cid`，或 `aid`/`bvid` + `p` 获取并转换 Bilibili 弹幕 |
| GET | `/api/other/bilibili/queryaid` | 查询 `aid`、`bvid` 和分 P 列表 |
| POST | `/api/admin/login` | 建立 `DCookie` 会话并写入 `ClientAuth` 角色 Cookie |
| GET | `/api/admin/logout` | 清理 Cookie 并以 302 跳转 `/` |
| GET | `/api/admin/auth/options` | 返回 CAS 登录选项 |
| GET | `/api/admin/session` | 从服务端 Cookie 恢复当前管理会话 |
| GET/POST | `/api/admin/user/*` | 用户资料与密码操作 |
| GET/POST | `/api/admin/users` | 管理员分页查询用户或创建本地用户；CAS 开启时不允许创建本地用户 |
| GET/PUT/DELETE | `/api/admin/users/{id}` | 管理员查询、更新或删除用户；CAS 用户只能修改角色 |
| PATCH | `/api/admin/users/{id}/status` | 管理员启用或停用用户 |
| GET | `/api/admin/danmakulist[/vids\|/dateselect\|/baseselect]` | 管理列表和筛选 |
| GET/POST | `/api/admin/danmakuedit[/delete\|/edit]` | 查询、软删除和编辑 |

客户端 IP 的取值顺序保持代理部署习惯：`X-Real-IP`、`X-Forwarded-For`、请求远端地址。提交时 JSON 未提供 `referer` 则读取 `Referer` 请求头。

## SignalR

端点为 `/api/live/danmaku`，支持 WebSocket 和 Server-Sent Events：

- `Connection(group)`：加入指定组。
- `SendMessage(group, user, message)`：向同组内除发送者以外的客户端触发 `ReceiveMessage(user, message)`。

## PostgreSQL

Go 后端启动时在单个事务中迁移并读写以下区分大小写对象：

- 表：`"Danmaku"`、`"Video"`、`"User"`、`"HttpClientCache"`。
- 软删除列：`"Danmaku"."IsDelete"`。
- 视频外键：`"Danmaku"."VideoId"` -> `"Video"."Id"`。
- 弹幕 JSONB：`"Danmaku"."Data"`，内部属性继续写为 `Time`、`Mode`、`Size`、`Color`、`TimeStamp`、`Pool`、`Author`、`AuthorId`、`Text`。
- 来源 JSONB：`"Video"."Referer"`，内部属性继续写为 `Protocol`、`Host`、`Port`、`Path`、`Query`、`Fragment`。

数据库读取同时兼容既有 PascalCase JSONB 和可能存在的 camelCase 数据，但新增数据仍写成旧 EF Core 的 PascalCase 结构。

已有 `"Danmu"` 表、`FK_Danmu_*` 外键和 `IX_Danmu_*` 索引会自动改为 `Danmaku` 命名。`"User"` 会新增 `"CASSubject"`、`"CASDisplayName"`、`"CASAvatar"`、`"Enabled"` 字段及唯一索引，用于稳定绑定、同步 CAS 资料及停用账户。结构检查在服务启动、开始监听前自动完成。

用户角色 `1` 为管理员，可管理弹幕和用户；角色 `2` 为弹幕管理员，只能管理弹幕；角色 `3` 为普通用户，只能查看自己的资料。CAS 启用后，CAS 返回的用户名、显示名、邮箱和头像会在登录时同步；包括电话在内的本地资料及密码均不可由本项目修改，只允许管理员调整角色或启停账户。
