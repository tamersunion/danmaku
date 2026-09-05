package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/redis/go-redis/v9"
)

func (p *Postgres) cachedDanmaku(ctx context.Context, namespace, identity string, load func(context.Context) ([]domain.DanmakuData, error)) ([]domain.DanmakuData, error) {
	if p.redis == nil {
		return load(ctx)
	}
	revision, err := p.redis.Get(ctx, p.cacheRevisionKey(namespace)).Result()
	if err == redis.Nil {
		revision = "0"
	} else if err != nil {
		return load(ctx)
	}
	hash := sha256.Sum256([]byte(identity))
	key := fmt.Sprintf("%s:data:%s:%s:%s", p.cachePrefix, namespace, revision, hex.EncodeToString(hash[:]))
	if raw, err := p.redis.Get(ctx, key).Bytes(); err == nil {
		var result []domain.DanmakuData
		if json.Unmarshal(raw, &result) == nil {
			if result == nil {
				result = []domain.DanmakuData{}
			}
			return result, nil
		}
	}
	result, err := load(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []domain.DanmakuData{}
	}
	if raw, err := json.Marshal(result); err == nil {
		_ = p.redis.Set(ctx, key, raw, p.cacheTTL).Err()
	}
	return result, nil
}

func (p *Postgres) invalidateDanmakuCache(ctx context.Context, namespace string) {
	if p.redis != nil {
		cacheContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = p.redis.Incr(cacheContext, p.cacheRevisionKey(namespace)).Err()
	}
}

func (p *Postgres) cacheRevisionKey(namespace string) string {
	return p.cachePrefix + ":revision:" + namespace
}
