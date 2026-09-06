package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCachedDanmakuUsesRedisAndInvalidatesByRevision(t *testing.T) {
	server := miniredis.RunT(t)
	repository := &Postgres{
		redis:       redis.NewClient(&redis.Options{Addr: server.Addr()}),
		cachePrefix: "test-danmaku", cacheTTL: time.Hour,
	}
	t.Cleanup(func() { _ = repository.redis.Close() })
	text := "cached"
	loads := 0
	loader := func(context.Context) ([]domain.DanmakuData, error) {
		loads++
		return []domain.DanmakuData{{Time: float32(loads), Text: &text}}, nil
	}

	first, err := repository.cachedDanmaku(context.Background(), "native", "video", loader)
	if err != nil || loads != 1 || first[0].Time != 1 {
		t.Fatalf("first load = %#v, loads=%d, err=%v", first, loads, err)
	}
	second, err := repository.cachedDanmaku(context.Background(), "native", "video", loader)
	if err != nil || loads != 1 || second[0].Time != 1 {
		t.Fatalf("cached load = %#v, loads=%d, err=%v", second, loads, err)
	}
	repository.invalidateDanmakuCache(context.Background(), "native")
	third, err := repository.cachedDanmaku(context.Background(), "native", "video", loader)
	if err != nil || loads != 2 || third[0].Time != 2 {
		t.Fatalf("invalidated load = %#v, loads=%d, err=%v", third, loads, err)
	}
}

func TestDandanplaySeparatePoolIDsDoNotShareRedisData(t *testing.T) {
	server := miniredis.RunT(t)
	repo := &Postgres{redis: redis.NewClient(&redis.Options{Addr: server.Addr()}), cachePrefix: "ddp-modes", cacheTTL: time.Hour}
	t.Cleanup(func() { _ = repo.redis.Close() })
	loads := map[string]int{}
	// The composite database key allocates a different ID to each related mode.
	for pass := 0; pass < 2; pass++ {
		for _, poolID := range []string{"1", "2"} {
			data, err := repo.cachedDanmaku(context.Background(), "dandanplay", poolID, func(context.Context) ([]domain.DanmakuData, error) {
				loads[poolID]++
				return []domain.DanmakuData{{Text: &poolID}}, nil
			})
			if err != nil || len(data) != 1 || *data[0].Text != poolID || loads[poolID] != 1 {
				t.Fatalf("cache mixed pool IDs: %v %v", data, err)
			}
		}
	}
}

func TestMergedCacheInvalidation(t *testing.T) {
	for _, useRedis := range []bool{false, true} {
		t.Run(fmt.Sprint("redis=", useRedis), func(t *testing.T) {
			repository := &Postgres{cachePrefix: "merged-test", cacheTTL: time.Hour}
			if useRedis {
				server := miniredis.RunT(t)
				repository.redis = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = repository.redis.Close() })
			}
			loads := 0
			loader := func(context.Context) ([]domain.DanmakuData, error) {
				loads++
				return []domain.DanmakuData{{Time: float32(loads)}}, nil
			}
			read := func(want int) {
				t.Helper()
				data, err := repository.CachedMergedDanmaku(context.Background(), "video-bindings-offsets", loader)
				if err != nil || len(data) != 1 || data[0].Time != float32(want) || loads != want {
					t.Fatalf("data=%v loads=%d want=%d err=%v", data, loads, want, err)
				}
				data[0].Time = 999
			}
			read(1)
			read(1)
			for i, namespace := range []string{"native", "bilibili", "iqiyi", "external", "dandanplay", "animeko"} {
				repository.invalidateDanmakuCache(context.Background(), namespace)
				read(i + 2)
				read(i + 2)
			}
			// A write during a cache miss must prevent the old result becoming a hit.
			repository.invalidateDanmakuCache(context.Background(), "native")
			_, err := repository.CachedMergedDanmaku(context.Background(), "video-bindings-offsets", func(ctx context.Context) ([]domain.DanmakuData, error) {
				repository.invalidateDanmakuCache(ctx, "external")
				return []domain.DanmakuData{{Time: 999}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			read(8)
		})
	}
}
