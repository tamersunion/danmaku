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
}

func TestConnectionStringEscapesCredentials(t *testing.T) {
	settings := DatabaseSettings{Host: "localhost", Port: 5432, UserName: "user@example", Password: "p:a/ss", Database: "data/base"}
	if got := settings.ConnectionString(); got != "postgres://user%40example:p%3Aa%2Fss@localhost:5432/data%2Fbase" {
		t.Fatalf("unexpected connection string: %s", got)
	}
}
