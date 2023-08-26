using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Cors;
using Microsoft.AspNetCore.Mvc;
using static Danmaku.Utils.Global.VariableDictionary;

namespace Danmaku.Controllers.Base
{
    [ApiController]
    [EnableCors(DanmakuAllowSpecificOrigins)]
    [FormatFilter]
    public abstract class DanmakuBaseController : ControllerBase
    {
        private protected DanmakuDao DanmakuDao;
        private protected VideoDao VideoDao;

        protected DanmakuBaseController(DanmakuDao danmakuDao, VideoDao videoDao)
        {
            DanmakuDao = danmakuDao;
            VideoDao = videoDao;
        }
    }
}
