package api

import (
	"context"
	"git.hanada.info/tamersunion/danmaku/internal/config"
	"os"
	"testing"
	"time"
)

func TestCatalogLive(t *testing.T) {
	if os.Getenv("CATALOG_LIVE_TEST") != "1" {
		t.Skip("set CATALOG_LIVE_TEST=1 for real upstream checks")
	}
	for _, source := range []string{"bahamut", "tencent", "youku"} {
		t.Run(source, func(t *testing.T) {
			c := NewCatalog(&fakeRepository{}, source, config.DefaultCatalogSettings(source), nil)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			keyword := map[string]string{"bahamut": "葬送的芙莉莲", "tencent": "斗罗大陆", "youku": "凡人修仙传"}[source]
			results, err := c.Search(ctx, keyword)
			if err != nil || len(results) == 0 {
				t.Fatalf("search count=%d err=%v", len(results), err)
			}
			t.Logf("search=%d first=%s id=%s", len(results), results[0].Title, results[0].AnimeID)
			episodes, err := c.Episodes(ctx, results[0].AnimeID)
			if err != nil || len(episodes) == 0 {
				t.Fatalf("episodes=%d err=%v", len(episodes), err)
			}
			t.Logf("episodes=%d first=%s id=%s", len(episodes), episodes[0].Title, episodes[0].EpisodeID)
			data, err := c.fetchData(ctx, episodes[0].EpisodeID)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("comments=%d", len(data))
		})
	}
}
