using Microsoft.Extensions.Configuration;

namespace Danmaku.Model.Config
{
    public class AppSettings
    {
        public AppSettings() { }

        public AppSettings(IConfiguration configuration)
        {
            configuration.Bind(this);
        }

        public KestrelSettings KestrelSettings { get; set; } = new KestrelSettings();
        public string[] WithOrigins { get; set; }
        public string[] LiveWithOrigins { get; set; }
        public string[] AdminWithOrigins { get; set; }
        public DanmakuSql DanmakuSql { get; set; } = new DanmakuSql();
        public Admin Admin { get; set; } = new Admin();
        public BiliBiliSetting BiliBiliSetting { get; set; } = new BiliBiliSetting();
    }

    public class DanmakuSql
    {
        public string Host { get; set; } = "127.0.0.1";
        public int Port { get; set; } = 0;
        public string UserName { get; set; }
        public string PassWord { get; set; }
        public string DataBase { get; set; }
        public int PoolSize { get; set; } = 8;
    }

    public class Admin
    {
        public string User { get; set; }
        public string Password { get; set; }
        public int MaxAge { get; set; } = 1;
    }

    public class BiliBiliSetting
    {
        public string Cookie { get; set; }
        public int CidCacheTime { get; set; } = 72;
        public int DanmakuCacheTime { get; set; } = 5;
    }
}
