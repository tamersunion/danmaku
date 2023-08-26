using Danmaku.Model.Config;
using Danmaku.Utils.Configuration;
using Microsoft.EntityFrameworkCore;

namespace Danmaku.Model.DbContext
{
    public class BaseContext : Microsoft.EntityFrameworkCore.DbContext
    {
        private protected readonly DanmakuSql Sql;

        public BaseContext(DbContextOptions options) : base(options)
        {
            Sql = AppConfiguration.AppSettings.DanmakuSql;
        }
    }
}
