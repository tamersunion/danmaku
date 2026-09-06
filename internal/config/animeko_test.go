package config

import "testing"

func TestAnimekoAndIqiyiSearchConfiguration(t *testing.T) {
	cfg, err := Load(writeConfig(t, "appsettings.json", `{}`))
	if err != nil || cfg.Animeko.APIBase != DefaultAnimekoAPIBase || cfg.Animeko.BangumiAPIBase != DefaultAnimekoBangumiAPIBase || cfg.Animeko.SyncIntervalSeconds != 600 || cfg.Iqiyi.SearchAPIBase != DefaultIqiyiSearchAPIBase || cfg.Iqiyi.EpisodesAPIBase != DefaultIqiyiEpisodesAPIBase {
		t.Fatalf("defaults=%v err=%v", cfg.Animeko, err)
	}
	cfg, err = Load(writeConfig(t, "appsettings.json", `{"animeko_setting":{"api_base":"https://anime.example","bangumi_api_base":"https://bangumi.example","sync_interval_seconds":31},"iqiyi_setting":{"search_api_base":"https://iqiyi.example/search","episodes_api_base":"https://iqiyi.example/episodes"}}`))
	if err != nil || cfg.Animeko.SyncIntervalSeconds != 31 || cfg.Iqiyi.SearchAPIBase != "https://iqiyi.example/search" {
		t.Fatalf("custom=%v err=%v", cfg.Animeko, err)
	}
	for _, raw := range []string{`{"animeko_setting":{"sync_interval_seconds":0}}`, `{"animeko_setting":{"api_base":"file:///tmp/a"}}`, `{"animeko_setting":{"bangumi_api_base":"ftp://example.com"}}`, `{"iqiyi_setting":{"search_api_base":"not-url"}}`, `{"iqiyi_setting":{"episodes_api_base":"file:///tmp/a"}}`} {
		if _, err := Load(writeConfig(t, "appsettings.json", raw)); err == nil {
			t.Errorf("invalid config accepted %s", raw)
		}
	}
}
