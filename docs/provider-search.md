# 第三方搜索与链接解析

2.11.0 控制台新增能力，原有手工 ID 创建方式继续保留。搜索与解析只读元数据，点击「创建并同步」才建立弹幕池并拉取弹幕

## 爱奇艺

新增弹幕池支持关键词搜索 → 作品 → 剧集，自动填入 VID。参考 [iqiyi.js](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/iqiyi.js)，支持电影和剧集、旧版分组和新版剧集列表，过滤非正片条目

`iqiyi_setting` 新增可配置完整地址（其余弹幕地址保持原值）

```json
"search_api_base": "https://mesh.if.iqiyi.com/portal/lw/search/homePageV3",
"episodes_api_base": "https://www.iqiyi.com/prelw/tvg/v2/lw/base_info"
```

管理 GET：`/api/admin/iqiyi/search?keyword=...`、`/api/admin/iqiyi/anime/{mediaId}/episodes`

## bilibili

支持普通视频、番剧、影视关键词搜索，选作品后选择分 P 或剧集。搜索和元数据使用现有 `bilibili_setting.api_base`、`cookie`；WBI 签名密钥自动获取并缓存

新增弹幕池的 BVID 输入框同时接受以下输入，可先点击「解析链接 / 标识」选择分 P 或剧集

- 裸 `BV...`、`av...`、`ep...`、`ss...`
- `www.bilibili.com/video/{BV或av}`，支持 `p` 分 P 参数
- `www.bilibili.com/bangumi/play/{ep或ss}`
- 移动站链接、播放器嵌入链接（`aid`/`bvid`、`page`/`p`）
- `b23.tv`、`bili2233.cn` 短链接、包含链接的分享文案
- `bilibili://video/{aid}`、`bilibili://bangumi/season/{id}`、`bilibili://bangumi/episode/{id}`

短链接重定向仅允许受支持的 B 站域名，限制跳转次数，不携带配置 Cookie。失效、登录受限或上游风控的链接会报告错误，不保证受限视频可解析

池仍按 CID 去重保存，AID 从 BVID 实时计算，不落库。SS 解析提供剧集选择；直接提交 SS 且未选择时使用首集，EP 定位指定剧集

管理 GET：`/api/admin/bilibili/search?keyword=...&type=video`（另支持 `media_bangumi`、`media_ft`）、`/api/admin/bilibili/resolve?input=...`

管理创建接口新增可选 `url` 输入，与 BVID/AID/CID 互斥；BVID 字段也接受上述链接。公开读取接口参数保持原有约定

参考 [bilibili.js](https://github.com/huangxd-/danmu_api/blob/main/danmu_api/sources/bilibili.js)，不引入跨平台替代来源
