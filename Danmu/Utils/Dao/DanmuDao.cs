using System.Linq;
using System.Threading.Tasks;
using Danmaku.Model.Danmaku.DanmakuData;
using Danmaku.Model.DataTable;
using Danmaku.Model.DbContext;
using Microsoft.EntityFrameworkCore;

namespace Danmaku.Utils.Dao
{
    public partial class DanmakuDao
    {
        private readonly DanmakuContext _con;

        public DanmakuDao(DanmakuContext con)
        {
            _con = con;
        }

        /// <summary>
        ///     通过视频Vid查询弹幕
        /// </summary>
        /// <param name="vid">视频vid</param>
        /// <returns>通用弹幕列表</returns>
        public async Task<BaseDanmakuData[]> QueryDanmakusByVidAsync(string vid)
        {
            return await _con.Danmaku.AsNoTracking().Where(e => e.Vid.Equals(vid) && !e.IsDelete).Select(s => s.Data).ToArrayAsync();
        }

        /// <summary>
        ///     插入弹幕
        /// </summary>
        /// <param name="danmaku">弹幕信息</param>
        /// <returns>是否成功</returns>
        public async Task<bool> InsertDanmakuAsync(DanmakuTable danmaku)
        {
            await _con.Danmaku.AddAsync(danmaku);
            return await _con.SaveChangesAsync() > 0;
        }
    }
}
