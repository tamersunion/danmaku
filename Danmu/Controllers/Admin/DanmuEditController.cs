using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.DataTable;
using Danmaku.Model.WebResult;
using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Admin
{
    [Route("/api/admin/danmakuedit/")]
    public class DanmakuEditController : AdminBaseController
    {
        public DanmakuEditController(DanmakuDao danmakuDao, VideoDao videoDao) : base(danmakuDao, videoDao) { }

        [HttpGet()]
        public async Task<WebResult<DanmakuTable>> Get(string id)
        {
            var danmaku = await DanmakuDao.QueryDanmakuByIdAsync(id);
            return new WebResult<DanmakuTable>(danmaku);
        }

        [HttpPost("edit")]
        public async Task<WebResult<DanmakuTable>> EditDanmaku(DanmakuTable data)
        {
            var result = await DanmakuDao.EditDanmakuAsync(data.Id, data.Data.Time, data.Data.Mode, data.Data.Color,
                    data.Data.Text, data.IsDelete);
            return new WebResult<DanmakuTable>(result);
        }

        [HttpGet("delete")]
        public async Task<WebResult> DeleteDanmaku(string id)
        {
            var result = await DanmakuDao.DeleteDanmakuAsync(id);
            return new WebResult(result ? 0 : 1);
        }
    }
}
