using Danmaku.Model.Config;
using Microsoft.Extensions.Configuration;

namespace Danmaku.Utils.Configuration
{
    public class AppConfiguration
    {
        internal static AppSettings AppSettings;
        private readonly AppSettings _appConfiguration;

        public AppConfiguration(IConfiguration configuration)
        {
            _appConfiguration = new AppSettings(configuration);
            AppSettings = _appConfiguration;
        }

        public AppSettings GetAppSetting()
        {
            return _appConfiguration;
        }
    }
}
