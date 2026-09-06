package api

import (
	"context"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"time"
)

type fakeAnimekoRepository struct {
	fakeRepository
	animekoPools    []domain.AnimekoPool
	animekoData     map[int][]domain.DanmakuData
	animekoClaims   map[int]time.Time
	animekoKeywords []domain.AnimekoKeyword
	animekoBindings []domain.AnimekoBinding
}

func (f *fakeAnimekoRepository) AnimekoPool(_ context.Context, id int) (*domain.AnimekoPool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	for index := range f.animekoPools {
		if f.animekoPools[index].ID == id {
			value := f.animekoPools[index]
			return &value, nil
		}
	}
	return nil, nil
}
func (f *fakeAnimekoRepository) EnsureAnimekoPool(_ context.Context, vid string) (*domain.AnimekoPool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	for index := range f.animekoPools {
		if f.animekoPools[index].EpisodeID == vid {
			value := f.animekoPools[index]
			return &value, nil
		}
	}
	value := domain.AnimekoPool{ID: len(f.animekoPools) + 1, EpisodeID: vid}
	f.animekoPools = append(f.animekoPools, value)
	return &value, nil
}
func (f *fakeAnimekoRepository) ClaimAnimekoPoolSync(_ context.Context, id int, interval time.Duration, force bool) (bool, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.animekoClaims == nil {
		f.animekoClaims = map[int]time.Time{}
	}
	last, exists := f.animekoClaims[id]
	if !force && exists && time.Since(last) <= interval {
		return false, nil
	}
	f.animekoClaims[id] = time.Now()
	return true, nil
}
func (f *fakeAnimekoRepository) MergeAnimekoDanmaku(_ context.Context, poolID int, data []domain.DanmakuData) (int, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	if f.animekoData == nil {
		f.animekoData = map[int][]domain.DanmakuData{}
	}
	inserted := 0
	for _, candidate := range data {
		duplicate := false
		for _, existing := range f.animekoData[poolID] {
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
			f.animekoData[poolID] = append(f.animekoData[poolID], candidate)
			inserted++
		}
	}
	return inserted, nil
}
func (f *fakeAnimekoRepository) AnimekoPoolData(_ context.Context, poolID int) ([]domain.DanmakuData, error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	return append([]domain.DanmakuData(nil), f.animekoData[poolID]...), nil
}
func (f *fakeAnimekoRepository) AnimekoPools(context.Context, store.AnimekoPoolFilter) (domain.Page[domain.AnimekoPool], error) {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	return domain.Page[domain.AnimekoPool]{Total: len(f.animekoPools), List: append([]domain.AnimekoPool{}, f.animekoPools...)}, nil
}
func (f *fakeAnimekoRepository) AnimekoDanmaku(context.Context, store.AnimekoDanmakuFilter) (domain.Page[domain.AnimekoDanmaku], error) {
	return domain.Page[domain.AnimekoDanmaku]{List: []domain.AnimekoDanmaku{}}, nil
}
func (f *fakeAnimekoRepository) SetAnimekoDanmakuBlocked(context.Context, int64, bool) (bool, error) {
	return true, nil
}
func (f *fakeAnimekoRepository) AnimekoKeywords(context.Context) ([]domain.AnimekoKeyword, error) {
	return f.animekoKeywords, nil
}
func (f *fakeAnimekoRepository) CreateAnimekoKeyword(_ context.Context, poolID *int, keyword string) (*domain.AnimekoKeyword, error) {
	value := domain.AnimekoKeyword{ID: len(f.animekoKeywords) + 1, PoolID: poolID, Keyword: keyword}
	f.animekoKeywords = append(f.animekoKeywords, value)
	return &value, nil
}
func (f *fakeAnimekoRepository) DeleteAnimekoKeyword(_ context.Context, id int) (bool, error) {
	for index := range f.animekoKeywords {
		if f.animekoKeywords[index].ID == id {
			f.animekoKeywords = append(f.animekoKeywords[:index], f.animekoKeywords[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeAnimekoRepository) AnimekoBindingsByVID(_ context.Context, vid string) ([]domain.AnimekoBinding, error) {
	result := make([]domain.AnimekoBinding, 0)
	for _, item := range f.animekoBindings {
		if item.Vid == vid {
			result = append(result, item)
		}
	}
	return result, nil
}
func (f *fakeAnimekoRepository) VideoAnimekoBindings(_ context.Context, videoID int) ([]domain.AnimekoBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return []domain.AnimekoBinding{}, nil
	}
	return f.AnimekoBindingsByVID(context.Background(), video.Vid)
}
func (f *fakeAnimekoRepository) UpsertVideoAnimekoBinding(_ context.Context, videoID, poolID int, offset float64) (*domain.AnimekoBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return nil, nil
	}
	if video.IsDeleted {
		return nil, store.ErrVideoDeleted
	}
	pool, _ := f.AnimekoPool(context.Background(), poolID)
	if pool == nil {
		return nil, nil
	}
	for index := range f.animekoBindings {
		if f.animekoBindings[index].Vid == video.Vid && f.animekoBindings[index].PoolID == poolID {
			f.animekoBindings[index].Offset = offset
			f.animekoBindings[index].PoolEpisodeID = pool.EpisodeID
			return &f.animekoBindings[index], nil
		}
	}
	value := domain.AnimekoBinding{ID: len(f.animekoBindings) + 1, Vid: video.Vid, PoolID: poolID, PoolEpisodeID: pool.EpisodeID, Offset: offset}
	f.animekoBindings = append(f.animekoBindings, value)
	return &f.animekoBindings[len(f.animekoBindings)-1], nil
}
func (f *fakeAnimekoRepository) DeleteVideoAnimekoBinding(_ context.Context, videoID, id int) (bool, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return false, nil
	}
	for index := range f.animekoBindings {
		if f.animekoBindings[index].ID == id && f.animekoBindings[index].Vid == video.Vid {
			f.animekoBindings = append(f.animekoBindings[:index], f.animekoBindings[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
