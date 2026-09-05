package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadJSONConfiguration(t *testing.T) {
	path := writeConfig(t, "appsettings.json", `{
		"kestrel_settings": {"host": "0.0.0.0", "port": 8080},
		"danmaku_sql": {
			"host": "database",
			"user_name": "app",
			"password": "secret",
			"database": "app",
			"pool_size": 12
		},
		"admin": {"max_age_seconds": 3600},
		"bilibili_setting": {
			"cid_cache_seconds": 10080,
			"sync_interval_seconds": 37
		},
		"cas": {
			"session_max_age_seconds": 604800,
			"request_timeout_seconds": 2
		}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DanmakuSQL.Host != "database" || cfg.DanmakuSQL.Port != 5432 || cfg.DanmakuSQL.PoolSize != 12 {
		t.Fatalf("unexpected database settings: %#v", cfg.DanmakuSQL)
	}
	if cfg.Admin.MaxAgeSeconds != 3600 || cfg.Bilibili.CIDCacheSeconds != 10080 || cfg.Bilibili.SyncIntervalSeconds != 37 {
		t.Fatalf("time settings are not in seconds: admin=%d bilibili=%#v", cfg.Admin.MaxAgeSeconds, cfg.Bilibili)
	}
	if cfg.CAS.SessionMaxAgeSeconds != 604800 || cfg.CAS.RequestTimeoutSeconds != 2 {
		t.Fatalf("unexpected CAS time settings: %#v", cfg.CAS)
	}
	if cfg.Bilibili.APIBase != DefaultBilibiliAPIBase {
		t.Fatalf("default bilibili API base = %q", cfg.Bilibili.APIBase)
	}
}

func TestLoadCustomBilibiliAPIBase(t *testing.T) {
	path := writeConfig(t, "appsettings.json", `{
		"bilibili_setting": {"api_base": "https://bilibili.example/api/"}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bilibili.APIBase != "https://bilibili.example/api" {
		t.Fatalf("custom bilibili API base = %q", cfg.Bilibili.APIBase)
	}
}

func TestConnectionStringEscapesCredentials(t *testing.T) {
	settings := DatabaseSettings{Host: "localhost", Port: 5432, UserName: "user@example", Password: "p:a/ss", Database: "data/base"}
	if got := settings.ConnectionString(); got != "postgres://user%40example:p%3Aa%2Fss@localhost:5432/data%2Fbase" {
		t.Fatalf("unexpected connection string: %s", got)
	}
}

func TestLoadRejectsRemovedConfigurationFormatsAndFields(t *testing.T) {
	for name, test := range map[string]struct {
		contents string
		expected string
	}{
		"YAML": {
			contents: "kestrel_settings:\n  port: 80\n",
			expected: `unsupported config format ".yml"`,
		},
		"legacy danmu field": {
			contents: `{"danmu_sql":{"host":"database"}}`,
			expected: `unknown field "danmu_sql"`,
		},
		"legacy PascalCase field": {
			contents: `{"KestrelSettings":{"Port":80}}`,
			expected: `unknown field "KestrelSettings"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			extension := ".json"
			if name == "YAML" {
				extension = ".yml"
			}
			_, err := Load(writeConfig(t, "appsettings"+extension, test.contents))
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("error = %v, want %q", err, test.expected)
			}
		})
	}
}

func TestConfigCandidatesUseJSONOnly(t *testing.T) {
	t.Setenv("DANMAKU_ENVIRONMENT", "Production")
	want := []string{"appsettings.json", "appsettings.Production.json"}
	if got := configCandidates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("config candidates = %#v, want %#v", got, want)
	}
}

func TestLoadCASConfiguration(t *testing.T) {
	path := writeConfig(t, "appsettings.json", `{
		"cas": {
			"enabled": true,
			"base_url": "https://cas.example/cas/app",
			"public_url": "https://danmaku.example",
			"default_role": 2,
			"session_max_age_seconds": 10080
		}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CAS.Enabled || !cfg.CAS.DefaultLogin || !cfg.CAS.AutoCreateUsers || cfg.CAS.DefaultRole != 2 || cfg.CAS.SessionMaxAgeSeconds != 10080 {
		t.Fatalf("unexpected CAS configuration: %#v", cfg.CAS)
	}
}

func TestLoadAllowsGeneralUserAsCASDefaultRole(t *testing.T) {
	path := writeConfig(t, "appsettings.json", `{
		"cas": {
			"enabled": true,
			"base_url": "https://cas.example/cas/app",
			"public_url": "https://danmaku.example",
			"default_role": 3
		}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CAS.DefaultRole != 3 {
		t.Fatalf("default role = %d", cfg.CAS.DefaultRole)
	}
}
