using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.Danmaku.BiliBili;
using Danmaku.Model.Danmaku.DanmakuData;
using Danmaku.Model.WebResult;
using Danmaku.Utils.BiliBili;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Danmaku.ArtPlayer.V1
{
    [Route("/api/danmaku/artplayer/v1/bilibili")]
    public class BiliBiliController : BiliBiliBaseController
    {
        public BiliBiliController(BiliBiliHelp bilibili) : base(bilibili) { }

        [HttpGet]
        [HttpGet("danmaku")]
        [HttpGet("danmaku.{format}")]
        public async Task<dynamic> Get([FromQuery] BiliBiliQuery query, string format)
        {
            if (query.Date.Length == 0 && !(!string.IsNullOrEmpty(format) && format.Equals("json")))
            {
                HttpContext.Response.ContentType = "application/xml; charset=utf-8";
                return await Bilibili.GetDanmakuRawByQueryAsync(query);
            }

            var danmaku = await Bilibili.GetDanmakuAsync(query);

            if (!string.IsNullOrEmpty(format) && format.Equals("json"))
                return new WebResult<IEnumerable<ArtPlayerDanmakuData>>(danmaku
                                                                     .ToDanmakuDataBases()
                                                                     .Select(s => (ArtPlayerDanmakuData) s));
            if (string.IsNullOrEmpty(format)) HttpContext.Request.Headers["Accept"] = "application/xml";
            return danmaku;
        }
    }
}
