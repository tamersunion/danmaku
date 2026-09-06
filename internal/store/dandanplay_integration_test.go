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
func TestDandanplayPostgresIntegration(t *testing.T) {
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
	schema := "danmaku_ddp_test_" + strings.ReplaceAll(id, "-", "")
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
	// Simulate 2.10.0 with an existing pool and the episode-only unique index.
	for _, sql := range schemaStatements() {
		if !strings.Contains(sql, "Dandanplay") {
			if _, err := pool.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sql := range schemaStatements() {
		if strings.HasPrefix(sql, `CREATE TABLE IF NOT EXISTS "DandanplayDanmakuPool"`) {
			if _, err := pool.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := pool.Exec(ctx, `CREATE UNIQUE INDEX "UX_DandanplayDanmakuPool_EpisodeId" ON "DandanplayDanmakuPool" ("EpisodeId")`); err != nil {
		t.Fatal(err)
	}
	var oldPoolID int
	if err := pool.QueryRow(ctx, `INSERT INTO "DandanplayDanmakuPool" ("EpisodeId","CreateTime","UpdateTime") VALUES ('123',NOW(),NOW()) RETURNING "Id"`).Scan(&oldPoolID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := repo.Initialize(ctx); err != nil {
			t.Fatal(err)
		}
	}
	video, err := repo.CreateVideo(ctx, "ddp-video", "example")
	if err != nil {
		t.Fatal(err)
	}
	source, err := repo.EnsureDandanplayPool(ctx, "00123", true)
	if err != nil || source.EpisodeID != "123" || !source.WithRelated || source.ID != oldPoolID {
		t.Fatalf("pool=%v err=%v", source, err)
	}
	same, err := repo.EnsureDandanplayPool(ctx, "123", true)
	if err != nil || same.ID != source.ID {
		t.Fatal("duplicate pool")
	}
	withoutRelated, err := repo.EnsureDandanplayPool(ctx, "123", false)
	if err != nil || withoutRelated.WithRelated || withoutRelated.ID == source.ID {
		t.Fatalf("second mode=%v err=%v", withoutRelated, err)
	}
	claim, err := repo.ClaimDandanplayPoolSync(ctx, source.ID, time.Minute, false)
	if err != nil || !claim {
		t.Fatal("initial claim failed")
	}
	claim, err = repo.ClaimDandanplayPoolSync(ctx, source.ID, time.Minute, false)
	if err != nil || claim {
		t.Fatal("cache window ignored")
	}
	data := []domain.DanmakuData{{Time: 1.25, Text: stringForDandanplayTest("spam")}, {Time: 2, Text: stringForDandanplayTest("keep")}}
	count, err := repo.MergeDandanplayDanmaku(ctx, source.ID, data)
	if err != nil || count != 2 {
		t.Fatalf("merge=%d %v", count, err)
	}
	count, err = repo.MergeDandanplayDanmaku(ctx, source.ID, data)
	if err != nil || count != 0 {
		t.Fatal("incremental duplicate inserted")
	}
	if _, err = repo.MergeDandanplayDanmaku(ctx, source.ID, nil); err != nil {
		t.Fatal(err)
	}
	binding, err := repo.UpsertVideoDandanplayBinding(ctx, video.ID, source.ID, 1.5)
	if err != nil || binding.PoolEpisodeID != "123" {
		t.Fatalf("binding=%v %v", binding, err)
	}
	detail, err := repo.Video(ctx, video.ID)
	if err != nil || detail.DandanplayPoolCount != 1 || detail.ThirdPartyDanmakuCount != 2 || len(detail.DandanplayBindings) != 1 {
		t.Fatalf("video=%v %v", detail, err)
	}
	sizes, err := repo.ThirdPartyPoolSizes(ctx, video.Vid)
	if err != nil || sizes["dandanplay:1"] != 2 {
		t.Fatalf("sizes=%v %v", sizes, err)
	}
	keyword, err := repo.CreateDandanplayKeyword(ctx, nil, "SPAM")
	if err != nil {
		t.Fatal(err)
	}
	visible, err := repo.DandanplayPoolData(ctx, source.ID)
	if err != nil || len(visible) != 1 || *visible[0].Text != "keep" {
		t.Fatalf("filter=%v %v", visible, err)
	}
	records, err := repo.DandanplayDanmaku(ctx, DandanplayDanmakuFilter{PoolID: source.ID})
	if err != nil || records.Total != 2 {
		t.Fatal("filter removed stored data")
	}
	if _, err = repo.DeleteDandanplayKeyword(ctx, keyword.ID); err != nil {
		t.Fatal(err)
	}
	poolKeyword, err := repo.CreateDandanplayKeyword(ctx, &source.ID, "keep")
	if err != nil {
		t.Fatal(err)
	}
	visible, err = repo.DandanplayPoolData(ctx, source.ID)
	if err != nil || len(visible) != 1 || *visible[0].Text != "spam" {
		t.Fatalf("pool filter=%v %v", visible, err)
	}
	if _, err = repo.DeleteDandanplayKeyword(ctx, poolKeyword.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.SetDandanplayDanmakuBlocked(ctx, records.List[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.MergeDandanplayDanmaku(ctx, source.ID, data); err != nil {
		t.Fatal(err)
	}
	visible, err = repo.DandanplayPoolData(ctx, source.ID)
	if err != nil || len(visible) != 1 {
		t.Fatal("refresh cleared manual block")
	}
	if _, err = repo.DeleteVideoDandanplayBinding(ctx, video.ID, binding.ID); err != nil {
		t.Fatal(err)
	}
	bindings, err := repo.VideoDandanplayBindings(ctx, video.ID)
	if err != nil || len(bindings) != 0 {
		t.Fatal("binding not removed")
	}
	if _, err := repo.MergeDandanplayDanmaku(ctx, withoutRelated.ID, []domain.DanmakuData{{Time: 3, Text: stringForDandanplayTest("independent")}}); err != nil {
		t.Fatal(err)
	}
	visible, err = repo.DandanplayPoolData(ctx, withoutRelated.ID)
	if err != nil || len(visible) != 1 || *visible[0].Text != "independent" {
		t.Fatal("pool data mixed across related modes")
	}
	for _, id := range []int{source.ID, withoutRelated.ID} {
		if _, err := repo.UpsertVideoDandanplayBinding(ctx, video.ID, id, 0); err != nil {
			t.Fatal(err)
		}
	}
	detail, err = repo.Video(ctx, video.ID)
	if err != nil || detail.DandanplayPoolCount != 2 || len(detail.DandanplayBindings) != 2 {
		t.Fatalf("two-mode bindings=%v err=%v", detail, err)
	}
	if detail.DandanplayBindings[0].WithRelated == detail.DandanplayBindings[1].WithRelated {
		t.Fatal("binding mode metadata was lost")
	}
}

func stringForDandanplayTest(value string) *string { return &value }
