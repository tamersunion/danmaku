package api

import (
	"git.hanada.info/tamersunion/danmaku/internal/domain"
	"testing"
)

func TestCrossPoolDeduplication(t *testing.T) {
	item := func(at float32, author string) domain.DanmakuData {
		text := "same"
		return domain.DanmakuData{Time: at, Text: &text, Author: author}
	}
	tests := []struct {
		name  string
		pools []mergePool
		want  []string
	}{
		{"native always retained including repetitions", []mergePool{
			{size: 100, data: []domain.DanmakuData{item(9, "third"), item(11, "third")}},
			{native: true, data: []domain.DanmakuData{item(10, "native"), item(10, "native")}},
		}, []string{"native", "native"}},
		{"raw pool size wins even when filtered", []mergePool{
			{size: 2, data: []domain.DanmakuData{item(10, "small"), item(10, "small")}},
			{size: 100, data: []domain.DanmakuData{item(10, "large")}},
		}, []string{"large"}},
		{"same pool repeats preserved and outside boundary retained", []mergePool{
			{size: 100, data: []domain.DanmakuData{item(10, "large"), item(10, "large")}},
			{data: []domain.DanmakuData{item(8.99, "outside"), item(9, "boundary"), item(11, "boundary"), item(11.01, "outside")}},
		}, []string{"outside", "large", "large", "outside"}},
		{"suppressed entries do not suppress subsequent pools", []mergePool{
			{size: 30, data: []domain.DanmakuData{item(10, "first")}},
			{size: 20, data: []domain.DanmakuData{item(11, "suppressed")}},
			{size: 10, data: []domain.DanmakuData{item(12, "last")}},
		}, []string{"first", "last"}},
		{"offset applied before deduplication", []mergePool{
			{native: true, data: []domain.DanmakuData{item(10, "native")}},
			{data: offsetDanmaku([]domain.DanmakuData{item(20, "shifted")}, -10)},
		}, []string{"native"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicatePools(tt.pools)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v entries, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Author != tt.want[i] {
					t.Fatalf("entry %d author = %q, want %q", i, got[i].Author, tt.want[i])
				}
			}
		})
	}
}
