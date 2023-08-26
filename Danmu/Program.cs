using System;
using System.Net;
using Danmu.Model.Config;
using Danmu.Model.DbContext;
using Danmu.Utils.Dao;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
#if LINUX
using System.IO;
#endif

namespace Danmu
{
    public class Program
    {
        private static AppSettings _appSettings = new AppSettings();

        private static void Main(string[] args)
        {
            var host = CreateHostBuilder(args).Build();
            CreateDbIfNotExists(host);
            host.Run();
        }

        private static IHostBuilder CreateHostBuilder(string[] args)
        {
            return Host.CreateDefaultBuilder(args)
                       .ConfigureAppConfiguration((context, builder) =>
                        {
                            var env = context.HostingEnvironment;
                            builder
                                   .AddJsonFile("appsettings.json", true, true)
                                   .AddYamlFile("appsettings.yml", true, true)
                                   .AddJsonFile($"appsettings.{env.EnvironmentName}.json", true)
                                   .AddJsonFile($"appsettings.{env.EnvironmentName}.yml", true);
                            _appSettings = builder.Build().Get<AppSettings>();
                        })
                       .ConfigureWebHostDefaults(webBuilder =>
                        {
                            webBuilder.ConfigureKestrel(options =>
                            {
                                var ks = _appSettings.KestrelSettings;
#if LINUX
                                if (!string.IsNullOrEmpty(ks.UnixSocketPath))
                                {
                                    if (File.Exists(ks.UnixSocketPath)) File.Delete(ks.UnixSocketPath);
                                    options.ListenUnixSocket(ks.UnixSocketPath);
                                }
#endif
                                if (ks.Port.HasValue && ks.Port.Value != 0)
                                {
                                    var host = string.IsNullOrEmpty(ks.Host) ? IPAddress.Any.ToString() : ks.Host;
                                    var port = ks.Port.Value;
                                    options.Listen(IPAddress.Loopback, port);
                                }
                            }).UseStartup<Startup>();
                        });
        }

        private static void CreateDbIfNotExists(IHost host)
        {
            using var scope = host.Services.CreateScope();
            var services = scope.ServiceProvider;
            try
            {
                var context = services.GetRequiredService<DanmuContext>();
                DbInitializer.Initialize(context, _appSettings);
            }
            catch (Exception ex)
            {
                var logger = services.GetRequiredService<ILogger<Program>>();
                logger.LogError(ex, "An error occurred creating the DB.");
            }
        }
    }
}
