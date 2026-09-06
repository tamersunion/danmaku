package api

import (
	"context"
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"git.hanada.info/tamersunion/danmaku/internal/store"
	"time"
)

type fakeDandanplayRepository struct {
	fakeRepository
	dandanplayPools    []domain.DandanplayPool
	dandanplayData     map[int][]domain.DanmakuData
	dandanplayClaims   map[int]time.Time
	dandanplayKeywords []domain.DandanplayKeyword
	dandanplayBindings []domain.DandanplayBinding
}

func (f *fakeDandanplayRepository) DandanplayPool(_ context.Context, id int) (*domain.DandanplayPool, error) {
	for index := range f.dandanplayPools {
		if f.dandanplayPools[index].ID == id {
			value := f.dandanplayPools[index]
			return &value, nil
		}
	}
	return nil, nil
}
func (f *fakeDandanplayRepository) EnsureDandanplayPool(_ context.Context, vid string) (*domain.DandanplayPool, error) {
	for index := range f.dandanplayPools {
		if f.dandanplayPools[index].EpisodeID == vid {
			value := f.dandanplayPools[index]
			return &value, nil
		}
	}
	value := domain.DandanplayPool{ID: len(f.dandanplayPools) + 1, EpisodeID: vid}
	f.dandanplayPools = append(f.dandanplayPools, value)
	return &value, nil
}
func (f *fakeDandanplayRepository) ClaimDandanplayPoolSync(_ context.Context, id int, interval time.Duration, force bool) (bool, error) {
	if f.dandanplayClaims == nil {
		f.dandanplayClaims = map[int]time.Time{}
	}
	last, exists := f.dandanplayClaims[id]
	if !force && exists && time.Since(last) <= interval {
		return false, nil
	}
	f.dandanplayClaims[id] = time.Now()
	return true, nil
}
func (f *fakeDandanplayRepository) MergeDandanplayDanmaku(_ context.Context, poolID int, data []domain.DanmakuData) (int, error) {
	if f.dandanplayData == nil {
		f.dandanplayData = map[int][]domain.DanmakuData{}
	}
	inserted := 0
	for _, candidate := range data {
		duplicate := false
		for _, existing := range f.dandanplayData[poolID] {
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
			f.dandanplayData[poolID] = append(f.dandanplayData[poolID], candidate)
			inserted++
		}
	}
	return inserted, nil
}
func (f *fakeDandanplayRepository) DandanplayPoolData(_ context.Context, poolID int) ([]domain.DanmakuData, error) {
	return append([]domain.DanmakuData(nil), f.dandanplayData[poolID]...), nil
}
func (f *fakeDandanplayRepository) DandanplayPools(context.Context, store.DandanplayPoolFilter) (domain.Page[domain.DandanplayPool], error) {
	return domain.Page[domain.DandanplayPool]{Total: len(f.dandanplayPools), List: append([]domain.DandanplayPool{}, f.dandanplayPools...)}, nil
}
func (f *fakeDandanplayRepository) DandanplayDanmaku(context.Context, store.DandanplayDanmakuFilter) (domain.Page[domain.DandanplayDanmaku], error) {
	return domain.Page[domain.DandanplayDanmaku]{List: []domain.DandanplayDanmaku{}}, nil
}
func (f *fakeDandanplayRepository) SetDandanplayDanmakuBlocked(context.Context, int64, bool) (bool, error) {
	return true, nil
}
func (f *fakeDandanplayRepository) DandanplayKeywords(context.Context) ([]domain.DandanplayKeyword, error) {
	return f.dandanplayKeywords, nil
}
func (f *fakeDandanplayRepository) CreateDandanplayKeyword(_ context.Context, poolID *int, keyword string) (*domain.DandanplayKeyword, error) {
	value := domain.DandanplayKeyword{ID: len(f.dandanplayKeywords) + 1, PoolID: poolID, Keyword: keyword}
	f.dandanplayKeywords = append(f.dandanplayKeywords, value)
	return &value, nil
}
func (f *fakeDandanplayRepository) DeleteDandanplayKeyword(_ context.Context, id int) (bool, error) {
	for index := range f.dandanplayKeywords {
		if f.dandanplayKeywords[index].ID == id {
			f.dandanplayKeywords = append(f.dandanplayKeywords[:index], f.dandanplayKeywords[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (f *fakeDandanplayRepository) DandanplayBindingsByVID(_ context.Context, vid string) ([]domain.DandanplayBinding, error) {
	result := make([]domain.DandanplayBinding, 0)
	for _, item := range f.dandanplayBindings {
		if item.Vid == vid {
			result = append(result, item)
		}
	}
	return result, nil
}
func (f *fakeDandanplayRepository) VideoDandanplayBindings(_ context.Context, videoID int) ([]domain.DandanplayBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return []domain.DandanplayBinding{}, nil
	}
	return f.DandanplayBindingsByVID(context.Background(), video.Vid)
}
func (f *fakeDandanplayRepository) UpsertVideoDandanplayBinding(_ context.Context, videoID, poolID int, offset float64) (*domain.DandanplayBinding, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return nil, nil
	}
	if video.IsDeleted {
		return nil, store.ErrVideoDeleted
	}
	pool, _ := f.DandanplayPool(context.Background(), poolID)
	if pool == nil {
		return nil, nil
	}
	for index := range f.dandanplayBindings {
		if f.dandanplayBindings[index].Vid == video.Vid && f.dandanplayBindings[index].PoolID == poolID {
			f.dandanplayBindings[index].Offset = offset
			f.dandanplayBindings[index].PoolEpisodeID = pool.EpisodeID
			return &f.dandanplayBindings[index], nil
		}
	}
	value := domain.DandanplayBinding{ID: len(f.dandanplayBindings) + 1, Vid: video.Vid, PoolID: poolID, PoolEpisodeID: pool.EpisodeID, Offset: offset}
	f.dandanplayBindings = append(f.dandanplayBindings, value)
	return &f.dandanplayBindings[len(f.dandanplayBindings)-1], nil
}
func (f *fakeDandanplayRepository) DeleteVideoDandanplayBinding(_ context.Context, videoID, id int) (bool, error) {
	video, _ := f.videoWithoutBindings(videoID)
	if video == nil {
		return false, nil
	}
	for index := range f.dandanplayBindings {
		if f.dandanplayBindings[index].ID == id && f.dandanplayBindings[index].Vid == video.Vid {
			f.dandanplayBindings = append(f.dandanplayBindings[:index], f.dandanplayBindings[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}
