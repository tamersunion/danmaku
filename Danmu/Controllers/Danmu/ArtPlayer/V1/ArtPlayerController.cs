using System;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.Danmaku.BiliBili;
using Danmaku.Model.Danmaku.DanmakuData;
using Danmaku.Model.DataTable;
using Danmaku.Model.WebResult;
using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Danmaku.ArtPlayer.V1
{
    [Route("/api/danmaku/artplayer/v1")]
    [FormatFilter]
    public class ArtPlayerController : DanmakuBaseController
    {
        public ArtPlayerController(DanmakuDao danmakuDao, VideoDao videoDao) : base(danmakuDao, videoDao) { }

        [HttpGet]
        [HttpGet("{id}.{format?}")]
        public async Task<dynamic> Get(string id, string format)
        {
            id ??= Request.Query["id"];
            if (string.IsNullOrEmpty(id)) return new WebResult(1);

            var result = await DanmakuDao.QueryDanmakusByVidAsync(id);

            if (!string.IsNullOrEmpty(format) && format.Equals("json"))
            {
                var aData = result.Select(s => (ArtPlayerDanmakuData) s).ToArray();
                return new WebResult<ArtPlayerDanmakuData[]>(aData);
            }

            if (string.IsNullOrEmpty(format)) HttpContext.Request.Headers["Accept"] = "application/xml";

            return (DanmakuDataBiliBili) result;
        }

        [HttpPost]
        public async Task<WebResult> Post([FromBody] ArtPlayerDanmakuDataIn data)
        {
            if (string.IsNullOrWhiteSpace(data.Id) || string.IsNullOrWhiteSpace(data.Text))
                return new WebResult(1);
            data.Ip = IPAddress.TryParse(Request.Headers["X-Real-IP"], out var ip)
                    ? ip
                    : Request.HttpContext.Connection.RemoteIpAddress;
            data.Referer ??= Request.Headers["Referer"].FirstOrDefault();

            var video = await VideoDao.InsertAsync(data.Id, new Uri(data.Referer));
            var danmaku = new DanmakuTable
            {
                Vid = data.Id,
                Data = data.ToBaseDanmakuData(),
                Ip = data.Ip,
                Video = video
            };
            var result = await DanmakuDao.InsertDanmakuAsync(danmaku);
            return new WebResult(result ? 0 : 1);
        }
    }
}
