using System.ComponentModel.DataAnnotations;
using System.Net;

namespace Danmaku.Model.Danmaku.DanmakuData
{
    public class DplayerDanmakuData : IDanmakuData
    {
        /// <summary>
        ///     弹幕时间
        /// </summary>
        public float Time { get; set; }

        /// <summary>
        ///     弹幕类型
        /// </summary>
        public int Type { get; set; }

        /// <summary>
        ///     弹幕颜色
        /// </summary>
        public int Color { get; set; }

        /// <summary>
        ///     弹幕作者
        /// </summary>
        public string Author { get; set; }

        /// <summary>
        ///     弹幕文字
        /// </summary>
        public string Text { get; set; }

        public BaseDanmakuData ToBaseDanmakuData()
        {
            var isId = int.TryParse(Author, out var authorId);
            return new BaseDanmakuData
            {
                Time = Time,
                Mode = Type == 1 ? 5 : Type == 2 ? 4 : 1,
                Color = Color,
                AuthorId = isId ? authorId : 0,
                Author = isId ? "" : Author,
                Text = Text
            };
        }

        public static explicit operator DplayerDanmakuData(BaseDanmakuData data)
        {
            var t = data.Mode;
            switch (t)
            {
                case 4:
                    t = 2;
                    break;
                case 5:
                    t = 1;
                    break;
                case 7:
                    t = 0;
                    data.Text = data.Text.Split(",")[4];
                    break;
                case 8:
                    t = 0;
                    data.Text = null;
                    break;
                default:
                    t = 0;
                    break;
            }

            return new DplayerDanmakuData
            {
                Time = data.Time,
                Type = t,
                Color = data.Color,
                Author = string.IsNullOrEmpty(data.Author) ? data.AuthorId.ToString() : data.Author,
                Text = data.Text
            };
        }
    }

    public class DplayerDanmakuDataIn : DplayerDanmakuData
    {
        /// <summary>
        ///     视频的id
        /// </summary>
        [MaxLength(36)]
        public string Id { get; set; }

        /// <summary>
        ///     评论者ip
        /// </summary>
        public IPAddress Ip { get; set; }

        /// <summary>
        ///     来源
        /// </summary>
        public string Referer { get; set; }
    }
}
