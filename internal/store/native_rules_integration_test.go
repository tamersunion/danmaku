package store

import (
	"context"
	"encoding/json"
	"errors"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in only: use a disposable test database. All objects are isolated in a
// randomly named schema; no deployment database/configuration is auto-detected.
func TestNativeRulePostgresIntegration(t *testing.T) {
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
	schema := "danmaku_rules_test_" + strings.ReplaceAll(id, "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
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
	for pass := 0; pass < 2; pass++ {
		for _, sql := range schemaStatements() {
			if strings.HasPrefix(sql, `CREATE TABLE IF NOT EXISTS "Video"`) || strings.HasPrefix(sql, `CREATE TABLE IF NOT EXISTS "Danmaku"`) || strings.HasPrefix(sql, `CREATE TABLE IF NOT EXISTS "NativeDanmakuRule"`) || strings.HasPrefix(sql, `CREATE UNIQUE INDEX IF NOT EXISTS "UX_Video_Vid"`) {
				if _, err := pool.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	text := "SPAM content"
	data := domain.DanmakuData{Author: "a", AuthorID: 5, Text: &text, Time: 1}
	ip := net.ParseIP("192.0.2.1")
	if _, err := repo.CreateNativeRule(ctx, "keywords", NativeRuleInput{Value: "spam"}); err != nil {
		t.Fatal(err)
	}
	inserted, err := repo.Insert(ctx, "v", data, ip, domain.Referer{})
	if err != nil || !inserted {
		t.Fatalf("insert=%v err=%v", inserted, err)
	}
	var deleted bool
	if err := pool.QueryRow(ctx, `SELECT "IsDelete" FROM "Danmaku"`).Scan(&deleted); err != nil || !deleted {
		t.Fatalf("keyword row deleted=%v err=%v", deleted, err)
	}
	if inserted, err := repo.Insert(ctx, "v", data, ip, domain.Referer{}); err != nil || inserted {
		t.Fatalf("duplicate=%v err=%v", inserted, err)
	}
	// A historical replacement must include soft-deleted data and invalidate a
	// previously populated merged cache without altering other fields.
	loads := 0
	loader := func(context.Context) ([]domain.DanmakuData, error) { loads++; return []domain.DanmakuData{data}, nil }
	if _, err := repo.CachedMergedDanmaku(ctx, "v", loader); err != nil {
		t.Fatal(err)
	}
	result, err := repo.CreateNativeRule(ctx, "authors", NativeRuleInput{Value: "a", Replacement: "b", ScanExisting: true})
	if err != nil || result.Replaced != 1 {
		t.Fatalf("replace=%#v err=%v", result, err)
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT "Data" FROM "Danmaku"`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored domain.DanmakuData
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Author != "b" || stored.AuthorID != 5 || *stored.Text != text {
		t.Fatalf("stored=%#v", stored)
	}
	if _, err := repo.CachedMergedDanmaku(ctx, "v", loader); err != nil || loads != 2 {
		t.Fatalf("cache loads=%d err=%v", loads, err)
	}
	if _, err := repo.CreateNativeRule(ctx, "authors", NativeRuleInput{Value: "a", Replacement: "other"}); !errors.Is(err, ErrNativeRuleExists) {
		t.Fatalf("duplicate rule err=%v", err)
	}
	if _, err := repo.CreateNativeRule(ctx, "authors", NativeRuleInput{Value: "b", Replacement: "c"}); err != nil {
		t.Fatal(err)
	}
	text = "new content"
	if _, err := repo.Insert(ctx, "v", data, ip, domain.Referer{}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT "Data"->>'Author' FROM "Danmaku" WHERE NOT "IsDelete"`).Scan(&stored.Author); err != nil || stored.Author != "b" {
		t.Fatalf("must map once, author=%s err=%v", stored.Author, err)
	}
	if _, err := repo.CreateNativeRule(ctx, "ips", NativeRuleInput{Value: "192.0.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Insert(ctx, "blocked-video", data, ip, domain.Referer{}); !errors.Is(err, ErrSubmissionDenied) {
		t.Fatalf("blocked err=%v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM "Video" WHERE "Vid"='blocked-video'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("blacklist created video count=%d err=%v", count, err)
	}
	t.Run("management creation order", func(t *testing.T) {
		ids := []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"}
		for i, id := range ids {
			created := time.Date(2090, 1, 1, 0, 0, 0, 0, time.UTC)
			if i > 0 {
				created = created.Add(time.Hour)
			}
			updated := created
			if i == 0 {
				updated = created.Add(24 * time.Hour)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO "Danmaku" ("Id","Vid","Data","IsDelete","CreateTime","UpdateTime") VALUES ($1,'sort-test','{}',FALSE,$2,$3)`, id, created, updated); err != nil {
				t.Fatal(err)
			}
		}
		for _, filtered := range []bool{false, true} {
			vid := ""
			if filtered {
				vid = "sort-test"
			}
			for page := 1; page <= 3; page++ {
				result, err := repo.Search(ctx, SearchFilter{Vid: vid, Page: page, Size: 1, Descending: true})
				if err != nil || len(result.List) != 1 || result.List[0].ID != ids[3-page] {
					t.Fatalf("search filtered=%v page=%d: %#v err=%v", filtered, page, result, err)
				}
				result, err = repo.List(ctx, vid, page, 1, true)
				if err != nil || len(result.List) != 1 || result.List[0].ID != ids[3-page] {
					t.Fatalf("list filtered=%v page=%d: %#v err=%v", filtered, page, result, err)
				}
			}
		}
		result, err := repo.Search(ctx, SearchFilter{Vid: "sort-test", Page: 1, Size: 3, Descending: false})
		if err != nil || len(result.List) != 3 {
			t.Fatalf("ascending: %#v err=%v", result, err)
		}
		for i, item := range result.List {
			if item.ID != ids[i] {
				t.Fatalf("ascending entry %d=%s", i, item.ID)
			}
		}
	})
}
