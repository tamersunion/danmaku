# Danmaku Server

通用弹幕服务器的 Go 重构版，支持 DPlayer、ArtPlayer、通用弹幕格式、Bilibili 弹幕代理和管理后台。

本次重构统一使用 `danmaku` 命名。HTTP、SignalR、管理接口和 PostgreSQL 对象中原先带 `danmu` 的名称均迁移为 `danmaku`。

## 运行

要求 Go 1.25 或更高版本和 PostgreSQL 13 或更高版本。

```bash
go run ./cmd/danmaku -config appsettings.yml
```

程序每次启动都会在事务中检查数据库结构。已有 `"Danmu"` 表会自动重命名为 `"Danmaku"`，相关外键和索引同步重命名，并为 `"User"` 补齐 CAS 账户字段；迁移失败时服务不会开始监听。若新旧表同时存在，程序会中止并要求人工确认，避免错误合并数据。

配置文件仍使用原来的 ASP.NET 风格层级，数据库节点是 `DanmakuSql`，Bilibili 弹幕缓存项是 `DanmakuCacheTime`。升级期间仍可读取旧的 `DanmuSql` 和 `DanmuCacheTime` 配置，加载后会归一到新名称。完整示例见 [appsettings.yml](appsettings.yml)，也可通过 `DANMAKU_CONFIG` 指定配置路径。

CAS 默认接管登录流程，入口为 `/cas/login`，回调为 `/cas/callback`，退出为 `/cas/logout`；旧入口 `/cas/auth` 继续作为原生 CAS 组合入口。紧急情况下可访问 `/login?skipsso=true` 使用本地账号。CAS 用户首次登录会自动建档，角色由 `CAS.DefaultRole` 决定。

## API 路径

- 通用格式：`GET/POST /api/danmaku/v1`，以及 `/{id}.{format}`
- DPlayer v3：`GET/POST /api/danmaku/dplayer/v3`
- ArtPlayer v1：`GET/POST /api/danmaku/artplayer/v1`
- Bilibili 转换：以上各格式下的 `bilibili`/`danmaku.{format}` 路由
- 管理后台：`/api/admin/login`、`logout`、`user/*`、`danmakulist/*`、`danmakuedit/*`
- 实时弹幕：SignalR `/api/live/danmaku`，方法 `Connection`、`SendMessage`，客户端事件 `ReceiveMessage`
- 辅助查询：`GET /api/other/bilibili/queryaid`

详细的方法、参数、响应和数据库映射见 [兼容说明](docs/compatibility.md)。

## 构建与测试

```bash
go test ./...
go vet ./...
go build ./cmd/danmaku
```

前端位于 `frontend`。容器构建会先生成 Vue 静态文件，再编译 Go 单文件服务：

```bash
docker build --build-arg DANMAKU_VERSION=2.0.2 -t danmaku:2.0.2 .
docker compose up -d
```

## 原项目文档

旧版接口文档曾发布于 <https://dandoc.u2sb.top>。本仓库内的兼容说明是当前 Go 实现的基准。
