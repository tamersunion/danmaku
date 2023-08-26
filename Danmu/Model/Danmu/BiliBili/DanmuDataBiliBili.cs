using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.IO;
using System.Linq;
using System.Xml.Serialization;
using Danmaku.Model.Danmaku.DanmakuData;

namespace Danmaku.Model.Danmaku.BiliBili
{
    [Serializable]
    [DesignerCategory("code")]
    [XmlType(AnonymousType = true)]
    [XmlRoot("i", Namespace = "", IsNullable = false)]
    public class DanmakuDataBiliBili
    {
        public DanmakuDataBiliBili() { }

        public DanmakuDataBiliBili(Stream s)
        {
            var serializer = new XmlSerializer(typeof(DanmakuDataBiliBili));
            var bd = (DanmakuDataBiliBili) serializer.Deserialize(s);
            D = bd.D;
        }

        [XmlElement("d")] public iD[] D { get; set; }

        public static explicit operator DanmakuDataBiliBili(BaseDanmakuData[] data)
        {
            var d = data.Select(s => new iD
            {
                P = $"{s.Time},{s.Mode},{s.Size},{s.Color},{s.TimeStamp},{s.Pool},{s.Author},{s.TimeStamp}",
                Value = s.Text
            }).ToArray();
            return new DanmakuDataBiliBili {D = d};
        }

        public IEnumerable<BaseDanmakuData> ToDanmakuDataBases()
        {
            if (D == null || D.Length == 0) return new BaseDanmakuData[0];
            return D.Select(s =>
            {
                var d = s.P.Split(",");
                return new BaseDanmakuData
                {
                    Time = float.Parse(d[0]),
                    Mode = int.Parse(d[1]),
                    Size = int.Parse(d[2]),
                    Color = int.Parse(d[3]),
                    TimeStamp = long.Parse(d[4]),
                    Pool = int.Parse(d[5]),
                    Author = d[6],
                    Text = s.Value
                };
            });
        }
    }

    [Serializable]
    [DesignerCategory("code")]
    [XmlType(AnonymousType = true)]
    // ReSharper disable once InconsistentNaming
    public class iD
    {
        [XmlAttribute("p")] public string P { get; set; }

        [XmlText] public string Value { get; set; }
    }
}
