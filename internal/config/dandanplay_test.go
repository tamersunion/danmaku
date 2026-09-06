package config

import "testing"

func TestDandanplayConfiguration(t *testing.T) {
	cfg, err := Load(writeConfig(t, "appsettings.json", `{}`))
	if err != nil || cfg.Dandanplay.APIBase != DefaultDandanplayAPIBase || cfg.Dandanplay.SyncIntervalSeconds != 600 {
		t.Fatalf("defaults=%#v err=%v", cfg.Dandanplay, err)
	}
	cfg, err = Load(writeConfig(t, "appsettings.json", `{"dandanplay_setting":{"api_base":"https://gateway.example/ddp/v1","sync_interval_seconds":31}}`))
	if err != nil || cfg.Dandanplay.SyncIntervalSeconds != 31 || cfg.Dandanplay.APIBase != "https://gateway.example/ddp/v1" {
		t.Fatalf("custom=%#v err=%v", cfg.Dandanplay, err)
	}
	for _, value := range []string{`{"api_base":"file:///tmp/test"}`, `{"sync_interval_seconds":0}`, `{"sync_interval_seconds":-1}`, `{"obsolete":true}`} {
		if _, err := Load(writeConfig(t, "appsettings.json", `{"dandanplay_setting":`+value+`}`)); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
}
