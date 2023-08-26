namespace Danmaku.Model.WebResult
{
    public class DanmakuListWebResult<T> : WebResult<DanmakuList<T>>
    {
        public DanmakuListWebResult() { }
        public DanmakuListWebResult(int code) : base(code) { }

        public DanmakuListWebResult(int total, T[] list) : this(0)
        {
            Data = new DanmakuList<T>
            {
                Total = total,
                List = list
            };
        }
    }

    public class DanmakuList<T>
    {
        public int Total { get; set; }
        public T[] List { get; set; }
    }
}
