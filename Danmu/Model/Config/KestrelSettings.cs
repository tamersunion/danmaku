namespace Danmaku.Model.Config
{
    public class KestrelSettings
    {
        /// <summary>
        ///     监听的IP地址
        /// </summary>
        public string Host { get; set; }

        /// <summary>
        ///     监听的端口
        /// </summary>
        public int Port { get; set; }

        /// <summary>
        ///     UnixSocketPath
        /// </summary>
        public string UnixSocketPath { get; set; }
    }
}
