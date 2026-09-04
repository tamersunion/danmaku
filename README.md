# Danmaku Server

通用弹幕服务器的 Go 重构版，支持 DPlayer、ArtPlayer、通用弹幕格式、Bilibili 弹幕代理和管理后台。

本次重构保持原有 HTTP/SignalR API 与 PostgreSQL 数据库结构不变。Go 代码内部统一使用 `danmaku`；为兼容现有客户端与数据库，仅对外路由中的 `/api/danmu/...`、数据库表 `"Danmu"` 及其既有列名继续保留原拼写。

## 运行

要求 Go 1.25 或更高版本和 PostgreSQL 13 或更高版本。

```bash
go run ./cmd/danmaku -config appsettings.yml
```

程序启动时会检查并补齐原有的四张表：`"Danmu"`、`"Video"`、`"User"`、`"HttpClientCache"`。不会重命名既有表、列或 JSONB 属性。

配置文件仍使用原来的 ASP.NET 风格层级，但内部命名已调整：数据库节点是 `DanmakuSql`，Bilibili 弹幕缓存项是 `DanmakuCacheTime`。完整示例见 [appsettings.yml](appsettings.yml)。也可通过 `DANMAKU_CONFIG` 指定配置路径。

## API 兼容范围

- 通用格式：`GET/POST /api/danmu/v1`，以及 `/{id}.{format}`
- DPlayer v3：`GET/POST /api/danmu/dplayer/v3`
- ArtPlayer v1：`GET/POST /api/danmu/artplayer/v1`
- Bilibili 转换：以上各格式下的 `bilibili`/`danmu.{format}` 路由
- 管理后台：`/api/admin/login`、`logout`、`user/*`、`danmulist/*`、`danmuedit/*`
- 实时弹幕：SignalR `/api/live/danmu`，方法 `Connection`、`SendMessage`，客户端事件 `ReceiveMessage`
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
docker build --build-arg DANMAKU_VERSION=2.0.0 -t danmaku:2.0.0 .
docker compose up -d
```

## 原项目文档

旧版接口文档曾发布于 <https://dandoc.u2sb.top>。本仓库内的兼容说明是当前 Go 实现的基准。
