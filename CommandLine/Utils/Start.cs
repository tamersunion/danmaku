using System;

namespace Danmaku.CommandLine.Utils
{
    public class Start
    {
        public static void StartCommandLine()
        {
            Console.WriteLine(Environment.Is64BitProcess);
        }
    }
}