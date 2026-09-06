package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"

	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
)

type mergedCache interface {
	CachedMergedDanmaku(context.Context, string, func(context.Context) ([]domain.DanmakuData, error)) ([]domain.DanmakuData, error)
}

type mergePool struct {
	native bool
	data   []domain.DanmakuData
	size   int
}

// Process entire pools in priority order. Only previously accepted pools can
// suppress a record, so repetitions inside a pool are always preserved.
func deduplicatePools(pools []mergePool) []domain.DanmakuData {
	sort.SliceStable(pools, func(i, j int) bool {
		if pools[i].native != pools[j].native {
			return pools[i].native
		}
		return max(pools[i].size, len(pools[i].data)) > max(pools[j].size, len(pools[j].data))
	})
	accepted := map[string][]float64{}
	result := make([]domain.DanmakuData, 0)
	for _, pool := range pools {
		pending := map[string][]float64{}
		for _, item := range pool.data {
			t := float64(item.Time)
			if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 {
				continue
			}
			content := pointerValue(item.Text)
			times := accepted[content]
			pos := sort.SearchFloat64s(times, t-1)
			if !pool.native && pos < len(times) && times[pos] <= t+1 {
				continue
			}
			result = append(result, item)
			pending[content] = append(pending[content], t)
		}
		for content, times := range pending {
			accepted[content] = append(accepted[content], times...)
			sort.Float64s(accepted[content])
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Time < result[j].Time })
	return result
}

func (s *Server) mergedVideoData(r *http.Request, vid string) ([]domain.DanmakuData, error) {
	ctx := r.Context()
	bili, err := s.repository.BilibiliBindingsByVID(ctx, vid)
	if err != nil {
		return nil, err
	}
	iqiyi, err := s.repository.IqiyiBindingsByVID(ctx, vid)
	if err != nil {
		return nil, err
	}
	dandanplay := []domain.DandanplayBinding{}
	if s.dandanplay != nil && s.dandanplay.repository != nil {
		dandanplay, err = s.dandanplay.repository.DandanplayBindingsByVID(ctx, vid)
		if err != nil {
			return nil, err
		}
	}
	external := []domain.ExternalBinding{}
	ext, hasExternal := s.repository.(store.ExternalRepository)
	if hasExternal {
		external, err = ext.ExternalBindingsByVID(ctx, vid)
		if err != nil {
			return nil, err
		}
	}
	// Keep passive upstream refresh outside the merged-cache lookup.
	for _, binding := range bili {
		_, _, err = s.bilibili.ensurePool(ctx, bilibiliQuery{BVID: binding.BVID, Page: binding.Page, CID: binding.CID}, false, true)
		if err != nil {
			return nil, err
		}
	}
	for _, binding := range iqiyi {
		_, _, err = s.iqiyi.ensurePool(ctx, binding.PoolVID, false, true)
		if err != nil {
			return nil, err
		}
	}
	for _, binding := range dandanplay {
		_, _, err = s.dandanplay.ensurePool(ctx, binding.PoolEpisodeID, false, true)
		if err != nil {
			return nil, err
		}
	}
	identity, _ := json.Marshal([]any{"cross-pool-v1", vid, bili, iqiyi, dandanplay, external})
	load := func(ctx context.Context) ([]domain.DanmakuData, error) {
		sizes := map[string]int{}
		if source, ok := s.repository.(interface {
			ThirdPartyPoolSizes(context.Context, string) (map[string]int, error)
		}); ok {
			sizes, err = source.ThirdPartyPoolSizes(ctx, vid)
			if err != nil {
				return nil, err
			}
		}
		local, err := s.repository.QueryByVid(ctx, vid)
		if err != nil {
			return nil, err
		}
		pools := []mergePool{{native: true, data: local}}
		for _, binding := range bili {
			data, err := s.repository.BilibiliPoolData(ctx, binding.PoolID)
			if err != nil {
				return nil, err
			}
			pools = append(pools, mergePool{size: sizes["bilibili:"+strconv.Itoa(binding.PoolID)], data: offsetDanmaku(data, binding.Offset)})
		}
		for _, binding := range iqiyi {
			data, err := s.repository.IqiyiPoolData(ctx, binding.PoolID)
			if err != nil {
				return nil, err
			}
			pools = append(pools, mergePool{size: sizes["iqiyi:"+strconv.Itoa(binding.PoolID)], data: offsetDanmaku(data, binding.Offset)})
		}
		for _, binding := range dandanplay {
			data, err := s.dandanplay.repository.DandanplayPoolData(ctx, binding.PoolID)
			if err != nil {
				return nil, err
			}
			pools = append(pools, mergePool{size: sizes["dandanplay:"+strconv.Itoa(binding.PoolID)], data: offsetDanmaku(data, binding.Offset)})
		}
		for _, binding := range external {
			data, err := ext.ExternalPoolData(ctx, binding.PoolID)
			if err != nil {
				return nil, err
			}
			pools = append(pools, mergePool{size: sizes["external:"+binding.PoolID], data: offsetDanmaku(data, binding.Offset)})
		}
		return deduplicatePools(pools), nil
	}
	if cache, ok := s.repository.(mergedCache); ok {
		return cache.CachedMergedDanmaku(ctx, string(identity), load)
	}
	return load(ctx)
}
