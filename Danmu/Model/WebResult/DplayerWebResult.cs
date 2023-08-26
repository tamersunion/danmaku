using System.Linq;
using System.Web;
using Danmaku.Model.Danmaku.DanmakuData;

namespace Danmaku.Model.WebResult
{
    public class DplayerWebResult : WebResult<object[][]>
    {
        public DplayerWebResult() { }
        public DplayerWebResult(int code) : base(code) { }

        public DplayerWebResult(BaseDanmakuData[] data) : this(0)
        {
            Data = data.Select(s =>
            {
                var d = (DplayerDanmakuData) s;
                return new object[]
                {
                    d.Time, d.Type, d.Color, HttpUtility.HtmlEncode(d.Author), HttpUtility.HtmlEncode(d.Text)
                };
            }).ToArray();
        }
    }
}
