package config

import "testing"

func TestCatalogConfiguration(t *testing.T) {
	cfg, e := Load(writeConfig(t, "appsettings.json", "{}"))
	if e != nil {
		t.Fatal(e)
	}
	for source, c := range map[string]CatalogSettings{"bahamut": cfg.Bahamut, "tencent": cfg.Tencent, "youku": cfg.Youku} {
		if c != DefaultCatalogSettings(source) {
			t.Errorf("defaults %s: %v", source, c)
		}
	}
	for _, raw := range []string{
		`{"bahamut_setting":{"api_base":"file:///tmp/a"}}`,
		`{"tencent_setting":{"sync_interval_seconds":0}}`,
		`{"youku_setting":{"session_api_base":"invalid"}}`,
		`{"youku_setting":{"unknown":true}}`,
	} {
		if _, e := Load(writeConfig(t, "appsettings.json", raw)); e == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	cfg, e = Load(writeConfig(t, "appsettings.json", `{"bahamut_setting":{"api_base":"https://example.com/comments","sync_interval_seconds":12}}`))
	if e != nil || cfg.Bahamut.SyncIntervalSeconds != 12 || cfg.Bahamut.APIBase != "https://example.com/comments" {
		t.Fatal("custom config")
	}
}
