using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Cors;
using Microsoft.AspNetCore.Mvc;
using static Danmaku.Utils.Global.VariableDictionary;

namespace Danmaku.Controllers.Base
{
    [ApiController]
    [EnableCors(AdminAllowSpecificOrigins)]
    [Authorize(Policy = AdminRolePolicy)]
    public class AdminBaseController : ControllerBase
    {
        private protected DanmakuDao DanmakuDao;
        private protected VideoDao VideoDao;

        protected AdminBaseController(DanmakuDao danmakuDao, VideoDao videoDao)
        {
            DanmakuDao = danmakuDao;
            VideoDao = videoDao;
        }
    }
}
