package store

import (
	"context"
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
