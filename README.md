# Danmaku Server

通用弹幕服务器的 Go 重构版，支持 DPlayer、ArtPlayer、通用弹幕格式、Bilibili 弹幕代理和管理后台。

本次重构统一使用 `danmaku` 命名。HTTP、SignalR、管理接口和 PostgreSQL 对象中原先带 `danmu` 的名称均迁移为 `danmaku`。

## 运行

要求 Go 1.25 或更高版本和 PostgreSQL 13 或更高版本。

```bash
go run ./cmd/danmaku -config appsettings.yml
```

程序每次启动都会在事务中检查数据库结构。已有 `"Danmu"` 表会自动重命名为 `"Danmaku"`，相关外键和索引同步重命名，并为 `"User"` 补齐 CAS 账户字段；2.2.0 起还会自动创建 Bilibili 弹幕池、关键词和视频关联表。迁移失败时服务不会开始监听。若新旧表同时存在，程序会中止并要求人工确认，避免错误合并数据。

配置文件仍使用原来的 ASP.NET 风格层级，数据库节点是 `DanmakuSql`，Bilibili 上游由 `BiliBiliSetting.ApiBase` 配置，默认为 `https://api.bilibili.com`。Bilibili 持久化弹幕池采用固定 10 分钟缓存窗口：窗口内只返回数据库缓存，超过窗口后由下一次外部请求被动执行增量更新，后台“立即更新”可绕过窗口；服务不会定时主动刷新。`DanmakuCacheTime` 及旧的 `DanmuCacheTime` 仍保留配置读取兼容，但不改变该固定窗口。完整示例见 [appsettings.yml](appsettings.yml)，也可通过 `DANMAKU_CONFIG` 指定配置路径。

CAS 默认接管登录流程，入口为 `/cas/login`，回调为 `/cas/callback`，退出为 `/cas/logout`；旧入口 `/cas/auth` 继续作为原生 CAS 组合入口。紧急情况下可访问 `/login?skipsso=true` 使用本地账号。CAS 用户首次登录会自动建档，默认以普通用户（`CAS.DefaultRole: 3`）加入；管理员可在用户管理中提升角色。CAS 开启时，用户名、显示名、邮箱和头像以 CAS 返回内容为准，前端及 API 均禁止修改资料和密码。

权限分为三级：管理员可管理弹幕和用户，弹幕管理员只能管理弹幕，普通用户只能查看自己的资料。数据库角色值仍分别为 `1`、`2`、`3`。

## API 路径

- 通用格式：`GET/POST /api/danmaku/v1`，以及 `/{id}.{format}`
- DPlayer v3：`GET/POST /api/danmaku/dplayer/v3`
- ArtPlayer v1：`GET/POST /api/danmaku/artplayer/v1`
- Bilibili 转换：以上各格式下的 `bilibili`/`danmaku.{format}` 路由；`offset` 参数按秒调整返回时间
- 管理后台：`/api/admin/login`、`logout`、`user/*`、`users/*`、`danmakulist/*`、`danmakuedit/*`、`bilibili/*`
- 实时弹幕：SignalR `/api/live/danmaku`，方法 `Connection`、`SendMessage`，客户端事件 `ReceiveMessage`
- 辅助查询：`GET /api/other/bilibili/queryaid`

详细的方法、参数、响应和数据库映射见 [兼容说明](docs/compatibility.md)。

## 构建与测试

```bash
go test ./...
go vet ./...
go build ./cmd/danmaku
```

前端位于 `frontend`，使用与 dnsmgr-frontend 一致的 React 19、Vite、Tailwind CSS v4、shadcn/ui Base UI Nova 预设和 Geist 字体。容器构建会先生成前端静态文件，再编译 Go 单文件服务：

```bash
docker build --build-arg DANMAKU_VERSION=2.2.0 -t danmaku:2.2.0 .
docker compose up -d
```

## 原项目文档

旧版接口文档曾发布于 <https://dandoc.u2sb.top>。本仓库内的兼容说明是当前 Go 实现的基准。
