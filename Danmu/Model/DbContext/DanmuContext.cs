using System.Threading.Tasks;
using Danmaku.Model.DataTable;
using Microsoft.EntityFrameworkCore;

namespace Danmaku.Model.DbContext
{
    public class DanmakuContext : BaseContext
    {
        public DanmakuContext(DbContextOptions<DanmakuContext> options) : base(options) { }

        public DbSet<DanmakuTable> Danmaku { get; set; }
        public DbSet<UserTable> User { get; set; }
        public DbSet<VideoTable> Video { get; set; }
        public DbSet<HttpClientCacheTable> HttpClientCache { get; set; }

        protected override void OnModelCreating(ModelBuilder modelBuilder)
        {
            modelBuilder.Entity<DanmakuTable>().Property(p => p.IsDelete).HasDefaultValue(false);
            modelBuilder.Entity<DanmakuTable>().HasIndex(d => new {d.Vid, d.IsDelete});

            modelBuilder.Entity<HttpClientCacheTable>().HasIndex(h => h.Key).HasMethod("hash");
        }

        public async Task<int> ClearTable(string tableName)
        {
            return await Database.ExecuteSqlRawAsync($"TRUNCATE \"{tableName}\" RESTART IDENTITY;");
        }
    }
}
