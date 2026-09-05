package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDanmakuConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "appsettings.yml")
	contents := []byte("KestrelSettings:\n  Host: 0.0.0.0\n  Port: 8080\nDanmakuSql:\n  Host: database\n  UserName: app\n  PassWord: secret\n  DataBase: app\n  PoolSize: 12\nBiliBiliSetting:\n  DanmakuCacheTime: 9\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DanmakuSQL.Host != "database" || cfg.DanmakuSQL.Port != 5432 || cfg.DanmakuSQL.PoolSize != 12 {
		t.Fatalf("unexpected database settings: %#v", cfg.DanmakuSQL)
	}
	if cfg.Bilibili.DataCacheMinutes != 9 {
		t.Fatalf("unexpected cache duration: %d", cfg.Bilibili.DataCacheMinutes)
	}
	if cfg.Bilibili.APIBase != DefaultBilibiliAPIBase {
		t.Fatalf("default Bilibili API base = %q", cfg.Bilibili.APIBase)
	}
	if cfg.CAS.DefaultRole != 3 {
		t.Fatalf("default CAS role = %d", cfg.CAS.DefaultRole)
	}
}

func TestLoadCustomBilibiliAPIBase(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "appsettings.yml")
	contents := []byte("KestrelSettings:\n  Port: 80\nBiliBiliSetting:\n  ApiBase: https://bilibili.example/api/\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bilibili.APIBase != "https://bilibili.example/api" {
		t.Fatalf("custom Bilibili API base = %q", cfg.Bilibili.APIBase)
	}
}

func TestConnectionStringEscapesCredentials(t *testing.T) {
	settings := DatabaseSettings{Host: "localhost", Port: 5432, UserName: "user@example", Password: "p:a/ss", Database: "data/base"}
	if got := settings.ConnectionString(); got != "postgres://user%40example:p%3Aa%2Fss@localhost:5432/data%2Fbase" {
		t.Fatalf("unexpected connection string: %s", got)
	}
}

func TestLoadLegacyDanmakuConfigurationAliases(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "appsettings.yml")
	contents := []byte("KestrelSettings:\n  Port: 80\nDanmuSql:\n  Host: legacy-database\n  UserName: legacy-user\n  PassWord: legacy-password\n  DataBase: legacy-name\nBiliBiliSetting:\n  DanmuCacheTime: 17\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DanmakuSQL.Host != "legacy-database" || cfg.DanmakuSQL.UserName != "legacy-user" || cfg.DanmakuSQL.Database != "legacy-name" || cfg.DanmakuSQL.Port != 5432 {
		t.Fatalf("legacy database alias was not normalized: %#v", cfg.DanmakuSQL)
	}
	if cfg.Bilibili.DataCacheMinutes != 17 {
		t.Fatalf("legacy cache alias was not normalized: %d", cfg.Bilibili.DataCacheMinutes)
	}
}

func TestLoadCASConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "appsettings.yml")
	contents := []byte("KestrelSettings:\n  Port: 80\nCAS:\n  Enabled: true\n  BaseURL: https://cas.example/cas/app\n  PublicURL: https://danmaku.example\n  DefaultRole: 2\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CAS.Enabled || !cfg.CAS.DefaultLogin || !cfg.CAS.AutoCreateUsers || cfg.CAS.DefaultRole != 2 || cfg.CAS.SessionMaxAgeMinutes != 10080 {
		t.Fatalf("unexpected CAS configuration: %#v", cfg.CAS)
	}
}

func TestLoadAllowsGeneralUserAsCASDefaultRole(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "appsettings.yml")
	contents := []byte("KestrelSettings:\n  Port: 80\nCAS:\n  Enabled: true\n  BaseURL: https://cas.example/cas/app\n  PublicURL: https://danmaku.example\n  DefaultRole: 3\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CAS.DefaultRole != 3 {
		t.Fatalf("default role = %d", cfg.CAS.DefaultRole)
	}
}
