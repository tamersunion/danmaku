using Danmaku.Utils.BiliBili;
using Microsoft.AspNetCore.Cors;
using Microsoft.AspNetCore.Mvc;
using static Danmaku.Utils.Global.VariableDictionary;

namespace Danmaku.Controllers.Base
{
    [EnableCors(DanmakuAllowSpecificOrigins)]
    [FormatFilter]
    [ApiController]
    public class BiliBiliBaseController : ControllerBase
    {
        private protected readonly BiliBiliHelp Bilibili;

        public BiliBiliBaseController(BiliBiliHelp bilibili)
        {
            Bilibili = bilibili;
        }
    }
}
