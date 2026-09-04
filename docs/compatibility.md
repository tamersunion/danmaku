# Go 后端兼容说明

## HTTP API

所有 JSON 响应继续使用 `{ "code": number, "data": any }`。业务成功为 `code: 0`，业务失败为 `code: 1`，管理接口未登录或无管理员权限为 `code: 401`。

| 方法 | 路径 | 兼容行为 |
| --- | --- | --- |
| GET | `/api/danmu/v1?id={vid}` | 返回通用弹幕数组 |
| GET | `/api/danmu/v1/{vid}.xml` | 返回 Bilibili XML 格式 |
| POST | `/api/danmu/v1` | 接收通用弹幕 JSON |
| GET/POST | `/api/danmu/dplayer/v3` | DPlayer v3 查询与提交 |
| GET/POST | `/api/danmu/artplayer/v1` | ArtPlayer 查询与提交；查询默认 XML，`.json` 返回 JSON |
| GET | `/api/danmu/v1/bilibili/*`、`/api/danmu/dplayer/v3/bilibili`、`/api/danmu/artplayer/v1/bilibili/*` | 按 `cid`，或 `aid`/`bvid` + `p` 获取并转换 Bilibili 弹幕 |
| GET | `/api/other/bilibili/queryaid` | 查询 `aid`、`bvid` 和分 P 列表 |
| POST | `/api/admin/login` | 建立 `DCookie` 会话并写入 `ClientAuth` 角色 Cookie |
| GET | `/api/admin/logout` | 清理 Cookie 并以 302 跳转 `/` |
| GET/POST | `/api/admin/user/*` | 用户资料与密码操作 |
| GET | `/api/admin/danmulist[/vids\|/dateselect\|/baseselect]` | 管理列表和筛选 |
| GET/POST | `/api/admin/danmuedit[/delete\|/edit]` | 查询、软删除和编辑 |

客户端 IP 的取值顺序保持代理部署习惯：`X-Real-IP`、`X-Forwarded-For`、请求远端地址。提交时 JSON 未提供 `referer` 则读取 `Referer` 请求头。

## SignalR

端点保持为 `/api/live/danmu`，支持 WebSocket 和 Server-Sent Events：

- `Connection(group)`：加入指定组。
- `SendMessage(group, user, message)`：向同组内除发送者以外的客户端触发 `ReceiveMessage(user, message)`。

## PostgreSQL

Go 后端继续读写原 EF Core 创建的区分大小写对象：

- 表：`"Danmu"`、`"Video"`、`"User"`、`"HttpClientCache"`。
- 软删除列：`"Danmu"."IsDelete"`。
- 视频外键：`"Danmu"."VideoId"` -> `"Video"."Id"`。
- 弹幕 JSONB：`"Danmu"."Data"`，内部属性继续写为 `Time`、`Mode`、`Size`、`Color`、`TimeStamp`、`Pool`、`Author`、`AuthorId`、`Text`。
- 来源 JSONB：`"Video"."Referer"`，内部属性继续写为 `Protocol`、`Host`、`Port`、`Path`、`Query`、`Fragment`。

数据库读取同时兼容既有 PascalCase JSONB 和可能存在的 camelCase 数据，但新增数据仍写成旧 EF Core 的 PascalCase 结构。
