using System;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.Danmaku.DanmakuData;
using Danmaku.Model.DataTable;
using Danmaku.Model.WebResult;
using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Danmaku.Dplayer.V3
{
    [Route("/api/danmaku/dplayer/v3")]
    public class DplayerController : DanmakuBaseController
    {
        public DplayerController(DanmakuDao danmakuDao, VideoDao videoDao) : base(danmakuDao, videoDao) { }

        // GET: api/dplayer/v3/
        [HttpGet]
        public async Task<DplayerWebResult> Get(string id)
        {
            id ??= Request.Query["id"];
            return string.IsNullOrEmpty(id)
                    ? new DplayerWebResult(1)
                    : new DplayerWebResult(await DanmakuDao.QueryDanmakusByVidAsync(id));
        }

        [HttpPost]
        public async Task<WebResult> Post([FromBody] DplayerDanmakuDataIn data)
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
