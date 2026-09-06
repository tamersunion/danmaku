package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Explicitly opt in with a disposable database; never use deployment settings.
func TestCatalogPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("DANMAKU_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("set DANMAKU_TEST_POSTGRES to an isolated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	id, err := randomUUID()
	if err != nil {
		t.Fatal(err)
	}
	schema := "danmaku_catalog_test_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := &Postgres{pool: pool, cacheTTL: time.Hour}
	for range 2 {
		if err := repo.Initialize(ctx); err != nil {
			t.Fatal(err)
		}
	}

	for _, source := range []string{"bahamut", "tencent", "youku"} {
		t.Run(source, func(t *testing.T) {
			catalog := repo.Catalog(source)
			video, err := repo.CreateVideo(ctx, source+"-video", "example")
			if err != nil {
				t.Fatal(err)
			}
			sourcePool, err := catalog.EnsureCatalogPool(ctx, "123")
			if err != nil {
				t.Fatal(err)
			}
			again, err := catalog.EnsureCatalogPool(ctx, "123")
			if err != nil || again.ID != sourcePool.ID {
				t.Fatal("duplicate pool")
			}
			claim, err := catalog.ClaimCatalogPoolSync(ctx, sourcePool.ID, time.Minute, false)
			if err != nil || !claim {
				t.Fatal("sync claim")
			}
			claim, err = catalog.ClaimCatalogPoolSync(ctx, sourcePool.ID, time.Minute, false)
			if err != nil || claim {
				t.Fatal("sync window")
			}
			text, keep := "spam", "keep"
			data := []domain.DanmakuData{{Time: 1, Text: &text}, {Time: 2, Text: &keep}}
			n, err := catalog.MergeCatalogDanmaku(ctx, sourcePool.ID, data)
			if err != nil || n != 2 {
				t.Fatalf("merge %d %v", n, err)
			}
			n, err = catalog.MergeCatalogDanmaku(ctx, sourcePool.ID, data)
			if err != nil || n != 0 {
				t.Fatal("dedup")
			}
			if _, err = catalog.MergeCatalogDanmaku(ctx, sourcePool.ID, nil); err != nil {
				t.Fatal(err)
			}
			binding, err := catalog.UpsertVideoCatalogBinding(ctx, video.ID, sourcePool.ID, 2)
			if err != nil || binding == nil {
				t.Fatalf("bind %v", err)
			}
			detail, err := repo.Video(ctx, video.ID)
			if err != nil || detail.CatalogPoolCount != 1 || detail.ThirdPartyDanmakuCount != 2 || len(detail.CatalogBindings) != 1 || detail.CatalogBindings[0].Source != source {
				t.Fatalf("video=%v err=%v", detail, err)
			}
			sizes, err := repo.ThirdPartyPoolSizes(ctx, video.Vid)
			if err != nil || sizes[source+":1"] != 2 {
				t.Fatalf("sizes %v %v", sizes, err)
			}
			rule, err := catalog.CreateCatalogKeyword(ctx, nil, "SPAM")
			if err != nil {
				t.Fatal(err)
			}
			visible, err := catalog.CatalogPoolData(ctx, sourcePool.ID)
			if err != nil || len(visible) != 1 || *visible[0].Text != "keep" {
				t.Fatalf("filter %v %v", visible, err)
			}
			_, err = catalog.DeleteCatalogKeyword(ctx, rule.ID)
			if err != nil {
				t.Fatal(err)
			}
			records, err := catalog.CatalogDanmaku(ctx, CatalogDanmakuFilter{PoolID: sourcePool.ID})
			if err != nil || records.Total != 2 {
				t.Fatal("stored rows lost")
			}
			if _, err = catalog.SetCatalogDanmakuBlocked(ctx, records.List[0].ID, true); err != nil {
				t.Fatal(err)
			}
			if _, err = catalog.MergeCatalogDanmaku(ctx, sourcePool.ID, data); err != nil {
				t.Fatal(err)
			}
			if err = repo.Initialize(ctx); err != nil {
				t.Fatal(err)
			}
			visible, err = catalog.CatalogPoolData(ctx, sourcePool.ID)
			if err != nil || len(visible) != 1 {
				t.Fatal("manual block/restart")
			}
			if _, err = catalog.DeleteVideoCatalogBinding(ctx, video.ID, binding.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Same ID in three tables remains three independent libraries.
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		pools, err := repo.Catalog(source).CatalogPools(ctx, CatalogPoolFilter{})
		if err != nil || pools.Total != 1 || pools.List[0].DanmakuCount != 2 {
			t.Fatalf("source isolation %s: %v %v", source, pools, err)
		}
	}
}
