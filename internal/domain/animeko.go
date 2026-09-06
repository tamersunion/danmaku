package domain

import "time"

type AnimekoPool struct {
	ID              int        `json:"id"`
	EpisodeID       string     `json:"episodeId"`
	DanmakuCount    int        `json:"danmakuCount"`
	BlockedCount    int        `json:"blockedCount"`
	BindingCount    int        `json:"bindingCount"`
	LastAttemptTime *time.Time `json:"lastAttemptTime"`
	LastSyncTime    *time.Time `json:"lastSyncTime"`
	CreateTime      time.Time  `json:"createTime"`
	UpdateTime      time.Time  `json:"updateTime"`
}

type AnimekoDanmaku struct {
	ID              int64       `json:"id"`
	PoolID          int         `json:"poolId"`
	Data            DanmakuData `json:"data"`
	IsBlocked       bool        `json:"isBlocked"`
	ManuallyBlocked bool        `json:"manuallyBlocked"`
	CreateTime      time.Time   `json:"createTime"`
	UpdateTime      time.Time   `json:"updateTime"`
}

type AnimekoKeyword struct {
	ID            int       `json:"id"`
	PoolID        *int      `json:"poolId"`
	PoolEpisodeID string    `json:"poolEpisodeId"`
	Keyword       string    `json:"keyword"`
	CreateTime    time.Time `json:"createTime"`
}

type AnimekoBinding struct {
	ID            int       `json:"id"`
	Vid           string    `json:"vid"`
	PoolID        int       `json:"poolId"`
	PoolEpisodeID string    `json:"poolEpisodeId"`
	Offset        float64   `json:"offset"`
	CreateTime    time.Time `json:"createTime"`
	UpdateTime    time.Time `json:"updateTime"`
}
