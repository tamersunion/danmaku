using System.Linq;
using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.Danmaku.BiliBili;
using Danmaku.Model.WebResult;
using Danmaku.Utils.BiliBili;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Danmaku.Dplayer.V3
{
    [Route("/api/danmaku/dplayer/v3")]
    public class BiliBiliController : BiliBiliBaseController
    {
        public BiliBiliController(BiliBiliHelp bilibili) : base(bilibili) { }

        [HttpGet("bilibili")]
        public async Task<DplayerWebResult> Get([FromQuery] BiliBiliQuery query)
        {
            HttpContext.Request.Headers["Accept"] = "application/json";
            var result = await Bilibili.GetDanmakuAsync(query);
            return new DplayerWebResult(result.ToDanmakuDataBases().ToArray());
        }
    }
}
