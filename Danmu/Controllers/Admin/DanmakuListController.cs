using System.Threading.Tasks;
using Danmaku.Controllers.Base;
using Danmaku.Model.DataTable;
using Danmaku.Model.WebResult;
using Danmaku.Utils.Dao;
using Microsoft.AspNetCore.Mvc;

namespace Danmaku.Controllers.Admin
{
    [Route("/api/admin/danmakulist")]
    public class DanmakuListController : AdminBaseController
    {
        public DanmakuListController(DanmakuDao danmakuDao, VideoDao videoDao) : base(danmakuDao, videoDao) { }

        /// <summary>
        ///     获取弹幕
        /// </summary>
        /// <returns></returns>
        [HttpGet]
        public async Task<DanmakuListWebResult<DanmakuTable>> GetDanmakuList(string vid = null, int page = 1, int size = 30,
                                                                       bool descending = true)
        {
            var total = string.IsNullOrEmpty(vid)
                    ? DanmakuDao.GetAllDanmakuAsync()
                    : DanmakuDao.GetDanmakuByVidAsync(vid);

            var danmaku = string.IsNullOrEmpty(vid)
                    ? DanmakuDao.GetAllDanmakuAsync(page, size, descending)
                    : DanmakuDao.GetDanmakuByVidAsync(vid, page, size, descending);

            return new DanmakuListWebResult<DanmakuTable>(await total, await danmaku);
        }

        /// <summary>
        ///     获取vid集合
        /// </summary>
        /// <returns></returns>
        [HttpGet("vids")]
        public async Task<WebResult<string[]>> GetVidList()
        {
            var vids = await VideoDao.GetVidsAsync();
            return new WebResult<string[]>(vids);
        }

        /// <summary>
        ///     日期筛选
        /// </summary>
        /// <param name="page"></param>
        /// <param name="size"></param>
        /// <param name="startDate"></param>
        /// <param name="endDate"></param>
        /// <param name="descending"></param>
        /// <returns></returns>
        [HttpGet("date" + "select")]
        public async Task<DanmakuListWebResult<DanmakuTable>> DateSelect(int page = 1, int size = 30,
                                                                     string startDate = null,
                                                                     string endDate = null, bool descending = true)
        {
            var result = DanmakuDao.DateSelectAsync(page, size, startDate, endDate);
            return new DanmakuListWebResult<DanmakuTable>(0)
            {
                Data = await result
            };
        }

        /// <summary>
        ///     基础查询
        /// </summary>
        /// <returns></returns>
        [HttpGet("base" + "select")]
        public async Task<DanmakuListWebResult<DanmakuTable>> DanmakuBasesSelect(
                int page = 1, int size = 30, string vid = null,
                string author = null, string authorId = null,
                string startDate = null,
                string endDate = null, int mode = 100,
                string ip = null, string key = null,
                bool descending = true)
        {
            var iAuthorId = int.TryParse(authorId, out var uid) ? uid : -1;
            var result = DanmakuDao.DanmakuBasesSelectAsync(page, size, vid, author, iAuthorId, startDate, endDate,
                    mode,
                    ip,
                    key, descending);
            return new DanmakuListWebResult<DanmakuTable>(0)
            {
                Data = await result
            };
        }
    }
}
