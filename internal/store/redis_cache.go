package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"github.com/redis/go-redis/v9"
)

type mergedMemoryEntry struct {
	data    []byte
	expires time.Time
}

type mergedMemoryCache struct {
	sync.Mutex
	revision uint64
	entries  map[string]mergedMemoryEntry
}

func (p *Postgres) CachedMergedDanmaku(ctx context.Context, identity string, load func(context.Context) ([]domain.DanmakuData, error)) ([]domain.DanmakuData, error) {
	if p.redis != nil {
		return p.cachedDanmaku(ctx, "merged", identity, load)
	}
	p.mergedCache.Lock()
	revision := p.mergedCache.revision
	entry, ok := p.mergedCache.entries[identity]
	p.mergedCache.Unlock()
	if ok && time.Now().Before(entry.expires) {
		var data []domain.DanmakuData
		if json.Unmarshal(entry.data, &data) == nil {
			return data, nil
		}
	}
	data, err := load(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	p.mergedCache.Lock()
	defer p.mergedCache.Unlock()
	if revision == p.mergedCache.revision {
		if len(p.mergedCache.entries) >= 128 {
			p.mergedCache.entries = nil
		}
		if p.mergedCache.entries == nil {
			p.mergedCache.entries = map[string]mergedMemoryEntry{}
		}
		ttl := p.cacheTTL
		if ttl <= 0 {
			ttl = time.Hour
		}
		p.mergedCache.entries[identity] = mergedMemoryEntry{data: raw, expires: time.Now().Add(ttl)}
	}
	return data, nil
}

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
	p.mergedCache.Lock()
	p.mergedCache.revision++
	p.mergedCache.entries = nil
	p.mergedCache.Unlock()
	if p.redis != nil {
		cacheContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = p.redis.Incr(cacheContext, p.cacheRevisionKey(namespace)).Err()
		if namespace != "merged" {
			_ = p.redis.Incr(cacheContext, p.cacheRevisionKey("merged")).Err()
		}
	}
}

func (p *Postgres) cacheRevisionKey(namespace string) string {
	return p.cachePrefix + ":revision:" + namespace
}
