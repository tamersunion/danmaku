package api

import (
	"context"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"time"
)

type fakeCatalogRepository struct {
	fakeRepository
	catalogPools    []domain.CatalogPool
	catalogData     map[int][]domain.DanmakuData
	catalogClaims   map[int]time.Time
	catalogKeywords []domain.CatalogKeyword
	catalogBindings []domain.CatalogBinding
}

func (f *fakeCatalogRepository) CatalogPool(_ context.Context, id int) (*domain.CatalogPool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	for index := range f.catalogPools {
		if f.catalogPools[index].ID == id {
			value := f.catalogPools[index]
			return &value, nil
		}
	}
	return nil, nil
}
func (f *fakeCatalogRepository) EnsureCatalogPool(_ context.Context, vid string) (*domain.CatalogPool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	for index := range f.catalogPools {
		if f.catalogPools[index].EpisodeID == vid {
			value := f.catalogPools[index]
			return &value, nil
		}
	}
	value := domain.CatalogPool{ID: len(f.catalogPools) + 1, EpisodeID: vid}
	f.catalogPools = append(f.catalogPools, value)
	return &value, nil
}
func (f *fakeCatalogRepository) ClaimCatalogPoolSync(_ context.Context, id int, interval time.Duration, force bool) (bool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.catalogClaims == nil {
		f.catalogClaims = map[int]time.Time{}
	}
	last, exists := f.catalogClaims[id]
	if !force && exists && time.Since(last) <= interval {
		return false, nil
	}
	f.catalogClaims[id] = time.Now()
	return true, nil
}
func (f *fakeCatalogRepository) MergeCatalogDanmaku(_ context.Context, poolID int, data []domain.DanmakuData) (int, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.catalogData == nil {
		f.catalogData = map[int][]domain.DanmakuData{}
	}
	inserted := 0
	for _, candidate := range data {
		duplicate := false
		for _, existing := range f.catalogData[poolID] {
			candidateText, existingText := "", ""
			if candidate.Text != nil {
				candidateText = *candidate.Text
			}
			if existing.Text != nil {
				existingText = *existing.Text
			}
			if candidate.Time == existing.Time && candidateText == existingText {
				duplicate = true
				break
			}
		}
		if !duplicate {
			f.catalogData[poolID] = append(f.catalogData[poolID], candidate)
			inserted++
		}
	}
	return inserted, nil
}
func (f *fakeCatalogRepository) CatalogPoolData(_ context.Context, poolID int) ([]domain.DanmakuData, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	return append([]domain.DanmakuData(nil), f.catalogData[poolID]...), nil
}
func (f *fakeCatalogRepository) CatalogPools(context.Context, store.CatalogPoolFilter) (domain.Page[domain.CatalogPool], error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	return domain.Page[domain.CatalogPool]{Total: len(f.catalogPools), List: append([]domain.CatalogPool{}, f.catalogPools...)}, nil
}
func (f *fakeCatalogRepository) CatalogDanmaku(context.Context, store.CatalogDanmakuFilter) (domain.Page[domain.CatalogDanmaku], error) {
	return domain.Page[domain.CatalogDanmaku]{List: []domain.CatalogDanmaku{}}, nil
}
func (f *fakeCatalogRepository) SetCatalogDanmakuBlocked(context.Context, int64, bool) (bool, error) {
	return true, nil
}
func (f *fakeCatalogRepository) CatalogKeywords(context.Context) ([]domain.CatalogKeyword, error) {
	return f.catalogKeywords, nil
}
func (f *fakeCatalogRepository) CreateCatalogKeyword(_ context.Context, poolID *int, keyword string) (*domain.CatalogKeyword, error) {
	value := domain.CatalogKeyword{ID: len(f.catalogKeywords) + 1, PoolID: poolID, Keyword: keyword}
	f.catalogKeywords = append(f.catalogKeywords, value)
	return &value, nil
}
func (f *fakeCatalogRepository) DeleteCatalogKeyword(_ context.Context, id int) (bool, error) {
	for index := range f.catalogKeywords {
		if f.catalogKeywords[index].ID == id {
			f.catalogKeywords = append(f.catalogKeywords[:index], f.catalogKeywords[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeCatalogRepository) CatalogBindingsByVID(_ context.Context, vid string) ([]domain.CatalogBinding, error) {
	result := make([]domain.CatalogBinding, 0)
	for _, item := range f.catalogBindings {
		if item.Vid == vid {
			result = append(result, item)
		}
	}
	return result, nil
}
func (f *fakeCatalogRepository) VideoCatalogBindings(_ context.Context, videoID int) ([]domain.CatalogBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return []domain.CatalogBinding{}, nil
	}
	return f.CatalogBindingsByVID(context.Background(), video.Vid)
}
func (f *fakeCatalogRepository) UpsertVideoCatalogBinding(_ context.Context, videoID, poolID int, offset float64) (*domain.CatalogBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return nil, nil
	}
	if video.IsDeleted {
		return nil, store.ErrVideoDeleted
	}
	pool, _ := f.CatalogPool(context.Background(), poolID)
	if pool == nil {
		return nil, nil
	}
	for index := range f.catalogBindings {
		if f.catalogBindings[index].Vid == video.Vid && f.catalogBindings[index].PoolID == poolID {
			f.catalogBindings[index].Offset = offset
			f.catalogBindings[index].PoolEpisodeID = pool.EpisodeID
			return &f.catalogBindings[index], nil
		}
	}
	value := domain.CatalogBinding{ID: len(f.catalogBindings) + 1, Vid: video.Vid, PoolID: poolID, PoolEpisodeID: pool.EpisodeID, Offset: offset}
	f.catalogBindings = append(f.catalogBindings, value)
	return &f.catalogBindings[len(f.catalogBindings)-1], nil
}
func (f *fakeCatalogRepository) DeleteVideoCatalogBinding(_ context.Context, videoID, id int) (bool, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return false, nil
	}
	for index := range f.catalogBindings {
		if f.catalogBindings[index].ID == id && f.catalogBindings[index].Vid == video.Vid {
			f.catalogBindings = append(f.catalogBindings[:index], f.catalogBindings[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
