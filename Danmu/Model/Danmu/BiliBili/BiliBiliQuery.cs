namespace Danmu.Model.Danmu.BiliBili
{
    public class BiliBiliQuery
    {
        public long Cid { get; set; }
        public int Aid { get; set; }
        public string Bvid { get; set; }
        public int P { get; set; }
        public string[] Date { get; set; } = new string[0];
    }
}
