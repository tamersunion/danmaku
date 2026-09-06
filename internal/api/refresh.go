package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Passive refreshes never queue unbounded work or borrow an HTTP request's
// lifetime. When all slots are busy, a later read can retry the refresh.
type backgroundRefresh struct {
	mu     sync.Mutex
	active map[string]bool
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	logger *slog.Logger
	closed bool
	slots  chan struct{}
}

func newBackgroundRefresh(ctx context.Context, logger *slog.Logger) *backgroundRefresh {
	ctx, cancel := context.WithCancel(ctx)
	return &backgroundRefresh{ctx: ctx, cancel: cancel, logger: logger, active: make(map[string]bool), slots: make(chan struct{}, 8)}
}

func (r *backgroundRefresh) schedule(key string, fetch func(context.Context) error) {
	r.mu.Lock()
	if r.closed || r.ctx.Err() != nil || r.active[key] || len(r.active) >= 256 {
		r.mu.Unlock()
		return
	}
	r.active[key] = true
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		defer func() {
			if value := recover(); value != nil && r.logger != nil {
				r.logger.Error("background danmaku refresh panic", "pool", key, "panic", value)
			}
			r.mu.Lock()
			delete(r.active, key)
			r.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(r.ctx, 5*time.Minute)
		defer cancel()
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		case <-ctx.Done():
			return
		}
		if ctx.Err() != nil {
			return
		}
		if err := fetch(ctx); err != nil && r.ctx.Err() == nil && r.logger != nil {
			r.logger.Warn("background danmaku refresh failed", "pool", key, "error", err)
		}
	}()
}

func (r *backgroundRefresh) close() {
	r.mu.Lock()
	r.closed = true
	r.cancel()
	r.mu.Unlock()
	r.wg.Wait()
}

func (b *Bilibili) refreshPool(query bilibiliQuery) {
	key := fmt.Sprintf("bilibili:%s:%d", query.BVID, query.Page)
	if query.CID != 0 {
		key = fmt.Sprintf("bilibili:cid:%d", query.CID)
	}
	b.refresh.schedule(key, func(ctx context.Context) error {
		_, _, err := b.ensurePool(ctx, query, false, false)
		return err
	})
}

func (i *Iqiyi) refreshPool(vid string) {
	i.refresh.schedule("iqiyi:"+vid, func(ctx context.Context) error {
		_, _, err := i.ensurePool(ctx, vid, false, false)
		return err
	})
}

func (i *Dandanplay) refreshPool(episodeID string, withRelated bool) {
	i.refresh.schedule(fmt.Sprintf("dandanplay:%s:%t", episodeID, withRelated), func(ctx context.Context) error {
		_, _, err := i.ensurePool(ctx, episodeID, false, false, withRelated)
		return err
	})
}
